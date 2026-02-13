package docker

import (
	"bytes"
	"context"
	"encoding/json"
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

type sdkClient struct {
	cli *dockerclient.Client
}

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

func (s *sdkClient) ping(ctx context.Context) error {
	_, err := s.cli.Ping(ctx)
	return err
}

func (s *sdkClient) imageExists(ctx context.Context, ref string) (bool, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("reference", ref)
	images, err := s.cli.ImageList(ctx, image.ListOptions{Filters: filterArgs})
	if err != nil {
		return false, err
	}
	return len(images) > 0, nil
}

func (s *sdkClient) pullImage(ctx context.Context, ref string) error {
	reader, err := s.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	defer reader.Close()

	
	decoder := json.NewDecoder(reader)
	for {
		var msg struct {
			Error string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			
			break
		}
		if msg.Error != "" {
			return fmt.Errorf("pull failed for %s: %s", ref, msg.Error)
		}
	}
	return nil
}

func (s *sdkClient) saveImage(ctx context.Context, ref string, dest io.Writer) error {
	reader, err := s.cli.ImageSave(ctx, []string{ref})
	if err != nil {
		return fmt.Errorf("image save failed: %w", err)
	}
	defer reader.Close()
	_, err = io.Copy(dest, reader)
	return err
}

func (s *sdkClient) loadImage(ctx context.Context, src io.Reader) (string, error) {
	resp, err := s.cli.ImageLoad(ctx, src)
	if err != nil {
		return "", fmt.Errorf("image load failed: %w", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, resp.Body)

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

func (s *sdkClient) commitContainer(ctx context.Context, containerID, ref string) (string, error) {
	resp, err := s.cli.ContainerCommit(ctx, containerID, container.CommitOptions{
		Reference: ref,
	})
	if err != nil {
		return "", fmt.Errorf("island commit failed: %w", err)
	}
	return resp.ID, nil
}

func (s *sdkClient) containerCreate(
	ctx context.Context,
	name, imageName, workspaceHost, workspaceBox string,
	projectConfig map[string]interface{},
) (string, error) {

	workspaceMount := mount.Mount{
		Type:   mount.TypeBind,
		Source: workspaceHost,
		Target: workspaceBox,
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
		Init:   &initTrue,
		Mounts: []mount.Mount{workspaceMount},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},

		Tmpfs: map[string]string{
			"/tmp": "rw,nosuid,nodev,size=256m",
		},

		ShmSize: 256 * 1024 * 1024,
	}

	networkConfig := &network.NetworkingConfig{}

	if projectConfig != nil {
		applyProjectConfigSDK(containerConfig, hostConfig, networkConfig, projectConfig)
	}

	resp, err := s.cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create island: %w", err)
	}
	return resp.ID, nil
}

func (s *sdkClient) containerStart(ctx context.Context, id string) error {
	return s.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (s *sdkClient) containerStop(ctx context.Context, id string, timeoutSec int) error {

	opts := container.StopOptions{Timeout: intPtr(timeoutSec)}
	return s.cli.ContainerStop(ctx, id, opts)
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

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

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

		_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, attachResp.Reader)
	} else {
		_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	}
	if err != nil {
		return nil, fmt.Errorf("exec read failed: %w", err)
	}

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

type statsJSON struct {
	CPUStats    cpuStatsJSON            `json:"cpu_stats"`
	PreCPUStats cpuStatsJSON            `json:"precpu_stats"`
	MemStats    memStatsJSON            `json:"memory_stats"`
	Networks    map[string]netStatsJSON `json:"networks"`
	BlkioStats  blkioStatsJSON          `json:"blkio_stats"`
	PidsStats   pidsStatsJSON           `json:"pids_stats"`
}

type cpuStatsJSON struct {
	CPUUsage struct {
		TotalUsage  uint64   `json:"total_usage"`
		PercpuUsage []uint64 `json:"percpu_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint32 `json:"online_cpus"`
}

type memStatsJSON struct {
	Usage uint64            `json:"usage"`
	Limit uint64            `json:"limit"`
	Stats map[string]uint64 `json:"stats"`
}

type netStatsJSON struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

type blkioStatsJSON struct {
	IoServiceBytesRecursive []blkioEntryJSON `json:"io_service_bytes_recursive"`
}

type blkioEntryJSON struct {
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

type pidsStatsJSON struct {
	Current uint64 `json:"current"`
}

func (s *sdkClient) containerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	resp, err := s.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get island stats: %w", err)
	}
	defer resp.Body.Close()

	var stats statsJSON
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	numCPUs := stats.CPUStats.OnlineCPUs
	if numCPUs == 0 {
		numCPUs = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	cpuPercent := 0.0
	if systemDelta > 0 && numCPUs > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(numCPUs) * 100.0
	}

	memUsage := stats.MemStats.Usage
	if cache, ok := stats.MemStats.Stats["cache"]; ok {
		memUsage -= cache
	}
	memLimit := stats.MemStats.Limit
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = float64(memUsage) / float64(memLimit) * 100.0
	}

	var rxBytes, txBytes uint64
	for _, iface := range stats.Networks {
		rxBytes += iface.RxBytes
		txBytes += iface.TxBytes
	}

	var blkRead, blkWrite uint64
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			blkRead += entry.Value
		case "write":
			blkWrite += entry.Value
		}
	}

	return &ContainerStats{
		CPUPercent: fmt.Sprintf("%.2f%%", cpuPercent),
		MemUsage:   fmt.Sprintf("%s / %s", units.HumanSize(float64(memUsage)), units.HumanSize(float64(memLimit))),
		MemPercent: fmt.Sprintf("%.2f%%", memPercent),
		NetIO:      fmt.Sprintf("%s / %s", units.HumanSize(float64(rxBytes)), units.HumanSize(float64(txBytes))),
		BlockIO:    fmt.Sprintf("%s / %s", units.HumanSize(float64(blkRead)), units.HumanSize(float64(blkWrite))),
		PIDs:       fmt.Sprintf("%d", stats.PidsStats.Current),
	}, nil
}

func applyProjectConfigSDK(
	cc *container.Config,
	hc *container.HostConfig,
	nc *network.NetworkingConfig,
	config map[string]interface{},
) {

	if restart, ok := config["restart"].(string); ok && restart != "" {
		hc.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(restart),
		}
	}

	if env, ok := config["environment"].(map[string]interface{}); ok {
		for key, value := range env {
			if valueStr, ok := value.(string); ok {
				cc.Env = append(cc.Env, fmt.Sprintf("%s=%s", key, valueStr))
			}
		}
	}

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

	if volumes, ok := config["volumes"].([]interface{}); ok {
		for _, volume := range volumes {
			if volumeStr, ok := volume.(string); ok {
				if strings.HasPrefix(volumeStr, "~") {
					if home, err := os.UserHomeDir(); err == nil {
						volumeStr = home + volumeStr[1:]
					}
				}
				
				parts := strings.SplitN(volumeStr, ":", 3)
				var source, target string
				if len(parts) == 3 && len(parts[0]) == 1 && parts[0][0] >= 'A' && parts[0][0] <= 'Z' || (len(parts) == 3 && len(parts[0]) == 1 && parts[0][0] >= 'a' && parts[0][0] <= 'z') {
					
					source = parts[0] + ":" + parts[1]
					target = parts[2]
				} else if len(parts) >= 2 {
					source = parts[0]
					target = parts[1]
				} else {
					continue
				}
				hc.Mounts = append(hc.Mounts, mount.Mount{
					Type:   mount.TypeBind,
					Source: source,
					Target: target,
				})
			}
		}
	}

	if dotfiles, ok := config["dotfiles"].([]interface{}); ok {
		for i, item := range dotfiles {
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
			target := "/dotfiles"
			if i > 0 {
				target = fmt.Sprintf("/dotfiles/%d", i)
			}
			hc.Mounts = append(hc.Mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: host,
				Target: target,
			})
		}
	}

	if workingDir, ok := config["working_dir"].(string); ok && workingDir != "" {
		cc.WorkingDir = workingDir
	}

	if user, ok := config["user"].(string); ok && user != "" {
		cc.User = user
	}

	if capabilities, ok := config["capabilities"].([]interface{}); ok {
		for _, cap := range capabilities {
			if capStr, ok := cap.(string); ok {
				hc.CapAdd = append(hc.CapAdd, capStr)
			}
		}
	}

	if labels, ok := config["labels"].(map[string]interface{}); ok {
		for key, value := range labels {
			if valueStr, ok := value.(string); ok {
				cc.Labels[key] = valueStr
			}
		}
	}

	if networkName, ok := config["network"].(string); ok && networkName != "" {
		hc.NetworkMode = container.NetworkMode(networkName)
	}

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

	if gpus, ok := config["gpus"].(string); ok && strings.TrimSpace(gpus) != "" {
		gpuStr := strings.TrimSpace(gpus)
		deviceReq := container.DeviceRequest{
			Capabilities: [][]string{{"gpu"}},
		}
		if gpuStr == "all" {
			deviceReq.Count = -1
		} else if n, err := strconv.Atoi(gpuStr); err == nil {
			deviceReq.Count = n
		} else {

			deviceReq.DeviceIDs = strings.Split(gpuStr, ",")
		}
		hc.DeviceRequests = append(hc.DeviceRequests, deviceReq)
	}

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

func intPtr(i int) *int { return &i }
