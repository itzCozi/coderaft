package docker

import (
	"context"
	"fmt"
	"time"
)

type Client struct {
	sdk *sdkClient
}

func NewClient() (*Client, error) {
	sdk, err := newSDKClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}
	return &Client{sdk: sdk}, nil
}

func (c *Client) Close() error {
	if c.sdk != nil {
		return c.sdk.close()
	}
	return nil
}

func (c *Client) SDKExecFunc() func(ctx context.Context, containerID string, cmd []string, showOutput bool) (string, string, int, error) {
	return func(ctx context.Context, containerID string, cmd []string, showOutput bool) (string, string, int, error) {
		result, err := c.sdk.containerExec(ctx, containerID, cmd, showOutput)
		if err != nil {
			return "", "", -1, err
		}
		return result.Stdout, result.Stderr, result.ExitCode, nil
	}
}

func IsDockerAvailable() error {

	sdk, err := newSDKClient()
	if err != nil {
		return fmt.Errorf("docker is not available: %w", err)
	}
	defer sdk.close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sdk.ping(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running. Please ensure Docker is installed and its daemon is running: %w", err)
	}
	return nil
}

func (c *Client) IsDockerAvailableWith() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.sdk.ping(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running: %w", err)
	}
	return nil
}
