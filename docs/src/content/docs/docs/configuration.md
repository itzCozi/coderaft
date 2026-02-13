---
title: Configuration Files
description: Comprehensive configuration management with coderaft.json and global settings
---

Coderaft supports configuration via a per-project `coderaft.json` and a global `~/.coderaft/config.json`.

## Project Configuration
---

Each project can have a `coderaft.json` file in its workspace directory that defines the development environment configuration.

##### Basic Structure

```json
{
  "name": "my-project",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y python3 python3-pip"
  ],
  "environment": {
    "PYTHONPATH": "/island"
  },
  "ports": ["5000:5000"],
  "volumes": ["/island/data:/data"]
}
```

##### Common Fields

```json
{
  "name": "example-project",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y python3 python3-pip nodejs npm"
  ],
  "environment": {
    "PYTHONPATH": "/island",
    "NODE_ENV": "development",
    "PYTHONUNBUFFERED": "1"
  },
  "ports": [
    "3000:3000",
    "5000:5000",
    "8080:8080"
  ],
  "volumes": [
    "/island/data:/data",
    "/island/logs:/var/log/app"
  ],
  "dotfiles": ["~/.dotfiles"],
  "working_dir": "/island",
  "shell": "/bin/bash",
  "user": "root",
  "capabilities": ["SYS_PTRACE"],
  "labels": {
    "coderaft.project": "example-project",
    "coderaft.type": "development"
  },
  "network": "bridge",
  "restart": "unless-stopped",
  "resources": {
    "cpus": "2.0",
    "memory": "2g"
  },
  "health_check": {
    "test": ["CMD", "curl", "-f", "http://localhost:5000/health"],
    "interval": "30s",
    "timeout": "10s",
    "retries": 3
  }
}
```

Key fields you may use: `name`, `base_image`, `setup_commands`, `environment`, `ports`, `volumes`, `dotfiles`, `working_dir`. Advanced options like `capabilities`, `labels`, `network`, `restart`, `resources`, and `health_check` are supported but optional.

:::note
Regardless of configuration, coderaft always runs `apt update -y && apt full-upgrade -y` first when initializing any Island to ensure the system is up to date. Your `setup_commands` will run after this system update.
:::

## Reproducible Installs
---

Coderaft provides two complementary mechanisms for reproducing island environments:

1. **Command history** (`coderaft.history`) - a lightweight replay log of ad-hoc package-manager commands you run inside the island.
2. **Environment lock file** (`coderaft.lock.json`) - a comprehensive, deterministic snapshot of every installed package, registry URL, apt source, and base image digest, sealed with a SHA-256 checksum.

### Command History

Coderaft automatically records package manager installs you run inside the Island to `/island/coderaft.history` (which is your project folder on the host).

History file paths:
- Inside Island: `/island/coderaft.history`
- On host: `~/coderaft/<project>/coderaft.history`

The following commands are tracked when they succeed:

| Package Manager | Tracked Commands |
|----------------|-----------------|
| apt / apt-get | `install`, `remove`, `purge`, `autoremove` |
| pip / pip3 | `install`, `uninstall` |
| npm | `install`, `i`, `add`, `uninstall`, `remove`, `rm` |
| yarn | `add`, `remove`, `global add`, `global remove` |
| pnpm | `add`, `install`, `i`, `remove`, `rm`, `uninstall` |
| corepack | Delegates to yarn/pnpm tracking |

On `coderaft up` and during `coderaft update` rebuilds, coderaft replays the commands from `coderaft.history` before running `setup_commands`. This makes it easy to reproduce the exact environment or share it with teammates by committing `coderaft.history` to your repo.

Notes:
- Only successful install commands are recorded, and duplicates are de-duplicated line-by-line.
- You can edit `coderaft.history` manually to remove mistakes or add comments (lines starting with `#` are ignored).
- If you prefer explicit configuration, keep using `setup_commands` in `coderaft.json`; the history file complements it for ad-hoc installs.

### Environment Lock File

For a comprehensive, version-pinned snapshot similar to Nix-style locks, use:

```bash
coderaft lock <project>
```

This writes a **v2** JSON snapshot (by default to `<workspace>/coderaft.lock.json`) that includes:

| Section | Contents |
|---------|----------|
| **Base image** | Name, digest (`sha256:...`), and image ID - ensures the exact base layer is pinned |
| **Container** | working_dir, user, restart policy, network, ports, volumes, labels, environment, capabilities, resources (cpus/memory) |
| **Packages** | **apt**: manually installed packages as `name=version` (sorted) |
| | **pip**: `pip freeze` output (sorted) |
| | **npm/yarn/pnpm**: globally installed packages as `name@version` (sorted) |
| **Registries** | pip `index-url` / `extra-index-url`, npm/yarn/pnpm registry URLs |
| **APT sources** | Full `sources.list` lines, snapshot base URL, OS release codename |
| **Checksum** | SHA-256 over all reproducibility-critical fields for fast equality checks |
| **Setup commands** | Any `setup_commands` from `coderaft.json` (for context) |

All package lists are **sorted alphabetically** so the lock file output is deterministic - regenerating the lock on an unchanged island produces byte-identical JSON (aside from the timestamp).

### Lock File Workflow

