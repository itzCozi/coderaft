package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"coderaft/internal/engine"
	"coderaft/internal/security"
)

func escapeShellVar(s string) string {

	s = strings.ReplaceAll(s, "\r\n", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")

	s = strings.ReplaceAll(s, `\`, `\\`)
	// Escape double quotes
	s = strings.ReplaceAll(s, `"`, `\"`)
	// Escape dollar signs
	s = strings.ReplaceAll(s, `$`, `\$`)
	// Escape backticks
	s = strings.ReplaceAll(s, "`", "\\`")

	return s
}

func AttachShell(islandName string, projectName string) error {

	cmd := exec.Command(dockerCmd(), "exec", "-it",
		"-e", fmt.Sprintf("CODERAFT_ISLAND_NAME=%s", islandName),
		"-e", fmt.Sprintf("PROJECT_NAME=%s", projectName),
		islandName, "/bin/bash", "-c",
		"export PS1='coderaft(\\$PROJECT_NAME):\\w\\$ '; exec /bin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {

		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code == 130 || code == 137 || code == 0 {
				return nil
			}
		}
		return fmt.Errorf("failed to attach shell: %w", err)
	}
	return nil
}

// RunCommand executes a command inside the specified island container.
// Commands are validated for safety before execution.
func RunCommand(islandName string, command []string) error {
	if err := security.ValidateShellCommand(command); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Sanitize each argument
	sanitizedParts := make([]string, len(command))
	for i, part := range command {
		sanitizedParts[i] = security.SanitizeShellArg(part)
	}

	cmdStr := strings.Join(sanitizedParts, " ")
	wrapped := security.WrapShellCommand(cmdStr)
	args := []string{"exec", "-it", islandName, "bash", "-lc", wrapped}
	cmd := exec.Command(dockerCmd(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}
	return nil
}

func (c *Client) RunDockerCommand(args []string) error {
	cmd := exec.Command(dockerCmd(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker command failed: %w", err)
	}
	return nil
}

// ExecCapture executes a command inside a container and captures its output.
// The command is wrapped with bashrc sourcing and pipefail for proper error handling.
func (c *Client) ExecCapture(islandName, command string) (string, string, error) {
	wrapped := ". /root/.bashrc >/dev/null 2>&1 || true; set -o pipefail; " + command
	ctx, cancel := context.WithTimeout(context.Background(), security.Timeouts.ContainerExec)
	defer cancel()

	result, err := c.sdk.containerExec(ctx, islandName, []string{"bash", "-lc", wrapped}, false)
	if err != nil {
		return "", "", fmt.Errorf("exec failed: %w", err)
	}
	if result.ExitCode != 0 {
		return result.Stdout, result.Stderr, fmt.Errorf("exec failed: exit code %d", result.ExitCode)
	}
	return result.Stdout, result.Stderr, nil
}

func dockerCmd() string {
	return engine.Cmd()
}
