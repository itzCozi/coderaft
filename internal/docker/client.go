package docker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/docker/docker/client"

	"devbox/internal/parallel"
)

type Client struct {
	sdk *sdkClient // SDK client for direct Docker API access
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

func dockerCmd() string {
	if eng := strings.TrimSpace(os.Getenv("DEVBOX_ENGINE")); eng != "" {
		return eng
	}
	return "docker"
}

func IsDockerAvailable() error {
	// Use SDK ping for fast daemon check (no process spawn)
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

// IsDockerAvailableWith checks Docker availability using an existing client,
// avoiding a redundant SDK connection.
func (c *Client) IsDockerAvailableWith() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.sdk.ping(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running: %w", err)
	}
	return nil
}

func (c *Client) PullImage(ref string) error {
	ctx := context.Background()

	// Check if image already exists locally (SDK: no process spawn)
	exists, err := c.sdk.imageExists(ctx, ref)
	if err == nil && exists {
		return nil
	}

	fmt.Printf("Pulling image %s...\n", ref)
	if err := c.sdk.pullImage(ctx, ref); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	fmt.Printf("Image %s pulled successfully.\n", ref)
	return nil
}

func (c *Client) CreateBox(name, image, workspaceHost, workspaceBox string) (string, error) {
	return c.CreateBoxWithConfig(name, image, workspaceHost, workspaceBox, nil)
}

func (c *Client) CreateBoxWithConfig(name, image, workspaceHost, workspaceBox string, projectConfig interface{}) (string, error) {
	ctx := context.Background()

	var config map[string]interface{}
	if projectConfig != nil {
		if cfg, ok := projectConfig.(map[string]interface{}); ok {
			config = cfg
		}
	}

	boxID, err := c.sdk.containerCreate(ctx, name, image, workspaceHost, workspaceBox, config)
	if err != nil {
		return "", fmt.Errorf("failed to create box: %w", err)
	}
	return boxID, nil
}

func (c *Client) ExecuteSetupCommands(boxName string, commands []string) error {
	return c.ExecuteSetupCommandsWithOutput(boxName, commands, true)
}

func (c *Client) ExecuteSetupCommandsWithOutput(boxName string, commands []string, showOutput bool) error {
	if len(commands) == 0 {
		return nil
	}

	if showOutput {
		fmt.Printf("Executing setup commands in box '%s'...\n", boxName)
	}

	config := parallel.LoadConfig()
	if config.EnableParallel {

		executor := parallel.NewSetupCommandExecutor(boxName, showOutput, config.SetupCommandWorkers)
		if err := executor.ExecuteParallel(commands); err != nil {

			fmt.Printf("Parallel execution failed, falling back to sequential: %v\n", err)
			return c.ExecuteSetupCommandsSequential(boxName, commands, showOutput)
		}
	} else {

		return c.ExecuteSetupCommandsSequential(boxName, commands, showOutput)
	}

	if showOutput {
		fmt.Printf("Setup commands completed successfully!\n")
	}
	return nil
}

func (c *Client) ExecuteSetupCommandsSequential(boxName string, commands []string, showOutput bool) error {
	if len(commands) == 0 {
		return nil
	}

	if showOutput {
		fmt.Printf("Executing setup commands in box '%s'...\n", boxName)
	}

	// Batch commands into a single exec call to avoid the overhead of
	// spawning a new docker exec process per command (~200ms each).
	// For N commands, this saves ~(N-1)*200ms of process startup time.
	batchSize := 10
	for i := 0; i < len(commands); i += batchSize {
		end := i + batchSize
		if end > len(commands) {
			end = len(commands)
		}
		batch := commands[i:end]

		if showOutput {
			fmt.Printf("Steps %d-%d/%d\n", i+1, end, len(commands))
		}

		// Join commands with error-checking: each command must succeed
		// before the next runs (set -e ensures this)
		var scriptBuilder strings.Builder
		scriptBuilder.WriteString(". /root/.bashrc >/dev/null 2>&1 || true; set -e; ")
		for j, command := range batch {
			if j > 0 {
				scriptBuilder.WriteString(" ; ")
			}
			if showOutput {
				// Echo the command being run for visibility
				scriptBuilder.WriteString(fmt.Sprintf("echo '==> %s' ; ", strings.ReplaceAll(command, "'", "'\\''")))
			}
			scriptBuilder.WriteString(command)
		}

		cmd := []string{"bash", "-lc", scriptBuilder.String()}
		ctx := context.Background()
		result, err := c.sdk.containerExec(ctx, boxName, cmd, showOutput)
		if err != nil {
			return fmt.Errorf("setup command batch failed (steps %d-%d): %w", i+1, end, err)
		}
		if result != nil && result.ExitCode != 0 {
			if !showOutput && result.Stderr != "" {
				fmt.Printf("Command batch failed (steps %d-%d)\n", i+1, end)
				fmt.Printf("Error output: %s\n", result.Stderr)
			}
			return fmt.Errorf("setup command batch failed (steps %d-%d): exit code %d", i+1, end, result.ExitCode)
		}
	}

	if showOutput {
		fmt.Printf("Setup commands completed successfully!\n")
	}
	return nil
}

func (c *Client) QueryPackagesParallel(boxName string) (aptList, pipList, npmList, yarnList, pnpmList []string) {
	config := parallel.LoadConfig()
	if !config.EnableParallel {

		return c.queryPackagesSequential(boxName)
	}

	executor := parallel.NewPackageQueryExecutor(boxName)

	packageLists, err := executor.QueryAllPackages()
	if err != nil {
		fmt.Printf("Warning: parallel package query failed, falling back to sequential: %v\n", err)

		return c.queryPackagesSequential(boxName)
	}

	return packageLists["apt"], packageLists["pip"], packageLists["npm"], packageLists["yarn"], packageLists["pnpm"]
}

func (c *Client) queryPackagesSequential(boxName string) (aptList, pipList, npmList, yarnList, pnpmList []string) {

	return nil, nil, nil, nil, nil
}

func (c *Client) StartBox(boxID string) error {
	ctx := context.Background()
	if err := c.sdk.containerStart(ctx, boxID); err != nil {
		return fmt.Errorf("failed to start box: %w", err)
	}
	return nil
}

func (c *Client) SetupDevboxInBox(boxName, projectName string) error {
	return c.setupDevboxInBoxWithOptions(boxName, projectName, false)
}

func (c *Client) SetupDevboxInBoxWithUpdate(boxName, projectName string) error {
	return c.setupDevboxInBoxWithOptions(boxName, projectName, true)
}

func (c *Client) setupDevboxInBoxWithOptions(boxName, projectName string, forceUpdate bool) error {

	ctx := context.Background()

	// Check if this is the first time setup
	checkResult, _ := c.sdk.containerExec(ctx, boxName, []string{"test", "-f", "/etc/devbox-initialized"}, false)
	isFirstTime := checkResult == nil || checkResult.ExitCode != 0

	if isFirstTime {
		_, err := c.sdk.containerExec(ctx, boxName, []string{"touch", "/etc/devbox-initialized"}, false)
		if err != nil {
			fmt.Printf("Warning: failed to create initialization marker: %v\n", err)
		}
	}

	wrapperScript := `#!/bin/bash

# devbox-wrapper.sh
# This script provides devbox commands inside the box

BOX_NAME="` + boxName + `"
PROJECT_NAME="` + projectName + `"

case "$1" in
	"status"|"info")
		echo "Devbox box status"
        echo "Project: $PROJECT_NAME"
        echo "Box: $BOX_NAME"
        echo "Workspace: /workspace"
        echo "Host: $(cat /etc/hostname)"
        echo "User: $(whoami)"
        echo "Working Directory: $(pwd)"
        echo ""
	echo "hint: available devbox commands inside box:"
        echo "  devbox exit     - Exit the shell"
        echo "  devbox status   - Show box information"
        echo "  devbox help     - Show this help"
        echo "  devbox host     - Run command on host (experimental)"
        ;;
	"help"|"--help"|"-h")
		echo "Devbox box commands"
        echo ""
        echo "Available commands inside the box:"
        echo "  devbox exit         - Exit the devbox shell"
        echo "  devbox status       - Show box and project information"
        echo "  devbox help         - Show this help message"
        echo "  devbox host <cmd>   - Execute command on host (experimental)"
        echo ""
	echo "Your project files are in: /workspace"
	echo "You are in an Ubuntu box with full package management"
        echo ""
        echo "Examples:"
        echo "  devbox exit                    # Exit to host"
        echo "  devbox status                  # Check box info"
        echo "  devbox host \"devbox list\"     # Run host command"
        echo ""
	echo "hint: Files in /workspace are shared with your host system"
        ;;
    "host")
		if [ -z "$2" ]; then
			echo "error: usage: devbox host <command>"
            echo "Example: devbox host \"devbox list\""
            exit 1
        fi
		echo "Executing on host: $2"
		echo "warning: This is experimental and may not work in all environments"
        # This is a placeholder - we cannot easily execute on host from box
        # without additional setup like Docker socket mounting
		echo "error: host command execution not yet implemented"
		echo "hint: Exit the box and run commands on the host instead"
        ;;
    "version")
        echo "devbox box wrapper v1.0"
        echo "Box: $BOX_NAME"
        echo "Project: $PROJECT_NAME"
        ;;
	"")
		echo "error: missing command. Use \"devbox help\" for available commands."
        exit 1
        ;;
    *)
		echo "error: unknown devbox command: $1"
		echo "hint: Use \"devbox help\" to see available commands inside the box"
        echo ""
        echo "Available commands:"
        echo "  exit, status, help, host, version"
        echo ""
        echo "Note: 'devbox exit' is handled by the shell function for proper exit behavior"
        exit 1
        ;;
esac`

	installCmd := `rm -f /usr/local/bin/devbox && cat > /usr/local/bin/devbox << 'DEVBOX_WRAPPER_EOF'
` + wrapperScript + `
DEVBOX_WRAPPER_EOF
chmod +x /usr/local/bin/devbox`

	result, err := c.sdk.containerExec(ctx, boxName, []string{"bash", "-c", installCmd}, false)
	if err != nil {
		return fmt.Errorf("failed to install devbox wrapper in box: %w", err)
	}
	if result != nil && result.ExitCode != 0 {
		return fmt.Errorf("failed to install devbox wrapper in box: exit code %d: %s", result.ExitCode, result.Stderr)
	}

	welcomeCmd := `# Remove any existing devbox configurations
sed -i '/# Devbox welcome message/,/^$/d' /root/.bashrc 2>/dev/null || true
sed -i '/devbox_exit()/,/^}$/d' /root/.bashrc 2>/dev/null || true
sed -i '/devbox() {/,/^}$/d' /root/.bashrc 2>/dev/null || true
	sed -i '/# Devbox package tracking start/,/# Devbox package tracking end/d' /root/.bashrc 2>/dev/null || true

cat >> /root/.bashrc << 'BASHRC_EOF'

if [ -t 1 ]; then
	echo "Welcome to devbox project: ` + projectName + `"
	echo "Your files are in: /workspace"
	echo "hint: Type 'devbox help' for available commands"
	echo "hint: Type 'devbox exit' to leave the box"
    echo ""
fi

if [ -d "/dotfiles" ]; then
	if [ -f "/dotfiles/.bashrc" ]; then
		. /dotfiles/.bashrc
	fi
	for f in .gitconfig .vimrc .zshrc .bash_profile; do
		if [ -f "/dotfiles/$f" ]; then
			ln -sf "/dotfiles/$f" "/root/$f"
		fi
	done
	if [ -d "/dotfiles/.config" ]; then
		mkdir -p /root/.config
		for item in /dotfiles/.config/*; do
			base=$(basename "$item")
			if [ ! -e "/root/.config/$base" ]; then
				ln -s "$item" "/root/.config/$base"
			fi
		done
	fi
fi

devbox_exit() {
	echo "Exiting devbox shell for project \"` + projectName + `\""
	exit 0
}

devbox() {
    if [[ "$1" == "exit" || "$1" == "quit" ]]; then
        devbox_exit
        return
    fi
    /usr/local/bin/devbox "$@"
}

export DEVBOX_LOCKFILE="${DEVBOX_LOCKFILE:-/workspace/devbox.lock}"

devbox_record_cmd() {
	local cmd="$1"
	if [ -n "$DEVBOX_LOCKFILE" ] && [ -w "$(dirname "$DEVBOX_LOCKFILE")" ]; then
		if [ ! -f "$DEVBOX_LOCKFILE" ] || ! grep -Fxq "$cmd" "$DEVBOX_LOCKFILE" 2>/dev/null; then
			echo "$cmd" >> "$DEVBOX_LOCKFILE"
		fi
	fi
}

_devbox_wrap_and_record() {
	local bin="$1"; shift
	local name="$1"; shift
	"$bin" "$@"
	local status=$?
	if [ $status -eq 0 ]; then
		case "$name" in
			apt|apt-get)
				# Track install/remove/purge/autoremove
				if printf ' %s ' "$*" | grep -qE '(^| )(install|remove|purge|autoremove)( |$)'; then
					devbox_record_cmd "$name $*"
				fi
				;;
			pip|pip3)
				if [ "$1" = install ] || [ "$1" = uninstall ]; then
					devbox_record_cmd "$name $*"
				fi
				;;
			npm)
				# Track install and uninstall variants
				if [ "$1" = install ] || [ "$1" = i ] || [ "$1" = add ] \
				   || [ "$1" = uninstall ] || [ "$1" = remove ] || [ "$1" = rm ] || [ "$1" = r ] || [ "$1" = un ]; then
					devbox_record_cmd "$name $*"
				fi
				;;
			yarn)
				# Track add/remove and global add/remove
				if [ "$1" = add ] || [ "$1" = remove ] || { [ "$1" = global ] && { [ "$2" = add ] || [ "$2" = remove ]; }; }; then
					devbox_record_cmd "$name $*"
				fi
				;;
			pnpm)
				# Track add/install and remove/uninstall variants
				if [ "$1" = add ] || [ "$1" = install ] || [ "$1" = i ] \
				   || [ "$1" = remove ] || [ "$1" = rm ] || [ "$1" = uninstall ] || [ "$1" = un ]; then
					devbox_record_cmd "$name $*"
				fi
				;;
			corepack)
				# Handle: corepack yarn add ..., corepack yarn global add ...
				#         corepack yarn remove ..., corepack yarn global remove ...
				#         corepack pnpm add/install/i/remove/rm/uninstall/un ...
				subcmd="$1"; shift || true
				if [ "$subcmd" = yarn ]; then
					if [ "$1" = add ] || [ "$1" = remove ] || { [ "$1" = global ] && { [ "$2" = add ] || [ "$2" = remove ]; }; }; then
						devbox_record_cmd "corepack yarn $*"
					fi
				elif [ "$subcmd" = pnpm ]; then
					if [ "$1" = add ] || [ "$1" = install ] || [ "$1" = i ] \
					   || [ "$1" = remove ] || [ "$1" = rm ] || [ "$1" = uninstall ] || [ "$1" = un ]; then
						devbox_record_cmd "corepack pnpm $*"
					fi
				fi
				;;
		esac
	fi
	return $status
}

APT_BIN="$(command -v apt 2>/dev/null || echo /usr/bin/apt)"
APTGET_BIN="$(command -v apt-get 2>/dev/null || echo /usr/bin/apt-get)"
PIP_BIN="$(command -v pip 2>/dev/null || echo /usr/bin/pip)"
PIP3_BIN="$(command -v pip3 2>/dev/null || echo /usr/bin/pip3)"
NPM_BIN="$(command -v npm 2>/dev/null || echo /usr/bin/npm)"
YARN_BIN="$(command -v yarn 2>/dev/null || echo /usr/bin/yarn)"
PNPM_BIN="$(command -v pnpm 2>/dev/null || echo /usr/bin/pnpm)"
COREPACK_BIN="$(command -v corepack 2>/dev/null || echo /usr/bin/corepack)"

apt()      { _devbox_wrap_and_record "$APT_BIN" apt "$@"; }
apt-get()  { _devbox_wrap_and_record "$APTGET_BIN" apt-get "$@"; }
pip()      { _devbox_wrap_and_record "$PIP_BIN" pip "$@"; }
pip3()     { _devbox_wrap_and_record "$PIP3_BIN" pip3 "$@"; }
npm()      { _devbox_wrap_and_record "$NPM_BIN" npm "$@"; }
yarn()     { _devbox_wrap_and_record "$YARN_BIN" yarn "$@"; }
pnpm()     { _devbox_wrap_and_record "$PNPM_BIN" pnpm "$@"; }
corepack(){ _devbox_wrap_and_record "$COREPACK_BIN" corepack "$@"; }
BASHRC_EOF`

	_, welcomeErr := c.sdk.containerExec(ctx, boxName, []string{"bash", "-c", welcomeCmd}, false)
	if welcomeErr != nil {

		fmt.Printf("Warning: failed to add welcome message: %v\n", welcomeErr)
	}

	return nil
}

func (c *Client) StopBox(boxName string) error {
	// With --init flag on creation, containers respond to SIGTERM properly
	// so we can use a short timeout for fast shutdown
	timeoutSec := 2
	if v := strings.TrimSpace(os.Getenv("DEVBOX_STOP_TIMEOUT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			timeoutSec = n
		}
	}
	ctx := context.Background()
	if err := c.sdk.containerStop(ctx, boxName, timeoutSec); err != nil {
		return fmt.Errorf("failed to stop box: %w", err)
	}
	return nil
}

func (c *Client) RemoveBox(boxName string) error {
	ctx := context.Background()
	if err := c.sdk.containerRemove(ctx, boxName); err != nil {
		return fmt.Errorf("failed to remove box: %w", err)
	}
	return nil
}

func (c *Client) BoxExists(boxName string) (bool, error) {
	ctx := context.Background()
	_, err := c.sdk.containerInspect(ctx, boxName)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect box: %w", err)
	}
	return true, nil
}

