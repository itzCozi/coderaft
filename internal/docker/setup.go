package docker

import (
	"context"
	"fmt"
	"strings"

	"coderaft/internal/parallel"
	"coderaft/internal/security"
	"coderaft/internal/ui"
)

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

	batchSize := security.Limits.MaxSetupBatchSize
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
		ctx, cancel := context.WithTimeout(context.Background(), security.Timeouts.Apply)
		result, err := c.sdk.containerExec(ctx, islandName, cmd, showOutput)
		cancel()

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
        echo "  coderaft history  - Show package history"
        echo "  coderaft files    - List project files"
        echo "  coderaft disk     - Show disk usage"
        echo "  coderaft env      - Show environment"
        echo "  coderaft help     - Show this help"
        ;;
	"help"|"--help"|"-h")
		echo "Coderaft island commands"
        echo ""
        echo "Available commands on the island:"
        echo "  coderaft exit         - Exit the coderaft shell"
        echo "  coderaft status       - Show island and project information"
        echo "  coderaft history      - Show recorded package install history"
        echo "  coderaft files        - List project files in /island"
        echo "  coderaft disk         - Show island disk usage"
        echo "  coderaft env          - Show coderaft environment variables"
        echo "  coderaft help         - Show this help message"
        echo ""
	echo "Your project files are in: /island"
	echo "You are on an Ubuntu island with full package management"
        echo ""
        echo "Examples:"
        echo "  coderaft exit                    # Exit to host"
        echo "  coderaft status                  # Check island info"
        echo "  coderaft history                 # See tracked packages"
        echo "  coderaft files                   # List /island contents"
        echo ""
	echo "hint: Files in /island are shared with your host system"
        ;;
    "history"|"log")
        HISTORY_FILE="${CODERAFT_HISTORY:-/island/coderaft.history}"
        if [ -f "$HISTORY_FILE" ]; then
            echo "Recorded package history:"
            cat "$HISTORY_FILE"
        else
            echo "No package history recorded yet."
            echo "hint: Install packages with apt, pip, npm, etc. and they'll be tracked automatically"
        fi
        ;;
    "files"|"ls")
        echo "Project files (/island):"
        ls -la /island 2>/dev/null || echo "No files found in /island"
        ;;
    "disk"|"usage")
        echo "Island disk usage:"
        df -h / 2>/dev/null | head -2
        echo ""
        echo "/island usage:"
        du -sh /island 2>/dev/null || echo "Unable to calculate"
        ;;
    "env")
        echo "Coderaft environment:"
        env | grep -i CODERAFT | sort || echo "No CODERAFT variables set"
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
        echo "  exit, status, history, files, disk, env, help, version"
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

export CODERAFT_HISTORY="${CODERAFT_HISTORY:-/island/coderaft.history}"
# Backward-compatible: also honor the legacy variable name
if [ -n "$CODERAFT_LOCKFILE" ] && [ -z "${CODERAFT_HISTORY_SET:-}" ]; then
  CODERAFT_HISTORY="$CODERAFT_LOCKFILE"
fi

coderaft_record_cmd() {
	local cmd="$1"
	if [ -n "$CODERAFT_HISTORY" ] && [ -w "$(dirname "$CODERAFT_HISTORY")" ]; then
		if [ ! -f "$CODERAFT_HISTORY" ] || ! grep -Fxq "$cmd" "$CODERAFT_HISTORY" 2>/dev/null; then
			echo "$cmd" >> "$CODERAFT_HISTORY"
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
