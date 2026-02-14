#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() {
	echo -e "${BLUE}info: $1${NC}"
}
print_success() {
	echo -e "${GREEN}ok: $1${NC}"
}
print_warning() {
	echo -e "${YELLOW}warning: $1${NC}"
}
print_error() {
	echo -e "${RED}error: $1${NC}"
}

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

print_header

if [[ "$(uname)" != "Darwin" ]]; then
	print_error "This script is for macOS only. On Linux, use uninstall.sh instead."
	exit 1
fi

print_info "Looking for coderaft installation..."

CODERAFT_PATH=""

# Check common installation locations
if [ -f "/usr/local/bin/coderaft" ]; then
	CODERAFT_PATH="/usr/local/bin/coderaft"
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

BREW_PREFIX=""
if command_exists brew; then
	BREW_PREFIX="$(brew --prefix)"
fi

COMPLETION_FILES=(
	"$HOME/.config/fish/completions/coderaft.fish"
)

if [ -n "$BREW_PREFIX" ]; then
	COMPLETION_FILES+=("$BREW_PREFIX/etc/bash_completion.d/coderaft")
fi

# Check common zsh completion locations
COMPLETION_FILES+=(
	"/usr/local/share/zsh/site-functions/_coderaft"
	"$HOME/.zsh/completions/_coderaft"
)

# Check $fpath for zsh completions
if [ -n "$ZSH_VERSION" ] && [ -n "$fpath" ]; then
	for dir in ${fpath[@]}; do
		if [ -f "$dir/_coderaft" ]; then
			COMPLETION_FILES+=("$dir/_coderaft")
		fi
	done
fi

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
echo "  - Docker Desktop (you may still need it for other tools)"
echo "  - Homebrew, Go, or Git (you may still need them)"
echo "  - Any coderaft projects you created"
echo
print_info "To remove Docker containers created by coderaft:"
echo "  docker ps -a | grep coderaft | awk '{print \$1}' | xargs docker rm -f"
echo
print_info "To remove Docker images created by coderaft:"
echo "  docker images | grep coderaft | awk '{print \$3}' | xargs docker rmi -f"
echo