func (c *Client) GetBoxStatus(boxName string) (string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, boxName)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return "not found", nil
		}
		return "", fmt.Errorf("failed to inspect box: %w", err)
	}
	return inspect.State.Status, nil
}

func AttachShell(boxName string) error {

	cmd := exec.Command(dockerCmd(), "exec", "-it",
		"-e", fmt.Sprintf("DEVBOX_BOX_NAME=%s", boxName),
		boxName, "/bin/bash", "-c",
		"export PS1='devbox(\\$PROJECT_NAME):\\w\\$ '; exec /bin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to attach shell: %w", err)
	}
	return nil
}

func RunCommand(boxName string, command []string) error {
	cmdStr := strings.Join(command, " ")
	wrapped := ". /root/.bashrc >/dev/null 2>&1 || true; " + cmdStr
	args := []string{"exec", "-it", boxName, "bash", "-lc", wrapped}
	cmd := exec.Command(dockerCmd(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}
	return nil
}

func (c *Client) WaitForBox(boxName string, timeout time.Duration) error {
	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for box to be ready")
		}

		status, err := c.GetBoxStatus(boxName)
		if err != nil {
			return fmt.Errorf("failed to get box status: %w", err)
		}

		if status == "running" {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

type BoxInfo struct {
	Names  []string
	Status string
	Image  string
}

func (c *Client) ListBoxes() ([]BoxInfo, error) {
	ctx := context.Background()
	containers, err := c.sdk.containerList(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list boxes: %w", err)
	}

	var boxes []BoxInfo
	for _, ctr := range containers {
		for _, name := range ctr.Names {
			// Docker prefixes names with "/"
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, "devbox_") {
				boxes = append(boxes, BoxInfo{
					Names:  []string{cleanName},
					Status: ctr.Status,
					Image:  ctr.Image,
				})
				break
			}
		}
	}
	return boxes, nil
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

type ContainerStats struct {
	CPUPercent string
	MemUsage   string
	MemPercent string
	NetIO      string
	BlockIO    string
	PIDs       string
}

func (c *Client) CommitContainer(containerName, imageTag string) (string, error) {
	ctx := context.Background()
	id, err := c.sdk.commitContainer(ctx, containerName, imageTag)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *Client) SaveImage(imageRef, tarPath string) error {
	f, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("failed to create tar file: %w", err)
	}
	defer f.Close()

	ctx := context.Background()
	return c.sdk.saveImage(ctx, imageRef, f)
}

func (c *Client) LoadImage(tarPath string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to open tar file: %w", err)
	}
	defer f.Close()

	ctx := context.Background()
	return c.sdk.loadImage(ctx, f)
}

