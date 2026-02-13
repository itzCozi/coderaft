#!/bin/bash















set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info()    { echo -e "${BLUE}info: $1${NC}"; }
print_success() { echo -e "${GREEN}ok: $1${NC}"; }
print_warning() { echo -e "${YELLOW}warning: $1${NC}"; }
print_error()   { echo -e "${RED}error: $1${NC}"; }

print_header() {
  echo -e "${BLUE}"
  echo " ============ "
  echo "   coderaft   "
  echo " ============ "
  echo -e "${NC}"
}

command_exists() {
  command -v "$1" > /dev/null 2>&1
}





print_header

if [[ "$(uname)" != "Darwin" ]]; then
  print_error "This script is for macOS only. On Linux, use install.sh instead."
  exit 1
fi

ARCH="$(uname -m)"
print_info "Detected architecture: $ARCH"





if ! command_exists brew; then
  print_info "Homebrew not found. Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"


  if [[ "$ARCH" == "arm64" ]]; then
    eval "$(/opt/homebrew/bin/brew shellenv)"
  fi
  print_success "Homebrew installed"
else
  print_success "Homebrew found: $(brew --version | head -1)"
fi






if ! command_exists git; then
  print_info "Installing Git..."
  brew install git
  print_success "Git installed"
else
  print_success "Git found: $(git --version)"
fi


if ! command_exists go; then
  print_info "Installing Go..."
  brew install go
  print_success "Go installed: $(go version)"
else
  print_success "Go found: $(go version)"
fi


if ! command_exists docker; then
  print_warning "Docker CLI not found."
  print_info "Installing Docker Desktop via Homebrew Cask..."
  brew install --cask docker
  print_success "Docker Desktop installed."
  print_warning "Please launch Docker Desktop from Applications and complete setup."
  print_warning "After Docker Desktop is running, re-run this script, or continue below."
else
  print_success "Docker found: $(docker --version)"
  if docker info > /dev/null 2>&1; then
    print_success "Docker daemon is running"
  else
    print_warning "Docker CLI found but daemon is not running."
    print_warning "Please start Docker Desktop before using coderaft."
  fi
fi





print_info "Cloning coderaft repository..."
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

git clone --depth 1 https://github.com/itzcozi/coderaft.git
cd coderaft

print_info "Building coderaft..."
make build

print_info "Installing coderaft to /usr/local/bin..."
sudo cp ./build/coderaft /usr/local/bin/coderaft
sudo chmod +x /usr/local/bin/coderaft


cd /
rm -rf "$TEMP_DIR"

print_success "coderaft installed to /usr/local/bin/coderaft"





print_info "Verifying installation..."

if command_exists coderaft; then
  CODERAFT_VERSION=$(coderaft version 2>/dev/null || echo "unknown")
  print_success "coderaft is installed: $CODERAFT_VERSION"
else
  print_error "coderaft installation verification failed."
  print_info "Try opening a new terminal and running: coderaft --help"
  exit 1
fi





echo
print_success "coderaft installation completed successfully!"
echo
print_info "Next steps:"
echo "  1. Make sure Docker Desktop is running"
echo "  2. Create your first project:"
echo "       coderaft init myproject"
echo "  3. Enter the development environment:"
echo "       coderaft shell myproject"
echo "  4. Get help anytime:"
echo "       coderaft --help"
echo
print_info "Shell completion (optional):"
echo "  # Bash:"
echo "  coderaft completion bash > \$(brew --prefix)/etc/bash_completion.d/coderaft"
echo "  # Zsh:"
echo "  coderaft completion zsh > \${fpath[1]}/_coderaft"
echo "  # Fish:"
echo "  coderaft completion fish > ~/.config/fish/completions/coderaft.fish"
echo
print_info "For more information: https://github.com/itzcozi/coderaft"
echo
