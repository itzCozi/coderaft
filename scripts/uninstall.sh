#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}info: $1${NC}"; }
print_success() { echo -e "${GREEN}ok: $1${NC}"; }
print_warning() { echo -e "${YELLOW}warning: $1${NC}"; }
print_error() { echo -e "${RED}error: $1${NC}"; }

print_header() {
	echo -e "${BLUE}"
	echo " ============ "
	echo "   coderaft   "
	echo "   uninstall  "
	echo " ============ "
	echo -e "${NC}"
}

command_exists() {
	command -v "$1" >/dev/null 2>&1
}

main() {
	print_header

	if [ "$(uname)" = "Darwin" ]; then
		print_error "This script is for Linux. On macOS, use uninstall-macos.sh instead."
		print_info "  curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/scripts/uninstall-macos.sh | bash"
		print_info "or use the short link"
		print_info "  curl -fsSL coderaft.ar0.eu/uninstall-macos.sh | bash"
		exit 1
	fi

	print_info "Looking for coderaft installation..."

	CODERAFT_PATH=""

	# Check common installation locations
	if [ -f "/usr/local/bin/coderaft" ]; then
		CODERAFT_PATH="/usr/local/bin/coderaft"
	elif [ -f "/usr/bin/coderaft" ]; then
		CODERAFT_PATH="/usr/bin/coderaft"
	elif [ -f "$HOME/.local/bin/coderaft" ]; then
		CODERAFT_PATH="$HOME/.local/bin/coderaft"
	elif [ -f "$HOME/go/bin/coderaft" ]; then
		CODERAFT_PATH="$HOME/go/bin/coderaft"
	elif command_exists coderaft; then
		CODERAFT_PATH="$(command -v coderaft)"
	fi

	if [ -z "$CODERAFT_PATH" ]; then
		print_warning "coderaft is not installed or could not be found."
		exit 0
	fi

	print_info "Found coderaft at: $CODERAFT_PATH"

	# Confirm removal
	echo
	read -p "Are you sure you want to uninstall coderaft? [y/N] " -n 1 -r
	echo
	if [[ ! $REPLY =~ ^[Yy]$ ]]; then
		print_info "Uninstall cancelled."
		exit 0
	fi

	print_info "Removing coderaft binary..."

	# Check if we need sudo
	if [ -w "$(dirname "$CODERAFT_PATH")" ]; then
		rm -f "$CODERAFT_PATH"
	else
		sudo rm -f "$CODERAFT_PATH"
	fi

	print_success "Removed $CODERAFT_PATH"

	# Check for shell completion files
	print_info "Checking for shell completion files..."

	COMPLETION_FILES=(
		"/etc/bash_completion.d/coderaft"
		"/usr/share/bash-completion/completions/coderaft"
		"$HOME/.config/fish/completions/coderaft.fish"
	)

	# Check zsh completion directories
	if [ -n "$fpath" ]; then
		for dir in ${fpath[@]}; do
			COMPLETION_FILES+=("$dir/_coderaft")
		done
	fi
	COMPLETION_FILES+=("/usr/local/share/zsh/site-functions/_coderaft")
	COMPLETION_FILES+=("$HOME/.zsh/completions/_coderaft")

	for file in "${COMPLETION_FILES[@]}"; do
		if [ -f "$file" ]; then
			print_info "Found completion file: $file"
			if [ -w "$(dirname "$file")" ]; then
				rm -f "$file"
			else
				sudo rm -f "$file" 2>/dev/null || true
			fi
			print_success "Removed $file"
		fi
	done

	echo
	print_success "coderaft has been uninstalled successfully!"
	echo
	print_info "Note: This script does not remove:"
	echo "  - Docker (you may still need it for other tools)"
	echo "  - Go (you may still need it for other projects)"
	echo "  - Any coderaft projects you created"
	echo
	print_info "To remove Docker containers created by coderaft:"
	echo "  docker ps -a | grep coderaft | awk '{print \$1}' | xargs -r docker rm -f"
	echo
	print_info "To remove Docker images created by coderaft:"
	echo "  docker images | grep coderaft | awk '{print \$3}' | xargs -r docker rmi -f"
	echo
}

main "$@"
