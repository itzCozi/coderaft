# coderaft

**Isolated development environments using Docker islands**

[![CI](https://github.com/itzcozi/coderaft/workflows/CI/badge.svg)](https://github.com/itzcozi/coderaft/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/itzcozi/coderaft)](https://goreportcard.com/report/github.com/itzcozi/coderaft)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

coderaft creates isolated development environments inside Docker islands. Each project lives in its own disposable container while your code stays organized on the host.

> **Source of Truth:** The `main` branch is the most up-to-date stable code. Always pull from [itzcozi/coderaft](https://github.com/itzcozi/coderaft) and build for your system, or use the install script for your platform.

## Why coderaft?

| Problem | coderaft Solution |
|---------|-------------------|
| "Works on my machine" | Every project runs in an identical, isolated island |
| Dependency conflicts | Each island has its own dependencies, no host pollution |
| Complex setup scripts | One JSON config, one command to start |
| Heavy VMs or slow tools | Lightweight Docker containers, instant startup |
| Cluttered host system | Your host stays clean; everything runs in islands |
| Team onboarding friction | Commit `coderaft.json`, teammates run `coderaft up` |

**In short:** Fast, disposable, Docker-native dev environments with minimal config.

## Features

- **Instant setup** — Create environments in seconds
- **Isolated** — Each project in its own Docker island
- **Reproducible** — Commit `coderaft.json` for consistent team environments
- **Package tracking** — Automatically records installs from 30+ package managers (apt, pip, npm, cargo, go, brew, and more)
- **Lock files** — Pin exact environment state with checksummed `coderaft.lock.json`
- **Secrets management** — AES-256 encrypted vault for API keys and credentials
- **Docker-in-Docker** — Use Docker inside your island out of the box
- **Cross-platform** — Linux, macOS, and Windows
- **Simple config** — One small JSON file, no frameworks

## Install

### Linux

```bash
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/scripts/install.sh | bash
# or mirror: curl -fsSL https://coderaft.ar0.eu/install.sh | bash
```

### macOS

```bash
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/scripts/install-macos.sh | bash
# or mirror: curl -fsSL https://coderaft.ar0.eu/install-macos.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/itzcozi/coderaft/main/scripts/install.ps1 | iex
# or mirror: irm https://coderaft.ar0.eu/install.ps1 | iex
```

**Manual builds:** See the [Installation Guide](https://coderaft.ar0.eu/docs/install/) for build instructions on all platforms.

> **Requires Docker.** On Windows/macOS, use Docker Desktop.

## Quick Start

```bash
# Create a project
coderaft init my-project

# Enter the environment
coderaft shell my-project

# Run a command
coderaft run my-project "python --version"

# List environments
coderaft list

# Clean up
coderaft destroy my-project
```

### Team Workflow

Commit `coderaft.json` to your repo. Teammates just run:

```bash
coderaft up
```

## Documentation

Full docs, guides, and examples: **[coderaft.ar0.eu](https://coderaft.ar0.eu)**

## License

MIT — see [LICENSE](LICENSE)

---

**Created by BadDeveloper with 💚**