```bash
# 1. Generate the lock file
coderaft lock myproject

# 2. Commit it to your repository
git add coderaft.lock.json
git commit -m "pin island environment"

# 3. A teammate clones and applies
coderaft up                       # boots the island
coderaft apply myproject          # reconciles packages to match the lock

# 4. Verify the island matches later
coderaft verify myproject

# 5. Preview changes before applying
coderaft apply myproject --dry-run
```

### Lock, Verify, Apply

| Command | Purpose |
|---------|---------|
| `coderaft lock <project>` | Snapshot the running island into `coderaft.lock.json` |
| `coderaft verify <project>` | Compare the live island against the lock; exits non-zero on drift. Shows per-package diffs (added/removed/version-changed) |
| `coderaft apply <project>` | Configure registries/sources and reconcile package sets to match the lock |
| `coderaft apply <project> --dry-run` | Preview what `apply` would change without modifying the island |

:::tip
**Checksum fast-path**: v2 lock files include a `checksum` field (SHA-256 over packages, registries, and image digest). Two lock files with the same checksum describe identical environments - no need to diff every package line.
:::

Notes:
- Commit `coderaft.lock.json` to your repository to share environment details with teammates.
- Local app dependencies (e.g. non-global Node packages in your repo) are intentionally not included; rely on your project's own lockfiles (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `requirements.txt`/`poetry.lock`, etc.).
## Initialize with Configuration
---

```bash
# Basic initialization
coderaft init myproject

# Initialize with template
coderaft init myproject --template python

# Generate config file only
coderaft init myproject --config-only --template python

# Initialize and generate config
coderaft init myproject --generate-config
```

### Shared Configs

To make onboarding easy, commit your `coderaft.json` to the repository. New teammates can simply run:

```bash
coderaft up
```

This reads `./coderaft.json` and starts the environment without requiring prior project registration.

### Dotfile Injection

You can mount your personal dotfiles into the Island to keep your editor/shell preferences:

```bash
# One-off via flag
coderaft up --dotfiles ~/.dotfiles

# Or persist via config
{
  "name": "my-project",
  "dotfiles": ["~/.dotfiles"]
}
```

Behavior summary: mount at `/dotfiles` and source/symlink common files on shell init.

## Configuration Management
---

```bash
# Generate coderaft.json for existing project
coderaft config generate myproject

# Validate project configuration
coderaft config validate myproject

# Show project configuration
coderaft config show myproject

# List available templates
coderaft config templates

# Show global configuration
coderaft config global
```

## Usage Examples
---

##### Python Development Project

```bash
# Create Python project
coderaft init python-app --template python

# The generated coderaft.json includes:
# - Python 3 with pip and venv
# - Development tools
# - PYTHONPATH configuration
# - Common ports
```

##### Custom Configuration

1. Initialize project:
```bash
coderaft init custom-project --generate-config
```

2. Edit `coderaft.json`:
```json
{
  "name": "custom-project",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y postgresql-client redis-tools"
  ],
  "environment": {
    "DATABASE_URL": "postgresql://localhost/mydb",
    "REDIS_URL": "redis://localhost:6379"
  },
  "ports": ["5432:5432", "6379:6379"]
}
```

3. Recreate with new configuration:
```bash
coderaft destroy custom-project
coderaft init custom-project
```

## Global Configuration
---

Global settings are stored in `~/.coderaft/config.json`:

```json
{
  "projects": {
    "my-project": {
      "name": "my-project",
      "island_name": "coderaft_my-project",
      "base_image": "ubuntu:latest",
      "workspace_path": "/home/user/coderaft/my-project",
      "config_file": "/home/user/coderaft/my-project/coderaft.json"
    }
  },
  "settings": {
    "default_base_image": "ubuntu:latest",
    "auto_update": true,
    "auto_stop_on_exit": true,
    "default_environment": {
      "TZ": "UTC"
    }
  }
}
```

### Global Settings

| Setting | Type | Default | Description |
|--------|------|---------|-------------|
| `default_base_image` | string | `ubuntu:latest` | Default base image for new projects |
| `auto_update` | boolean | `true` | Whether to run updates during initialization |
| `auto_stop_on_exit` | boolean | `true` | If enabled, coderaft stops a project's Island automatically after exiting an interactive shell or finishing a one-off `run` command. Override per-invocation with `--keep-running`. |

When `auto_stop_on_exit` is enabled:
- `coderaft up` will also stop the container if it is idle right after setup (no ports exposed and only the init process running), unless `--keep-running` is passed.
- If your `coderaft.json` does not specify a `restart` policy, coderaft will default to `--restart no` so that manual stops persist.

Note: If `auto_stop_on_exit` is missing in older installs, add it under `settings`.

## Migration
---

Existing projects continue to work without configuration files. You can:

1. Generate configuration for existing projects:
```bash
coderaft config generate existing-project
```

2. Apply templates to existing projects:
```bash
coderaft config generate existing-project --template python
```

3. Recreate projects with new configuration:
```bash
coderaft destroy old-project
coderaft init old-project --template nodejs
```

## Error Handling
---

- Invalid JSON in `coderaft.json` will show parsing errors
- Missing required fields are validated before Island creation
- Invalid port/volume formats are caught during validation
- Failed setup commands stop initialization with clear error messages

## Configuration Precedence
---

1. Project `coderaft.json` configuration (highest priority)
2. Global project settings in `~/.coderaft/config.json`
3. Global default settings
4. Built-in defaults (lowest priority)

This allows for flexible configuration management while maintaining backward compatibility.
