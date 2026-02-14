package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"coderaft/internal/security"
	"coderaft/internal/ui"
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

	ctx, cancel := context.WithTimeout(context.Background(), security.Timeouts.DockerPing)
	defer cancel()

	if err := sdk.ping(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running. Please ensure Docker is installed and its daemon is running: %w", err)
	}
	return nil
}

func (c *Client) IsDockerAvailableWith() error {
	ctx, cancel := context.WithTimeout(context.Background(), security.Timeouts.DockerPing)
	defer cancel()
	if err := c.sdk.ping(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running: %w", err)
	}
	return nil
}

func findDockerDesktop() string {
	switch runtime.GOOS {
	case "windows":

		paths := []string{
			filepath.Join(os.Getenv("ProgramFiles"), "Docker", "Docker", "Docker Desktop.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Docker", "Docker", "Docker Desktop.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Docker", "Docker Desktop.exe"),
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}

		if path, err := exec.LookPath("Docker Desktop.exe"); err == nil {
			return path
		}
	case "darwin":
		paths := []string{
			"/Applications/Docker.app",
			filepath.Join(os.Getenv("HOME"), "Applications", "Docker.app"),
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func isDockerDesktopRunning() bool {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq Docker Desktop.exe", "/NH")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "Docker Desktop.exe") {
			return true
		}
	case "darwin":
		cmd := exec.Command("pgrep", "-x", "Docker")
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return false
}

func startDockerDesktop() error {
	switch runtime.GOOS {
	case "windows":
		dockerPath := findDockerDesktop()
		if dockerPath == "" {
			return fmt.Errorf("Docker Desktop not found. Please install Docker Desktop from https://docker.com/products/docker-desktop")
		}

		cmd := exec.Command("powershell", "-WindowStyle", "Hidden", "-Command",
			fmt.Sprintf(`Start-Process -FilePath "%s" -WindowStyle Minimized`, dockerPath))
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {

			cmd = exec.Command(dockerPath)
			cmd.Stdout = nil
			cmd.Stderr = nil
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to start Docker Desktop: %w", err)
			}
		}
		return nil

	case "darwin":
		dockerPath := findDockerDesktop()
		if dockerPath == "" {
			return fmt.Errorf("Docker Desktop not found. Please install Docker Desktop from https://docker.com/products/docker-desktop")
		}

		cmd := exec.Command("open", "-g", "-a", "Docker")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start Docker Desktop: %w", err)
		}
		return nil

	default:

		if err := exec.Command("systemctl", "is-active", "--quiet", "docker").Run(); err == nil {

			cmd := exec.Command("systemctl", "start", "docker")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to start Docker service (try: sudo systemctl start docker): %w", err)
			}
			return nil
		}

		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			return fmt.Errorf("Docker is not installed. Please install Docker: https://docs.docker.com/engine/install")
		}
		return fmt.Errorf("Docker daemon is not running. Please start it with: sudo systemctl start docker")
	}
}

func EnsureDockerRunning(timeout time.Duration) error {

	if err := IsDockerAvailable(); err == nil {
		return nil
	}

	if isDockerDesktopRunning() {
		ui.Info("Docker Desktop is starting up, waiting for daemon...")
		return waitForDocker(timeout)
	}

	ui.Info("Starting Docker...")
	if err := startDockerDesktop(); err != nil {
		return err
	}

	return waitForDocker(timeout)
}

func waitForDocker(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 2 * time.Second
	dots := 0

	for time.Now().Before(deadline) {
		if err := IsDockerAvailable(); err == nil {

			fmt.Print("\r\033[K")
			ui.Success("Docker is ready")
			return nil
		}

		dots = (dots % 3) + 1
		elapsed := time.Since(time.Now().Add(-timeout + time.Until(deadline)))
		fmt.Printf("\r  Waiting for Docker daemon%s (%.0fs)", strings.Repeat(".", dots), elapsed.Seconds())

		time.Sleep(checkInterval)
	}

	fmt.Print("\r\033[K")
	return fmt.Errorf("Docker did not start within %v. Please check Docker Desktop and try again", timeout)
}
