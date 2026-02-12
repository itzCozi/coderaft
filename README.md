# coderaft

**Isolated development environments for anything**

[![CI](https://github.com/itzcozi/coderaft/workflows/CI/badge.svg)](https://github.com/itzcozi/coderaft/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/itzcozi/coderaft)](https://goreportcard.com/report/github.com/itzcozi/coderaft)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

coderaft creates isolated development environments, contained in a project's Docker island. Each project operates in its own disposable environment, while your code remains neatly organized in a simple, flat folder on the host machine.

## Features

- **Instant Setup** - Create isolated development environments in seconds
- **Docker-based** - Leverage the power of islands (containers) for consistent environments
- **Clean Organization** - Keep your code organized in simple, flat folders
- **Configurable** - Define your environment with simple JSON configuration
- **Disposable** - Easily destroy and recreate environments as needed
- **Isolated** - Each project runs in its own island, preventing conflicts
- **Docker-in-Docker** - Use Docker within your coderaft environments by default
- **Cross-platform** - Supports Linux, macOS, and Windows (primary target: Debian/Ubuntu)
- **Well Tested** - Comprehensive test suite

## Why coderaft?

coderaft focuses on fast, disposable, Docker-native development environments with simple, commit-friendly config.

- Minimal config: a small JSON file, no heavy frameworks
- Clean host workspace: flat folders, no complex mounts
- Reproducible: isolated per-project islands you can destroy/recreate anytime
- Docker-in-Docker ready: use Docker inside your environment out of the box
- Cross-platform: runs on Linux, macOS, and Windows wherever Docker is available

## Installation

```bash
# Using the install script
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/install.sh | bash

# Mirror (CDN)
curl -fsSL https://coderaft.ar0.eu/install.sh | bash

# Or manually: https://coderaft.ar0.eu/docs/install/#manual-build-from-source
```

Note: coderaft requires Docker. It supports Linux, macOS, and Windows. On Windows without WSL2, ensure Docker Desktop is running.

## Quick Start

1. **Initialize a new project**
   ```bash
   coderaft init my-project
   ```

2. **Enter the development environment**
   ```bash
   coderaft shell my-project
   ```

3. **Run commands in the environment**
   ```bash
   coderaft run my-project "python --version"
   ```

4. **List your environments**
   ```bash
   coderaft list
   ```

5. **Clean up when done**
   ```bash
   coderaft destroy my-project
   ```

### Shared configs

Commit a `coderaft.json` to your repo so teammates can just:

```bash
coderaft up
```

Optional: mount your local dotfiles into the island

```bash
coderaft up --dotfiles ~/.dotfiles
```

## Documentation

For detailed documentation, guides, and examples, visit:

**[coderaft.ar0.eu](https://coderaft.ar0.eu)**

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.


**Created by BadDeveloper with 💚**