func (c *Client) GetContainerStats(boxName string) (*ContainerStats, error) {

	format := "{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}"
	cmd := exec.Command(dockerCmd(), "stats", "--no-stream", "--format", format, boxName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if s := strings.TrimSpace(stderr.String()); s != "" {
			return nil, fmt.Errorf("failed to get stats: %s", s)
		}
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	line := strings.TrimSpace(stdout.String())
	if line == "" {

		return &ContainerStats{}, nil
	}
	parts := strings.Split(line, "\t")

	for len(parts) < 6 {
		parts = append(parts, "")
	}
	return &ContainerStats{
		CPUPercent: strings.TrimSpace(parts[0]),
		MemUsage:   strings.TrimSpace(parts[1]),
		MemPercent: strings.TrimSpace(parts[2]),
		NetIO:      strings.TrimSpace(parts[3]),
		BlockIO:    strings.TrimSpace(parts[4]),
		PIDs:       strings.TrimSpace(parts[5]),
	}, nil
}

func (c *Client) GetContainerID(boxName string) (string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, boxName)
	if err != nil {
		return "", fmt.Errorf("failed to get container ID: %w", err)
	}
	return inspect.ID, nil
}

func (c *Client) GetUptime(boxName string) (time.Duration, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, boxName)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect container: %w", err)
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

