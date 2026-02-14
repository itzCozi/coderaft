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





detect_distro() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO_ID="${ID}"
    DISTRO_ID_LIKE="${ID_LIKE:-}"
    DISTRO_NAME="${PRETTY_NAME:-$ID}"
  elif [ -f /etc/alpine-release ]; then
    DISTRO_ID="alpine"
    DISTRO_NAME="Alpine Linux"
  else
    DISTRO_ID="unknown"
    DISTRO_NAME="Unknown"
  fi


  case "$DISTRO_ID" in
    ubuntu|debian|linuxmint|pop|elementary|zorin|kali)
      PKG_FAMILY="apt"
      ;;
    fedora|rhel|centos|rocky|alma|ol)
      PKG_FAMILY="dnf"
      ;;
    arch|manjaro|endeavouros)
      PKG_FAMILY="pacman"
      ;;
    opensuse*|sles)
      PKG_FAMILY="zypper"
      ;;
    alpine)
      PKG_FAMILY="apk"
      ;;
    *)

      case "$DISTRO_ID_LIKE" in
        *debian*|*ubuntu*)  PKG_FAMILY="apt" ;;
        *fedora*|*rhel*)    PKG_FAMILY="dnf" ;;
        *arch*)             PKG_FAMILY="pacman" ;;
        *suse*)             PKG_FAMILY="zypper" ;;
        *)                  PKG_FAMILY="unknown" ;;
      esac
      ;;
  esac
}





install_deps_apt() {
  print_info "Installing dependencies via apt..."
  sudo apt update
  sudo apt install -y git build-essential golang-go docker.io curl wget

  print_info "Configuring Docker..."
  sudo systemctl start docker 2>/dev/null || true
  sudo systemctl enable docker 2>/dev/null || true
  sudo usermod -aG docker "$USER" 2>/dev/null || true
}

install_deps_dnf() {
  print_info "Installing dependencies via dnf..."
  if command_exists dnf; then
    sudo dnf install -y git make golang docker curl wget gcc
  else
    sudo yum install -y git make golang docker curl wget gcc
  fi

  print_info "Configuring Docker..."
  sudo systemctl start docker 2>/dev/null || true
  sudo systemctl enable docker 2>/dev/null || true
  sudo usermod -aG docker "$USER" 2>/dev/null || true
}

install_deps_pacman() {
  print_info "Installing dependencies via pacman..."
  sudo pacman -Syu --noconfirm git base-devel go docker curl wget

  print_info "Configuring Docker..."
  sudo systemctl start docker 2>/dev/null || true
  sudo systemctl enable docker 2>/dev/null || true
  sudo usermod -aG docker "$USER" 2>/dev/null || true
}

install_deps_zypper() {
  print_info "Installing dependencies via zypper..."
  sudo zypper install -y git make go docker curl wget gcc

  print_info "Configuring Docker..."
  sudo systemctl start docker 2>/dev/null || true
  sudo systemctl enable docker 2>/dev/null || true
  sudo usermod -aG docker "$USER" 2>/dev/null || true
}

install_deps_apk() {
  print_info "Installing dependencies via apk..."
  sudo apk add --no-cache git make go docker curl wget build-base

  print_info "Configuring Docker..."
  sudo rc-update add docker default 2>/dev/null || true
  sudo service docker start 2>/dev/null || true
  sudo addgroup "$USER" docker 2>/dev/null || true
}

install_deps_generic() {
  print_warning "Unknown distribution: $DISTRO_NAME"
  print_info "Attempting to detect available package managers..."

  if command_exists apt-get; then
    install_deps_apt
  elif command_exists dnf; then
    install_deps_dnf
  elif command_exists yum; then
    install_deps_dnf
  elif command_exists pacman; then
    install_deps_pacman
  elif command_exists zypper; then
    install_deps_zypper
  elif command_exists apk; then
    install_deps_apk
  else
    print_error "No supported package manager found."
    print_info "Please install the following manually: git, make, go, docker, curl"
    exit 1
  fi
}





ensure_go() {
  if command_exists go; then
    GO_VERSION=$(go version 2>/dev/null | grep -oP 'go(\d+\.\d+)' | head -1 | sed 's/go//')
    GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
    GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)


    if [ "$GO_MAJOR" -ge 1 ] && [ "$GO_MINOR" -ge 22 ]; then
      print_success "Go found: $(go version)"
      return
    fi

    print_warning "Go version $GO_VERSION is too old, installing newer version..."
  fi

  print_info "Installing Go from official binary..."
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)  GOARCH="amd64" ;;
    aarch64) GOARCH="arm64" ;;
    armv7l)  GOARCH="armv6l" ;;
    *)       print_error "Unsupported architecture: $ARCH"; exit 1 ;;
  esac

  GO_TAR="go1.24.0.linux-${GOARCH}.tar.gz"
  GO_URL="https://go.dev/dl/${GO_TAR}"

  TMPDIR=$(mktemp -d)
  curl -fsSL "$GO_URL" -o "$TMPDIR/$GO_TAR"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "$TMPDIR/$GO_TAR"
  rm -rf "$TMPDIR"

  export PATH="/usr/local/go/bin:$PATH"
  print_success "Go installed: $(go version)"
}





install_coderaft() {
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





main() {
  print_header

  if [ "$(uname)" = "Darwin" ]; then
    print_error "This script is for Linux. On macOS, use install-macos.sh instead."
    print_info "  curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/scripts/install-macos.sh | bash"
    print_info "  # Short link (optional)"
    print_info "  curl -fsSL https://coderaft.ar0.eu/install-macos.sh | bash"
    exit 1
  fi

  detect_distro
  print_info "Detected distribution: $DISTRO_NAME (family: $PKG_FAMILY)"


  case "$PKG_FAMILY" in
    apt)     install_deps_apt ;;
    dnf)     install_deps_dnf ;;
    pacman)  install_deps_pacman ;;
    zypper)  install_deps_zypper ;;
    apk)     install_deps_apk ;;
    *)       install_deps_generic ;;
  esac


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

  if command_exists docker; then
    print_success "docker: $(docker --version)"
    print_warning "You may need to log out and log back in for Docker group permissions to take effect"
  else
    print_error "docker installation failed"
    exit 1
  fi


  ensure_go


  install_coderaft

  if verify_installation; then
    echo
    print_success "coderaft installation completed successfully!"
    echo
    print_info "Next steps:"
    echo "  1. If Docker group permissions are needed, log out and log back in"
    echo "  2. Create your first project:"
    echo "       coderaft init myproject"
    echo "  3. Enter the development environment:"
    echo "       coderaft shell myproject"
    echo "  4. Get help anytime:"
    echo "       coderaft --help"
    echo
    print_info "Shell completion (optional):"
    echo "  # Bash:"
    echo "  coderaft completion bash > /etc/bash_completion.d/coderaft"
    echo "  # Zsh:"
    echo '  coderaft completion zsh > "${fpath[1]}/_coderaft"'
    echo "  # Fish:"
    echo "  coderaft completion fish > ~/.config/fish/completions/coderaft.fish"
    echo
    print_info "For more information: https://github.com/itzcozi/coderaft"
    echo
  else
    print_error "Installation completed but verification failed"
    print_info "Try running 'coderaft --help' manually to test"
    exit 1
  fi
}

main "$@"
