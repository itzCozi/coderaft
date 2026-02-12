# devbox

**Isolated development environments for anything**

[![CI](https://github.com/itzcozi/devbox/workflows/CI/badge.svg)](https://github.com/itzcozi/devbox/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/itzcozi/devbox)](https://goreportcard.com/report/github.com/itzcozi/devbox)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

devbox creates isolated development environments, contained in a project's Docker box (container). Each project operates in its own disposable environment, while your code remains neatly organized in a simple, flat folder on the host machine.

## Features

- 🚀 **Instant Setup** - Create isolated development environments in seconds
- 🐳 **Docker-based** - Leverage the power of boxes (containers) for consistent environments
- 📁 **Clean Organization** - Keep your code organized in simple, flat folders
- 🔧 **Configurable** - Define your environment with simple JSON configuration
- 🗑️ **Disposable** - Easily destroy and recreate environments as needed
- 🛡️ **Isolated** - Each project runs in its own box, preventing conflicts
- 🔄 **Docker-in-Docker** - Use Docker within your devbox environments by default
- 🐧 **Cross-platform** - Supports Linux, macOS, and Windows (primary target: Debian/Ubuntu)
- 🧪 **Well Tested** - Comprehensive test suite

## Why devbox?

devbox focuses on fast, disposable, Docker-native development environments with simple, commit-friendly config.

- Minimal config: a small JSON file, no heavy frameworks
- Clean host workspace: flat folders, no complex mounts
- Reproducible: isolated per-project boxes you can destroy/recreate anytime
- Docker-in-Docker ready: use Docker inside your environment out of the box
- Cross-platform: runs on Linux, macOS, and Windows wherever Docker is available

## Installation

```bash
# Using the install script
curl -fsSL https://devbox.ar0.eu/install.sh | bash
# Or manually: https://devbox.ar0.eu/docs/install/#manual-build-from-source
```

Note: devbox requires Docker. It supports Linux, macOS, and Windows. On Windows without WSL2, ensure Docker Desktop is running.

## Quick Start

1. **Initialize a new project**
   ```bash
   devbox init my-project
   ```

2. **Enter the development environment**
   ```bash
   devbox shell my-project
   ```

3. **Run commands in the environment**
   ```bash
   devbox run my-project "python --version"
   ```

4. **List your environments**
   ```bash
   devbox list
   ```

5. **Clean up when done**
   ```bash
   devbox destroy my-project
   ```

### Shared configs

Commit a `devbox.json` to your repo so teammates can just:

```bash
devbox up
```

Optional: mount your local dotfiles into the box

```bash
devbox up --dotfiles ~/.dotfiles
```

## Documentation

For detailed documentation, guides, and examples, visit:

**📖 [devbox.ar0.eu](https://devbox.ar0.eu)**

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.


**Created by BadDeveloper with 💚**
