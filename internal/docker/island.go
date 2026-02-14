package docker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/docker/docker/client"
)

func (c *Client) CreateIsland(name, image, workspaceHost, workspaceIsland string) (string, error) {
	return c.CreateIslandWithConfig(name, image, workspaceHost, workspaceIsland, nil)
}

func (c *Client) CreateIslandWithConfig(name, image, workspaceHost, workspaceIsland string, projectConfig interface{}) (string, error) {
	ctx := context.Background()

	var config map[string]interface{}
	if projectConfig != nil {
		if cfg, ok := projectConfig.(map[string]interface{}); ok {
			config = cfg
		}
	}

	islandID, err := c.sdk.containerCreate(ctx, name, image, workspaceHost, workspaceIsland, config)
	if err != nil {
		return "", fmt.Errorf("failed to create island: %w", err)
	}
	return islandID, nil
}

func (c *Client) StartIsland(islandID string) error {
	ctx := context.Background()
	if err := c.sdk.containerStart(ctx, islandID); err != nil {
		return fmt.Errorf("failed to start island: %w", err)
	}
	return nil
}

func (c *Client) StopIsland(islandName string) error {

	timeoutSec := 2
	if v := strings.TrimSpace(os.Getenv("CODERAFT_STOP_TIMEOUT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			timeoutSec = n
		}
	}
	ctx := context.Background()
	if err := c.sdk.containerStop(ctx, islandName, timeoutSec); err != nil {
		return fmt.Errorf("failed to stop island: %w", err)
	}
	return nil
}

func (c *Client) RemoveIsland(islandName string) error {
	ctx := context.Background()
	if err := c.sdk.containerRemove(ctx, islandName); err != nil {
		return fmt.Errorf("failed to remove island: %w", err)
	}
	return nil
}

func (c *Client) IslandExists(islandName string) (bool, error) {
	ctx := context.Background()
	_, err := c.sdk.containerInspect(ctx, islandName)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect island: %w", err)
	}
	return true, nil
}

func (c *Client) GetIslandStatus(islandName string) (string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, islandName)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return "not found", nil
		}
		return "", fmt.Errorf("failed to inspect island: %w", err)
	}
	return inspect.State.Status, nil
}

func (c *Client) WaitForIsland(islandName string, timeout time.Duration) error {
	start := time.Now()

	pollInterval := 25 * time.Millisecond
	maxInterval := 500 * time.Millisecond
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for island to be ready")
		}

		status, err := c.GetIslandStatus(islandName)
		if err != nil {
			return fmt.Errorf("failed to get island status: %w", err)
		}

		if status == "running" {
			return nil
		}

		time.Sleep(pollInterval)
		pollInterval *= 2
		if pollInterval > maxInterval {
			pollInterval = maxInterval
		}
	}
}

type IslandInfo struct {
	Names  []string
	Status string
	Image  string
}

func (c *Client) ListIslands() ([]IslandInfo, error) {
	ctx := context.Background()
	containers, err := c.sdk.containerList(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list islands: %w", err)
	}

	var islands []IslandInfo
	for _, ctr := range containers {
		for _, name := range ctr.Names {

			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, "coderaft_") {
				islands = append(islands, IslandInfo{
					Names:  []string{cleanName},
					Status: ctr.Status,
					Image:  ctr.Image,
				})
				break
			}
		}
	}
	return islands, nil
}
