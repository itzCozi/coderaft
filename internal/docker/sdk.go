// Package docker provides the Docker client for devbox.
//
// This file contains the SDK-based wrapper around the Docker Engine API.
// Instead of shelling out to the `docker` CLI (which spawns a new process
// per call at ~200ms overhead each), we talk directly to the Docker daemon
// over a Unix socket (Linux/macOS) or named pipe (Windows) using a
// persistent HTTP connection. This eliminates process-spawn overhead and
// gives us structured JSON responses instead of parsing text output.
package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
)

// sdkClient wraps the official Docker SDK client.
// A single instance is created per devbox Client and reused for all API calls,
// maintaining a persistent HTTP connection to the Docker daemon.
type sdkClient struct {
	cli *dockerclient.Client
}

// newSDKClient creates a Docker SDK client using environment configuration.
// It reads DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH, etc.
// API version negotiation ensures compatibility across Docker versions.
func newSDKClient() (*sdkClient, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker SDK client: %w", err)
	}
	return &sdkClient{cli: cli}, nil
}

func (s *sdkClient) close() error {
	if s != nil && s.cli != nil {
		return s.cli.Close()
	}
	return nil
}

// --- Connectivity ---

func (s *sdkClient) ping(ctx context.Context) error {
	_, err := s.cli.Ping(ctx)
	return err
}

// --- Images ---

// imageExists checks if an image exists locally.
func (s *sdkClient) imageExists(ctx context.Context, ref string) (bool, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("reference", ref)
	images, err := s.cli.ImageList(ctx, image.ListOptions{Filters: filterArgs})
	if err != nil {
		return false, err
	}
	return len(images) > 0, nil
}

// pullImage downloads an image. Progress events are drained to complete the pull.
func (s *sdkClient) pullImage(ctx context.Context, ref string) error {
	reader, err := s.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	defer reader.Close()

	// Drain the response to complete the pull.
	// The reader emits JSON progress events; we discard them for clean output.
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// saveImage streams an image to a tar file.
func (s *sdkClient) saveImage(ctx context.Context, ref string, dest io.Writer) error {
	reader, err := s.cli.ImageSave(ctx, []string{ref})
	if err != nil {
		return fmt.Errorf("image save failed: %w", err)
	}
	defer reader.Close()
	_, err = io.Copy(dest, reader)
	return err
}

// loadImage loads an image from a tar reader.
func (s *sdkClient) loadImage(ctx context.Context, src io.Reader) (string, error) {
	resp, err := s.cli.ImageLoad(ctx, src)
	if err != nil {
		return "", fmt.Errorf("image load failed: %w", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, resp.Body)

	// Parse output to get loaded image reference
	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		last := lines[len(lines)-1]
		if i := strings.LastIndex(last, ": "); i != -1 {
			return strings.TrimSpace(last[i+2:]), nil
		}
	}
	return output, nil
}

// commitContainer creates a new image from a container's changes.
func (s *sdkClient) commitContainer(ctx context.Context, containerID, ref string) (string, error) {
	resp, err := s.cli.ContainerCommit(ctx, containerID, container.CommitOptions{
		Reference: ref,
	})
	if err != nil {
		return "", fmt.Errorf("container commit failed: %w", err)
	}
	return resp.ID, nil
}

// --- Containers ---

// containerCreate creates a container with full configuration from a project config map.
func (s *sdkClient) containerCreate(
	ctx context.Context,
	name, imageName, workspaceHost, workspaceBox string,
	projectConfig map[string]interface{},
) (string, error) {

	// Build mount for workspace with OS-specific consistency
	workspaceMount := mount.Mount{
		Type:   mount.TypeBind,
		Source: workspaceHost,
		Target: workspaceBox,
	}
	if consistency := MountConsistencyFlag(); consistency != "" {
		workspaceMount.BindOptions = &mount.BindOptions{
			Propagation: mount.PropagationRPrivate,
		}
	}

	containerConfig := &container.Config{
		Image:      imageName,
		WorkingDir: workspaceBox,
		Tty:        true,
		OpenStdin:  true,
		Cmd:        []string{"sleep", "infinity"},
		Labels:     map[string]string{},
		Env:        []string{},
	}

	initTrue := true
	hostConfig := &container.HostConfig{
		Init:   &initTrue, // PID 1 signal handling for fast shutdown
		Mounts: []mount.Mount{workspaceMount},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	networkConfig := &network.NetworkingConfig{}

	// Apply project configuration
	if projectConfig != nil {
		applyProjectConfigSDK(containerConfig, hostConfig, networkConfig, projectConfig)
	}

	resp, err := s.cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	return resp.ID, nil
}

func (s *sdkClient) containerStart(ctx context.Context, id string) error {
	return s.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (s *sdkClient) containerStop(ctx context.Context, id string, timeoutSec int) error {
	timeout := time.Duration(timeoutSec) * time.Second
	opts := container.StopOptions{Timeout: intPtr(timeoutSec)}

	err := s.cli.ContainerStop(ctx, id, opts)
	if err != nil {
		// If graceful stop fails, force kill
		killCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if killErr := s.cli.ContainerKill(killCtx, id, "SIGKILL"); killErr != nil {
			return fmt.Errorf("failed to stop container: %w (kill also failed: %v)", err, killErr)
		}
	}
	return nil
}

func (s *sdkClient) containerRemove(ctx context.Context, id string) error {
	return s.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (s *sdkClient) containerInspect(ctx context.Context, id string) (container.InspectResponse, error) {
	return s.cli.ContainerInspect(ctx, id)
}

func (s *sdkClient) containerList(ctx context.Context, all bool) ([]container.Summary, error) {
	return s.cli.ContainerList(ctx, container.ListOptions{All: all})
}

// --- Exec ---

// ExecResult holds the output from a container exec call.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// containerExec runs a command inside a container via the Docker API.
// This avoids spawning a `docker exec` CLI process (~200ms overhead).
func (s *sdkClient) containerExec(ctx context.Context, containerID string, cmd []string, showOutput bool) (*ExecResult, error) {
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := s.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("exec create failed: %w", err)
	}

	attachResp, err := s.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("exec attach failed: %w", err)
	}
	defer attachResp.Close()

	var stdout, stderr bytes.Buffer
	if showOutput {
		// Stream directly to os.Stdout/Stderr
		_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, attachResp.Reader)
	} else {
		_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	}
	if err != nil {
		return nil, fmt.Errorf("exec read failed: %w", err)
	}

	// Get exit code
	inspectResp, err := s.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("exec inspect failed: %w", err)
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: inspectResp.ExitCode,
	}, nil
}

