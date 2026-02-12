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
echo " = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = "
echo " coderaft Installation Script"
echo " = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = "
echo -e "${NC}"
}

check_os() {
print_info "Checking operating system compatibility..."

if [[ ! -f /etc/os-release ]] ; then
  print_error "Cannot determine operating system"
  exit 1
fi

. /etc/os-release

case $ID in
ubuntu | debian)
print_success "Compatible OS detected: $PRETTY_NAME"
;;
*)
print_error "Unsupported operating system: $PRETTY_NAME"
print_error "coderaft requires Debian or Ubuntu Linux"
exit 1
;;
esac
}


command_exists() {
command -v "$1" > /dev/null 2>&1
}


install_coderaft() {
print_info "Cloning coderaft repository..."

TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"


git clone https://github.com/itzcozi/coderaft.git
cd coderaft

print_info "Building coderaft..."


if ! command_exists go; then
  print_error "Go is not available. Installation may have failed."
  exit 1
fi

print_info "Go version: $(go version)"


make build

print_info "Installing coderaft to /usr/local/bin..."


sudo make install


cd /
rm -rf "$TEMP_DIR"

print_success "coderaft installed successfully"
}


verify_installation() {
print_info "Verifying installation..."

if command_exists coderaft; then
  CODERAFT_VERSION=$(coderaft version 2>/dev/null || echo "unknown")
  print_success "coderaft is installed and accessible: $CODERAFT_VERSION"

  if docker ps > /dev/null 2>&1; then
    print_success "Docker is accessible"
    else
    print_warning "Docker may require logout/login for group permissions"
  fi

  return 0
  else
  print_error "coderaft installation verification failed"
  return 1
fi
}


print_next_steps() {
echo
print_success "🎉 coderaft installation completed successfully!"
echo
print_info "Next steps:"
echo " 1. If Docker group permissions are needed, log out and log back in"
echo " 2. Create your first project:"
echo " coderaft init myproject"
echo " 3. Enter the development environment:"
echo " coderaft shell myproject"
echo " 4. Get help anytime:"
echo " coderaft --help"
echo
print_info "For more information, visit: https://github.com/itzcozi/coderaft"
echo
}


main() {
print_header
check_os

print_info "Updating package lists..."
sudo apt update

print_info "Installing all dependencies in one command (git, make, go, docker)..."
sudo apt install -y git build-essential golang-go docker.io

print_info "Configuring Docker..."
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker "$USER"


print_info "Verifying installations..."
if command_exists git; then
  print_success "git: $(git --version | head -n1)"
  else
  print_error "git installation failed"
  exit 1
fi

if command_exists make; then
  print_success "make: $(make --version | head -n1)"
  else
  print_error "make installation failed"
  exit 1
fi

if command_exists go; then
  print_success "go: $(go version)"
  else
  print_error "go installation failed"
  exit 1
fi

if command_exists docker; then
  print_success "docker: $(docker --version)"
  print_warning "You may need to log out and log back in for Docker group permissions to take effect"
  else
  print_error "docker installation failed"
  exit 1
fi

print_info "Installing coderaft..."
install_coderaft

if verify_installation; then
  print_next_steps
  else
  print_error "Installation completed but verification failed"
  print_info "Try running 'coderaft --help' manually to test"
  exit 1
fi
}


main "$@"
