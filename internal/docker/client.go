package docker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/docker/docker/client"

	"coderaft/internal/parallel"
	"coderaft/internal/ui"
)

type Client struct {
	sdk *sdkClient
}

const yarnGlobalListQuery = `node -e "(async()=>{const cp=require('child_process');function sh(c){try{return cp.execSync(c,{stdio:['ignore','pipe','ignore']}).toString()}catch(e){return ''}}const dir=sh('yarn global dir').trim();if(!dir){process.exit(0)}const fs=require('fs'),path=require('path');const pkgLock=path.join(dir,'package.json');let deps={};try{const pkg=JSON.parse(fs.readFileSync(pkgLock,'utf8'));deps=Object.assign({},pkg.dependencies||{},pkg.devDependencies||{})}catch{}Object.keys(deps).forEach(n=>{let v='';try{const pj=JSON.parse(fs.readFileSync(path.join(dir,'node_modules',n,'package.json'),'utf8'));v=pj.version||''}catch{}if(v)console.log(n+'@'+v)});})();" 2>/dev/null || true`

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

func dockerCmd() string {
	if eng := strings.TrimSpace(os.Getenv("CODERAFT_ENGINE")); eng != "" {
		return eng
	}
	return "docker"
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

func (c *Client) PullImage(ref string) error {
	ctx := context.Background()

	exists, err := c.sdk.imageExists(ctx, ref)
	if err == nil && exists {
		return nil
	}

	ui.Status("pulling image %s...", ref)
	if err := c.sdk.pullImage(ctx, ref); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	return nil
}

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

func (c *Client) ExecuteSetupCommands(islandName string, commands []string) error {
	return c.ExecuteSetupCommandsWithOutput(islandName, commands, true)
}

func (c *Client) ExecuteSetupCommandsWithOutput(islandName string, commands []string, showOutput bool) error {
	if len(commands) == 0 {
		return nil
	}

	if showOutput {
		ui.Status("executing setup commands on island '%s'...", islandName)
	}

	config := parallel.LoadConfig()
	if config.EnableParallel {

		executor := parallel.NewSetupCommandExecutorWithSDK(islandName, showOutput, config.SetupCommandWorkers, c.SDKExecFunc())
		if err := executor.ExecuteParallel(commands); err != nil {

			ui.Warning("parallel execution failed, falling back to sequential: %v", err)
			return c.ExecuteSetupCommandsSequential(islandName, commands, showOutput)
		}
	} else {

		return c.ExecuteSetupCommandsSequential(islandName, commands, showOutput)
	}

	if showOutput {
		ui.Success("setup commands completed")
	}
	return nil
}

func (c *Client) ExecuteSetupCommandsSequential(islandName string, commands []string, showOutput bool) error {
	if len(commands) == 0 {
		return nil
	}

	if showOutput {
		ui.Status("executing setup commands on island '%s'...", islandName)
	}

	batchSize := 10
	for i := 0; i < len(commands); i += batchSize {
		end := i + batchSize
		if end > len(commands) {
			end = len(commands)
		}
		batch := commands[i:end]

		if showOutput {
			ui.Step(i+1, len(commands), fmt.Sprintf("steps %d-%d", i+1, end))
		}

		var scriptBuilder strings.Builder
		scriptBuilder.WriteString(". /root/.bashrc >/dev/null 2>&1 || true; set -e; ")
		for j, command := range batch {
			if j > 0 {
				scriptBuilder.WriteString(" ; ")
			}

			scriptBuilder.WriteString(command)
		}

		cmd := []string{"bash", "-lc", scriptBuilder.String()}
		ctx := context.Background()
		result, err := c.sdk.containerExec(ctx, islandName, cmd, showOutput)
		if err != nil {
			return fmt.Errorf("setup command batch failed (steps %d-%d): %w", i+1, end, err)
		}
		if result != nil && result.ExitCode != 0 {
			if !showOutput && result.Stderr != "" {
				ui.Error("command batch failed (steps %d-%d)", i+1, end)
				ui.Detail("stderr", result.Stderr)
			}
			return fmt.Errorf("setup command batch failed (steps %d-%d): exit code %d", i+1, end, result.ExitCode)
		}
	}

	if showOutput {
		ui.Success("setup commands completed")
	}
	return nil
}

func (c *Client) QueryPackagesParallel(islandName string) (aptList, pipList, npmList, yarnList, pnpmList []string) {
	config := parallel.LoadConfig()
	if !config.EnableParallel {

		return c.queryPackagesSequential(islandName)
	}

	executor := parallel.NewPackageQueryExecutorWithSDK(islandName, c.SDKExecFunc())

	packageLists, err := executor.QueryAllPackages()
	if err != nil {
		ui.Warning("parallel package query failed, falling back to sequential: %v", err)

		return c.queryPackagesSequential(islandName)
	}

	return packageLists["apt"], packageLists["pip"], packageLists["npm"], packageLists["yarn"], packageLists["pnpm"]
}

func (c *Client) queryPackagesSequential(islandName string) (aptList, pipList, npmList, yarnList, pnpmList []string) {
	type query struct {
		name    string
		command string
		jsonPkg bool
	}

	queries := []query{
		{"apt", `dpkg-query -W -f='${Package}=${Version}\n' $(apt-mark showmanual 2>/dev/null || true) 2>/dev/null | sort`, false},
		{"pip", "python3 -m pip freeze 2>/dev/null || pip3 freeze 2>/dev/null || true", false},
		{"npm", "npm list -g --depth=0 --json 2>/dev/null || true", true},
		{"yarn", yarnGlobalListQuery, false},
		{"pnpm", "pnpm ls -g --depth=0 --json 2>/dev/null || true", true},
	}

	results := make(map[string][]string)
	ctx := context.Background()

	for _, q := range queries {
		result, err := c.sdk.containerExec(ctx, islandName, []string{"bash", "-c", q.command}, false)
		if err != nil {
			ui.Warning("sequential query for %s failed: %v", q.name, err)
			continue
		}

		if q.jsonPkg {
			results[q.name] = parallel.ParseJSONPackageList(result.Stdout)
		} else {
			results[q.name] = parallel.ParseLineList(result.Stdout)
		}
	}

	return results["apt"], results["pip"], results["npm"], results["yarn"], results["pnpm"]
}

func (c *Client) StartIsland(islandID string) error {
	ctx := context.Background()
	if err := c.sdk.containerStart(ctx, islandID); err != nil {
		return fmt.Errorf("failed to start island: %w", err)
	}
	return nil
}

func (c *Client) SetupCoderaftOnIsland(islandName, projectName string) error {
	return c.setupCoderaftOnIslandWithOptions(islandName, projectName, false)
}

func (c *Client) SetupCoderaftOnIslandWithUpdate(islandName, projectName string) error {
	return c.setupCoderaftOnIslandWithOptions(islandName, projectName, true)
}

func (c *Client) IsIslandInitialized(islandName string) bool {
	ctx := context.Background()
	result, err := c.sdk.containerExec(ctx, islandName, []string{"test", "-f", "/etc/coderaft-initialized"}, false)
	return err == nil && result != nil && result.ExitCode == 0
}

func (c *Client) ImageExists(ref string) bool {
	ctx := context.Background()
	exists, err := c.sdk.imageExists(ctx, ref)
	return err == nil && exists
}

func (c *Client) setupCoderaftOnIslandWithOptions(islandName, projectName string, forceUpdate bool) error {

	ctx := context.Background()

	wrapperScript := `#!/bin/bash

# coderaft-wrapper.sh
# This script provides coderaft commands on the island

ISLAND_NAME="` + islandName + `"
PROJECT_NAME="` + projectName + `"

case "$1" in
	"status"|"info")
		echo "Coderaft island status"
        echo "Project: $PROJECT_NAME"
        echo "Island: $ISLAND_NAME"
        echo "Files: /island"
        echo "Host: $(cat /etc/hostname)"
        echo "User: $(whoami)"
        echo "Working Directory: $(pwd)"
        echo ""
	echo "hint: available coderaft commands on island:"
        echo "  coderaft exit     - Exit the shell"
        echo "  coderaft status   - Show island information"
        echo "  coderaft help     - Show this help"
        ;;
	"help"|"--help"|"-h")
		echo "Coderaft island commands"
        echo ""
        echo "Available commands on the island:"
        echo "  coderaft exit         - Exit the coderaft shell"
        echo "  coderaft status       - Show island and project information"
        echo "  coderaft help         - Show this help message"
        echo ""
	echo "Your project files are in: /island"
	echo "You are on an Ubuntu island with full package management"
        echo ""
        echo "Examples:"
        echo "  coderaft exit                    # Exit to host"
        echo "  coderaft status                  # Check island info"
        echo ""
	echo "hint: Files in /island are shared with your host system"
        ;;
    "host")
		echo "error: the 'coderaft host' command is not yet available"
		echo "hint: Exit the island with 'coderaft exit' and run commands on the host directly"
		exit 1
        ;;
    "version")
        echo "Coderaft island wrapper v1.0"
        echo "Island: $ISLAND_NAME"
        echo "Project: $PROJECT_NAME"
        ;;
	"")
		echo "error: missing command. Use \"coderaft help\" for available commands."
        exit 1
        ;;
    *)
		echo "error: unknown coderaft command: $1"
		echo "hint: Use \"coderaft help\" to see available commands on the island"
        echo ""
        echo "Available commands:"
        echo "  exit, status, help, version"
        echo ""
        echo "Note: 'coderaft exit' is handled by the shell function for proper exit behavior"
        exit 1
        ;;
esac`

	setupScript := `set -e

# Mark as initialized
touch /etc/coderaft-initialized

# Install coderaft wrapper
rm -f /usr/local/bin/coderaft
cat > /usr/local/bin/coderaft << 'CODERAFT_WRAPPER_EOF'
` + wrapperScript + `
CODERAFT_WRAPPER_EOF
chmod +x /usr/local/bin/coderaft

# Configure bashrc
sed -i '/# Coderaft welcome message/,/^$/d' /root/.bashrc 2>/dev/null || true
sed -i '/coderaft_exit()/,/^}$/d' /root/.bashrc 2>/dev/null || true
sed -i '/coderaft() {/,/^}$/d' /root/.bashrc 2>/dev/null || true
sed -i '/# Coderaft package tracking start/,/# Coderaft package tracking end/d' /root/.bashrc 2>/dev/null || true

cat >> /root/.bashrc << 'BASHRC_EOF'

# Handle sudo gracefully - just run the command if sudo is not installed
if ! command -v sudo &>/dev/null; then
    sudo() { "$@"; }
fi

if [ -t 1 ]; then
	echo "Welcome to coderaft project: ` + projectName + `"
	echo "Your files are in: /island"
	echo "hint: Type 'coderaft help' for available commands"
	echo "hint: Type 'coderaft exit' to leave the island"
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

coderaft_exit() {
	echo "Exiting coderaft shell for project \"` + projectName + `\""
	exit 0
}

coderaft() {
    if [[ "$1" == "exit" || "$1" == "quit" ]]; then
        coderaft_exit
        return
    fi
    /usr/local/bin/coderaft "$@"
}

export CODERAFT_LOCKFILE="${CODERAFT_LOCKFILE:-/island/coderaft.history}"

coderaft_record_cmd() {
	local cmd="$1"
	if [ -n "$CODERAFT_LOCKFILE" ] && [ -w "$(dirname "$CODERAFT_LOCKFILE")" ]; then
		if [ ! -f "$CODERAFT_LOCKFILE" ] || ! grep -Fxq "$cmd" "$CODERAFT_LOCKFILE" 2>/dev/null; then
			echo "$cmd" >> "$CODERAFT_LOCKFILE"
		fi
	fi
}

_coderaft_wrap_and_record() {
	local bin="$1"; shift
	local name="$1"; shift
	"$bin" "$@"
	local status=$?
	if [ $status -eq 0 ]; then
		case "$name" in
			apt|apt-get)
				# Track install/remove/purge/autoremove
				if printf ' %s ' "$*" | grep -qE '(^| )(install|remove|purge|autoremove)( |$)'; then
					coderaft_record_cmd "$name $*"
				fi
				;;
			pip|pip3)
				if [ "$1" = install ] || [ "$1" = uninstall ]; then
					coderaft_record_cmd "$name $*"
				fi
				;;
			npm)
				# Track install and uninstall variants
				if [ "$1" = install ] || [ "$1" = i ] || [ "$1" = add ] \
				   || [ "$1" = uninstall ] || [ "$1" = remove ] || [ "$1" = rm ] || [ "$1" = r ] || [ "$1" = un ]; then
					coderaft_record_cmd "$name $*"
				fi
				;;
			yarn)
				# Track add/remove and global add/remove
				if [ "$1" = add ] || [ "$1" = remove ] || { [ "$1" = global ] && { [ "$2" = add ] || [ "$2" = remove ]; }; }; then
					coderaft_record_cmd "$name $*"
				fi
				;;
			pnpm)
				# Track add/install and remove/uninstall variants
				if [ "$1" = add ] || [ "$1" = install ] || [ "$1" = i ] \
				   || [ "$1" = remove ] || [ "$1" = rm ] || [ "$1" = uninstall ] || [ "$1" = un ]; then
					coderaft_record_cmd "$name $*"
				fi
				;;
			corepack)
				# Handle: corepack yarn add ..., corepack yarn global add ...
				#         corepack yarn remove ..., corepack yarn global remove ...
				#         corepack pnpm add/install/i/remove/rm/uninstall/un ...
				subcmd="$1"; shift || true
				if [ "$subcmd" = yarn ]; then
					if [ "$1" = add ] || [ "$1" = remove ] || { [ "$1" = global ] && { [ "$2" = add ] || [ "$2" = remove ]; }; }; then
						coderaft_record_cmd "corepack yarn $*"
					fi
				elif [ "$subcmd" = pnpm ]; then
					if [ "$1" = add ] || [ "$1" = install ] || [ "$1" = i ] \
					   || [ "$1" = remove ] || [ "$1" = rm ] || [ "$1" = uninstall ] || [ "$1" = un ]; then
						coderaft_record_cmd "corepack pnpm $*"
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

apt()      { _coderaft_wrap_and_record "$APT_BIN" apt "$@"; }
apt-get()  { _coderaft_wrap_and_record "$APTGET_BIN" apt-get "$@"; }
pip()      { _coderaft_wrap_and_record "$PIP_BIN" pip "$@"; }
pip3()     { _coderaft_wrap_and_record "$PIP3_BIN" pip3 "$@"; }
npm()      { _coderaft_wrap_and_record "$NPM_BIN" npm "$@"; }
yarn()     { _coderaft_wrap_and_record "$YARN_BIN" yarn "$@"; }
pnpm()     { _coderaft_wrap_and_record "$PNPM_BIN" pnpm "$@"; }
corepack(){ _coderaft_wrap_and_record "$COREPACK_BIN" corepack "$@"; }
BASHRC_EOF
`

	result, err := c.sdk.containerExec(ctx, islandName, []string{"bash", "-c", setupScript}, false)
	if err != nil {
		return fmt.Errorf("failed to setup coderaft on island: %w", err)
	}
	if result != nil && result.ExitCode != 0 {
		return fmt.Errorf("failed to setup coderaft on island: exit code %d: %s", result.ExitCode, result.Stderr)
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

func AttachShell(islandName string) error {

	cmd := exec.Command(dockerCmd(), "exec", "-it",
		"-e", fmt.Sprintf("CODERAFT_ISLAND_NAME=%s", islandName),
		islandName, "/bin/bash", "-c",
		"export PS1='coderaft(\\$PROJECT_NAME):\\w\\$ '; exec /bin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Exit codes 130 (SIGINT/Ctrl+C) and 137 (SIGKILL) are normal shell exits
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

func RunCommand(islandName string, command []string) error {
	cmdStr := strings.Join(command, " ")
	wrapped := ". /root/.bashrc >/dev/null 2>&1 || true; " + cmdStr
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

func (c *Client) ExecCapture(islandName, command string) (string, string, error) {
	wrapped := ". /root/.bashrc >/dev/null 2>&1 || true; set -o pipefail; " + command
	ctx := context.Background()
	result, err := c.sdk.containerExec(ctx, islandName, []string{"bash", "-lc", wrapped}, false)
	if err != nil {
		return "", "", fmt.Errorf("exec failed: %w", err)
	}
	if result.ExitCode != 0 {
		return result.Stdout, result.Stderr, fmt.Errorf("exec failed: exit code %d", result.ExitCode)
	}
	return result.Stdout, result.Stderr, nil
}

func (c *Client) GetAptSources(islandName string) (snapshotURL string, sources []string, release string) {

	out, _, err := c.ExecCapture(islandName, "cat /etc/apt/sources.list 2>/dev/null; echo; cat /etc/apt/sources.list.d/*.list 2>/dev/null || true")
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

	if relOut, _, err2 := c.ExecCapture(islandName, ". /etc/os-release 2>/dev/null; echo $VERSION_CODENAME"); err2 == nil {
		release = strings.TrimSpace(relOut)
	}
	return
}

func (c *Client) GetPipRegistries(islandName string) (indexURL string, extra []string) {

	out, _, err := c.ExecCapture(islandName, "(pip3 config debug || pip config debug) 2>/dev/null | sed -n 's/^ *index-url *= *//p; s/^ *extra-index-url *= *//p'")
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

		if conf, _, err2 := c.ExecCapture(islandName, "grep -hE '^(index-url|extra-index-url)' /etc/pip.conf ~/.pip/pip.conf 2>/dev/null || true"); err2 == nil {
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

func (c *Client) GetNodeRegistries(islandName string) (npmReg, yarnReg, pnpmReg string) {
	if out, _, err := c.ExecCapture(islandName, "npm config get registry 2>/dev/null || true"); err == nil {
		npmReg = strings.TrimSpace(out)
	}
	if out, _, err := c.ExecCapture(islandName, "yarn config get npmRegistryServer 2>/dev/null || true"); err == nil {
		yarnReg = strings.TrimSpace(out)
	}
	if out, _, err := c.ExecCapture(islandName, "pnpm config get registry 2>/dev/null || true"); err == nil {
		pnpmReg = strings.TrimSpace(out)
	}
	return
}

func (c *Client) GetImageDigestInfo(ref string) (string, string, error) {
	ctx := context.Background()

	imgInspect, _, err := c.sdk.cli.ImageInspectWithRaw(ctx, ref)
	if err == nil {
		digest := ""
		if len(imgInspect.RepoDigests) > 0 {
			digest = imgInspect.RepoDigests[0]
		}
		return digest, imgInspect.ID, nil
	}

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