func (c *Client) GetPortMappings(boxName string) ([]string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, boxName)
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

func (c *Client) GetMounts(boxName string) ([]string, error) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, boxName)
	if err != nil {
		return nil, fmt.Errorf("failed to get mounts: %w", err)
	}
	var mounts []string
	for _, m := range inspect.Mounts {
		mounts = append(mounts, fmt.Sprintf("%s %s -> %s (rw=%v)", m.Type, m.Source, m.Destination, m.RW))
	}
	return mounts, nil
}

func (c *Client) IsContainerIdle(boxName string) (bool, error) {
	stats, err := c.GetContainerStats(boxName)
	if err != nil {
		return false, err
	}
	ports, err := c.GetPortMappings(boxName)
	if err != nil {
		return false, err
	}
	pids := 0
	if stats != nil && strings.TrimSpace(stats.PIDs) != "" {
		fmt.Sscanf(stats.PIDs, "%d", &pids)
	}
	return len(ports) == 0 && pids <= 1, nil
}

func (c *Client) ExecCapture(boxName, command string) (string, string, error) {
	wrapped := ". /root/.bashrc >/dev/null 2>&1 || true; set -o pipefail; " + command
	ctx := context.Background()
	result, err := c.sdk.containerExec(ctx, boxName, []string{"bash", "-lc", wrapped}, false)
	if err != nil {
		return "", "", fmt.Errorf("exec failed: %w", err)
	}
	if result.ExitCode != 0 {
		return result.Stdout, result.Stderr, fmt.Errorf("exec failed: exit code %d", result.ExitCode)
	}
	return result.Stdout, result.Stderr, nil
}

