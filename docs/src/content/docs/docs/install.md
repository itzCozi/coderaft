---
title: Installation Guide
description: How to install coderaft on Linux, macOS, and Windows
---

coderaft runs on **Linux**, **macOS**, and **Windows**. All platforms require Docker.

## Linux

Supports Debian, Ubuntu, Fedora, CentOS, RHEL, Arch, openSUSE, Alpine, and derivatives.

```bash
# GitHub (Primary)
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/install.sh | bash

# Mirror (CDN)
curl -fsSL https://coderaft.ar0.eu/install.sh | bash
```

:::note
We recommend GitHub as a reliable hosting solution; many providers may not fully trust our domain. Use the GitHub link for faster downloads and better reliability.
:::

This script will automatically:
- Detect your Linux distribution and package manager
- Install Go, Docker, make, and git if needed
- Install Go from official binary if the system package is too old
- Clone the repository and build coderaft
- Install coderaft to `/usr/local/bin`
- Set up proper permissions

## macOS

```bash
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/install-macos.sh | bash
```

This script will:
- Install Homebrew (if not already installed)
- Install Go, Git, and Docker Desktop
- Clone the repository, build, and install coderaft

### Manual install (macOS)

```bash
# Install prerequisites
brew install go git
brew install --cask docker   # Docker Desktop

# Clone, build, install
git clone https://github.com/itzcozi/coderaft.git
cd coderaft
make build
sudo make install
```

:::tip
On Apple Silicon Macs (M1/M2/M3/M4), Docker Desktop and Go handle ARM64 natively. No extra setup needed.
:::

## Windows

### PowerShell Installer

```powershell
# Run in PowerShell (as Administrator recommended)
irm https://raw.githubusercontent.com/itzcozi/coderaft/main/install.ps1 | iex
```

This script will:
- Detect your architecture (AMD64 or ARM64)
- Download a pre-built binary or build from source
- Install coderaft and add it to your PATH
- Verify the installation

### Manual install (Windows)

**Prerequisites:**
1. Install [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/)
2. Install [Go](https://go.dev/dl/)
3. Install [Git](https://git-scm.com/download/win)

```powershell
git clone https://github.com/itzcozi/coderaft.git
cd coderaft

# Option A: Use the PowerShell build script
.\build.ps1
.\build.ps1 -Install

# Option B: Build manually
go build -o coderaft.exe ./cmd/coderaft
Move-Item coderaft.exe "$env:LOCALAPPDATA\coderaft\bin\coderaft.exe"
```

:::note
On Windows, Docker Desktop must be running before you use coderaft. The Docker engine runs Linux containers via WSL2 or Hyper-V.
:::

## Shell Completion

coderaft supports shell completion on all platforms:

```bash
# Bash (Linux)
coderaft completion bash > /etc/bash_completion.d/coderaft

# Bash (macOS with Homebrew)
coderaft completion bash > $(brew --prefix)/etc/bash_completion.d/coderaft

# Zsh
coderaft completion zsh > "${fpath[1]}/_coderaft"

# Fish
coderaft completion fish > ~/.config/fish/completions/coderaft.fish
```

```powershell
# PowerShell
coderaft completion powershell >> $PROFILE
```

## Manual Build from Source (all platforms)
---

If you prefer to build coderaft manually:

### Install Dependencies

**Linux (Debian/Ubuntu):**
```bash
sudo apt update \
	&& sudo apt install -y docker.io golang-go make git \
	&& sudo systemctl enable --now docker \
	&& sudo usermod -aG docker $USER
# Note: log out/in (or run `newgrp docker`) for group changes to take effect.
```

**Linux (Fedora/RHEL/CentOS):**
```bash
sudo dnf install -y docker golang make git \
	&& sudo systemctl enable --now docker \
	&& sudo usermod -aG docker $USER
```

**Linux (Arch):**
```bash
sudo pacman -Syu --noconfirm go docker make git \
	&& sudo systemctl enable --now docker \
	&& sudo usermod -aG docker $USER
```

**macOS:**
```bash
brew install go git
brew install --cask docker
```

**Windows:**
Install [Go](https://go.dev/dl/), [Git](https://git-scm.com/download/win), and [Docker Desktop](https://docs.docker.com/desktop/install/windows-install/).

### Build and Install
```bash
# Clone the repository
git clone https://github.com/itzcozi/coderaft.git
cd coderaft

# Build for current platform
make build          # Linux/macOS
.\build.ps1          # Windows (PowerShell)

# Install (Linux/macOS)
sudo make install

# Install (Windows)
.\build.ps1 -Install  # Adds to PATH automatically
```

### Cross-compile for all platforms

```bash
# Linux/macOS
make build-all

# Windows (PowerShell)
.\build.ps1 -Target build-all
```

Produces binaries in `build/`:
#   coderaft-linux-amd64
#   coderaft-linux-arm64
#   coderaft-darwin-amd64
#   coderaft-darwin-arm64
#   coderaft-windows-amd64.exe
#   coderaft-windows-arm64.exe
```

## File Locations
---

| | Linux/macOS | Windows |
|---|---|---|
| **Project files** | `~/coderaft/<project>/` | `%USERPROFILE%\coderaft\<project>\` |
| **Configuration** | `~/.coderaft/config.json` | `%USERPROFILE%\.coderaft\config.json` |
| **Island workspace** | `/island/` (inside container) | `/island/` (inside container) |

## Next Steps
---

Now that you have coderaft installed, quickly get started by following the [Quick Start Guide](/docs/start/).
