package docker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ContainerStats struct {
	CPUPercent string
	MemUsage   string
	MemPercent string
	NetIO      string
	BlockIO    string
	PIDs       string
}

func (c *Client) GetContainerStats(islandName string) (*ContainerStats, error) {
	ctx := context.Background()
	return c.sdk.containerStats(ctx, islandName)
}

func (c *Client) GetContainerID(islandName string) (string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, islandName)
	if err != nil {
		return "", fmt.Errorf("failed to get island ID: %w", err)
	}
	return inspect.ID, nil
}

func (c *Client) GetUptime(islandName string) (time.Duration, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, islandName)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect island: %w", err)
	}
	if inspect.State == nil || !inspect.State.Running {
		return 0, nil
	}
	startedAt := inspect.State.StartedAt
	t, parseErr := time.Parse(time.RFC3339Nano, startedAt)
	if parseErr != nil {
		if t2, err2 := time.Parse(time.RFC3339, startedAt); err2 == nil {
			return time.Since(t2), nil
		}
		return 0, nil
	}
	return time.Since(t), nil
}

func (c *Client) GetPortMappings(islandName string) ([]string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, islandName)
	if err != nil {
		return []string{}, nil
	}
	var ports []string
	if inspect.NetworkSettings != nil {
		for containerPort, bindings := range inspect.NetworkSettings.Ports {
			for _, binding := range bindings {
				ports = append(ports, fmt.Sprintf("%s -> %s:%s", containerPort, binding.HostIP, binding.HostPort))
			}
		}
	}
	return ports, nil
}

func (c *Client) GetMounts(islandName string) ([]string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, islandName)
	if err != nil {
		return nil, fmt.Errorf("failed to get mounts: %w", err)
	}
	var mounts []string
	for _, m := range inspect.Mounts {
		mounts = append(mounts, fmt.Sprintf("%s %s -> %s (rw=%v)", m.Type, m.Source, m.Destination, m.RW))
	}
	return mounts, nil
}

func (c *Client) IsContainerIdle(islandName string) (bool, error) {
	stats, err := c.GetContainerStats(islandName)
	if err != nil {
		return false, err
	}
	ports, err := c.GetPortMappings(islandName)
	if err != nil {
		return false, err
	}
	pids := 0
	if stats != nil && strings.TrimSpace(stats.PIDs) != "" {
		fmt.Sscanf(stats.PIDs, "%d", &pids)
	}
	return len(ports) == 0 && pids <= 1, nil
}

func (c *Client) GetContainerMeta(islandName string) (map[string]string, string, string, string, map[string]string, []string, map[string]string, string) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, islandName)
	if err != nil {
		return map[string]string{}, "", "", "", map[string]string{}, []string{}, map[string]string{}, ""
	}

	env := map[string]string{}
	for _, e := range inspect.Config.Env {
		if kv := strings.SplitN(e, "=", 2); len(kv) == 2 {
			env[kv[0]] = kv[1]
		}
	}

	resources := map[string]string{}
	if inspect.HostConfig != nil {
		if inspect.HostConfig.NanoCPUs > 0 {
			cpu := float64(inspect.HostConfig.NanoCPUs) / 1e9
			resources["cpus"] = strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", cpu), "0"), ".")
		}
		if inspect.HostConfig.Memory > 0 {
			mb := float64(inspect.HostConfig.Memory) / (1024 * 1024)
			resources["memory"] = fmt.Sprintf("%.0fMB", mb)
		}
	}

	restartPolicy := ""
	var capAdd []string
	networkMode := ""
	if inspect.HostConfig != nil {
		restartPolicy = string(inspect.HostConfig.RestartPolicy.Name)
		capAdd = inspect.HostConfig.CapAdd
		networkMode = string(inspect.HostConfig.NetworkMode)
	}

	return env, inspect.Config.WorkingDir, inspect.Config.User, restartPolicy, inspect.Config.Labels, capAdd, resources, networkMode
}