func (c *Client) GetAptSources(boxName string) (snapshotURL string, sources []string, release string) {

	out, _, err := c.ExecCapture(boxName, "cat /etc/apt/sources.list 2>/dev/null; echo; cat /etc/apt/sources.list.d/*.list 2>/dev/null || true")
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			sources = append(sources, line)
			if strings.Contains(line, "snapshot.debian.org") || strings.Contains(line, "snapshot.ubuntu.com") {

				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
						snapshotURL = p
						break
					}
				}
			}
		}
	}

	if relOut, _, err2 := c.ExecCapture(boxName, ". /etc/os-release 2>/dev/null; echo $VERSION_CODENAME"); err2 == nil {
		release = strings.TrimSpace(relOut)
	}
	return
}

func (c *Client) GetPipRegistries(boxName string) (indexURL string, extra []string) {

	out, _, err := c.ExecCapture(boxName, "(pip3 config debug || pip config debug) 2>/dev/null | sed -n 's/^ *index-url *= *//p; s/^ *extra-index-url *= *//p'")
	if err == nil && strings.TrimSpace(out) != "" {

		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if indexURL == "" && (strings.Contains(l, "://") || strings.HasPrefix(l, "file:")) {
				indexURL = l
			} else {
				extra = append(extra, l)
			}
		}
	}
	if indexURL == "" {

		if conf, _, err2 := c.ExecCapture(boxName, "grep -hE '^(index-url|extra-index-url)' /etc/pip.conf ~/.pip/pip.conf 2>/dev/null || true"); err2 == nil {
			for _, line := range strings.Split(conf, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "index-url") && indexURL == "" {
					if i := strings.Index(line, "="); i != -1 {
						indexURL = strings.TrimSpace(line[i+1:])
					}
				}
				if strings.HasPrefix(line, "extra-index-url") {
					if i := strings.Index(line, "="); i != -1 {
						extra = append(extra, strings.TrimSpace(line[i+1:]))
					}
				}
			}
		}
	}
	return
}

