---
title: Installation
description: Install coderaft on Linux, macOS, or Windows
---

coderaft runs on **Linux**, **macOS**, and **Windows**. All platforms require Docker.

> **Source of Truth:** Install scripts pull from the `main` branch and build locally. Always get coderaft from [itzcozi/coderaft](https://github.com/itzcozi/coderaft).

## Linux

```bash
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/install.sh | bash
# or mirror: curl -fsSL https://coderaft.ar0.eu/install.sh | bash
```

Supports Debian, Ubuntu, Fedora, CentOS, RHEL, Arch, openSUSE, Alpine, and derivatives. The script installs dependencies, builds coderaft, and sets up permissions.

## macOS

```bash
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/install-macos.sh | bash
# or mirror: curl -fsSL https://coderaft.ar0.eu/install-macos.sh | bash
```

Installs Homebrew (if needed), Go, Git, Docker Desktop, then builds and installs coderaft.

:::tip
Apple Silicon (M1/M2/M3/M4) works natively — no extra setup.
:::

## Windows

```powershell
irm https://raw.githubusercontent.com/itzcozi/coderaft/main/install.ps1 | iex
# or mirror: irm https://coderaft.ar0.eu/install.ps1 | iex
```

Requires Git and Go. The script clones, builds, and adds coderaft to your PATH.

:::note
Docker Desktop must be running before using coderaft.
:::

## Manual Build

For all platforms, you can build manually:

**Prerequisites:** Git, Go, Docker

```bash
git clone https://github.com/itzcozi/coderaft.git
cd coderaft
go build -o coderaft ./cmd/coderaft
```

**Install:**
- **Linux/macOS:** `sudo cp coderaft /usr/local/bin/`
- **Windows:** Move `coderaft.exe` to a directory in your PATH

## Shell Completion

```bash
# Bash
coderaft completion bash > /etc/bash_completion.d/coderaft

# Zsh
coderaft completion zsh > "${fpath[1]}/_coderaft"

# Fish
coderaft completion fish > ~/.config/fish/completions/coderaft.fish

# PowerShell
coderaft completion powershell >> $PROFILE
```

## File Locations

| | Linux/macOS | Windows |
|---|---|---|
| Projects | `~/coderaft/<project>/` | `%USERPROFILE%\coderaft\<project>\` |
| Config | `~/.coderaft/config.json` | `%USERPROFILE%\.coderaft\config.json` |
| Workspace | `/island/` (inside container) | `/island/` (inside container) |

## Next Steps

→ [Quick Start](/docs/start/)