// --- Config Translation ---

// applyProjectConfigSDK translates the project config map into SDK container config structs.
// This replaces the old applyProjectConfigToArgs which built CLI flags.
func applyProjectConfigSDK(
	cc *container.Config,
	hc *container.HostConfig,
	nc *network.NetworkingConfig,
	config map[string]interface{},
) {
	// Restart policy
	if restart, ok := config["restart"].(string); ok && restart != "" {
		hc.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(restart),
		}
	}

	// Environment variables
	if env, ok := config["environment"].(map[string]interface{}); ok {
		for key, value := range env {
			if valueStr, ok := value.(string); ok {
				cc.Env = append(cc.Env, fmt.Sprintf("%s=%s", key, valueStr))
			}
		}
	}

	// Port mappings
	if ports, ok := config["ports"].([]interface{}); ok {
		var portSpecs []string
		for _, port := range ports {
			if portStr, ok := port.(string); ok {
				portSpecs = append(portSpecs, portStr)
			}
		}
		if len(portSpecs) > 0 {
			exposedPorts, portBindings, err := nat.ParsePortSpecs(portSpecs)
			if err == nil {
				if cc.ExposedPorts == nil {
					cc.ExposedPorts = nat.PortSet{}
				}
				for k, v := range exposedPorts {
					cc.ExposedPorts[k] = v
				}
				if hc.PortBindings == nil {
					hc.PortBindings = nat.PortMap{}
				}
				for k, v := range portBindings {
					hc.PortBindings[k] = v
				}
			}
		}
	}

	// Volumes
	if volumes, ok := config["volumes"].([]interface{}); ok {
		for _, volume := range volumes {
			if volumeStr, ok := volume.(string); ok {
				if strings.HasPrefix(volumeStr, "~") {
					if home, err := os.UserHomeDir(); err == nil {
						volumeStr = home + volumeStr[1:]
					}
				}
				parts := strings.SplitN(volumeStr, ":", 2)
				if len(parts) == 2 {
					hc.Mounts = append(hc.Mounts, mount.Mount{
						Type:   mount.TypeBind,
						Source: parts[0],
						Target: parts[1],
					})
				}
			}
		}
	}

	// Dotfiles mount
	if dotfiles, ok := config["dotfiles"].([]interface{}); ok {
		for _, item := range dotfiles {
			pathStr, ok := item.(string)
			if !ok || pathStr == "" {
				continue
			}
			host := pathStr
			if strings.HasPrefix(host, "~") {
				if home, err := os.UserHomeDir(); err == nil {
					host = home + host[1:]
				}
			}
			hc.Mounts = append(hc.Mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: host,
				Target: "/dotfiles",
			})
			break
		}
	}

	// Working directory
	if workingDir, ok := config["working_dir"].(string); ok && workingDir != "" {
		cc.WorkingDir = workingDir
	}

	// User
	if user, ok := config["user"].(string); ok && user != "" {
		cc.User = user
	}

	// Capabilities
	if capabilities, ok := config["capabilities"].([]interface{}); ok {
		for _, cap := range capabilities {
			if capStr, ok := cap.(string); ok {
				hc.CapAdd = append(hc.CapAdd, capStr)
			}
		}
	}

	// Labels
	if labels, ok := config["labels"].(map[string]interface{}); ok {
		for key, value := range labels {
			if valueStr, ok := value.(string); ok {
				cc.Labels[key] = valueStr
			}
		}
	}

	// Network
	if networkName, ok := config["network"].(string); ok && networkName != "" {
		hc.NetworkMode = container.NetworkMode(networkName)
	}

	// Resources
	if resources, ok := config["resources"].(map[string]interface{}); ok {
		if cpus, ok := resources["cpus"].(string); ok && cpus != "" {
			if cpuVal, err := strconv.ParseFloat(cpus, 64); err == nil {
				hc.Resources.NanoCPUs = int64(cpuVal * 1e9)
			}
		}
		if memory, ok := resources["memory"].(string); ok && memory != "" {
			if memBytes, err := units.RAMInBytes(memory); err == nil {
				hc.Resources.Memory = memBytes
			}
		}
	}

	// GPUs
	if gpus, ok := config["gpus"].(string); ok && strings.TrimSpace(gpus) != "" {
		gpuStr := strings.TrimSpace(gpus)
		deviceReq := container.DeviceRequest{
			Capabilities: [][]string{{"gpu"}},
		}
		if gpuStr == "all" {
			deviceReq.Count = -1 // all GPUs
		} else if n, err := strconv.Atoi(gpuStr); err == nil {
			deviceReq.Count = n
		} else {
			// Treat as device IDs
			deviceReq.DeviceIDs = strings.Split(gpuStr, ",")
		}
		hc.DeviceRequests = append(hc.DeviceRequests, deviceReq)
	}

	// Health check
	if healthCheck, ok := config["health_check"].(map[string]interface{}); ok {
		healthCfg := &container.HealthConfig{}
		if test, ok := healthCheck["test"].([]interface{}); ok && len(test) > 0 {
			for _, t := range test {
				if testStr, ok := t.(string); ok {
					healthCfg.Test = append(healthCfg.Test, testStr)
				}
			}
		}
		if interval, ok := healthCheck["interval"].(string); ok && interval != "" {
			if d, err := time.ParseDuration(interval); err == nil {
				healthCfg.Interval = d
			}
		}
		if timeout, ok := healthCheck["timeout"].(string); ok && timeout != "" {
			if d, err := time.ParseDuration(timeout); err == nil {
				healthCfg.Timeout = d
			}
		}
		if retries, ok := healthCheck["retries"].(float64); ok && retries > 0 {
			healthCfg.Retries = int(retries)
		}
		cc.Healthcheck = healthCfg
	}
}

// --- Helpers ---

func intPtr(i int) *int { return &i }