func (c *Client) GetNodeRegistries(boxName string) (npmReg, yarnReg, pnpmReg string) {
	if out, _, err := c.ExecCapture(boxName, "npm config get registry 2>/dev/null || true"); err == nil {
		npmReg = strings.TrimSpace(out)
	}
	if out, _, err := c.ExecCapture(boxName, "yarn config get npmRegistryServer 2>/dev/null || true"); err == nil {
		yarnReg = strings.TrimSpace(out)
	}
	if out, _, err := c.ExecCapture(boxName, "pnpm config get registry 2>/dev/null || true"); err == nil {
		pnpmReg = strings.TrimSpace(out)
	}
	return
}

func (c *Client) GetImageDigestInfo(ref string) (string, string, error) {
	ctx := context.Background()

	// Try image inspect first
	imgInspect, _, err := c.sdk.cli.ImageInspectWithRaw(ctx, ref)
	if err == nil {
		digest := ""
		if len(imgInspect.RepoDigests) > 0 {
			digest = imgInspect.RepoDigests[0]
		}
		return digest, imgInspect.ID, nil
	}

	// Fall back to container inspect → get image ID → inspect image
	containerInspect, err := c.sdk.containerInspect(ctx, ref)
	if err != nil {
		return "", "", fmt.Errorf("inspect failed: %w", err)
	}
	imageID := containerInspect.Image
	if imageID == "" {
		return "", "", nil
	}

	imgInspect, _, err = c.sdk.cli.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		return "", imageID, nil
	}
	digest := ""
	if len(imgInspect.RepoDigests) > 0 {
		digest = imgInspect.RepoDigests[0]
	}
	return digest, imgInspect.ID, nil
}

func (c *Client) GetContainerMeta(boxName string) (map[string]string, string, string, string, map[string]string, []string, map[string]string, string) {
	ctx := context.Background()
	inspect, err := c.sdk.containerInspect(ctx, boxName)
	if err != nil {
		return map[string]string{}, "", "", "", map[string]string{}, []string{}, map[string]string{}, ""
	}

	// Parse environment variables
	env := map[string]string{}
	for _, e := range inspect.Config.Env {
		if kv := strings.SplitN(e, "=", 2); len(kv) == 2 {
			env[kv[0]] = kv[1]
		}
	}

	// Parse resources
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
