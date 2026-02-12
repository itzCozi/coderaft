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
  "base_image": "ubuntu:22.04",
  "setup_commands": [
    "apt install -y python3 python3-pip"
  ],
  "environment": {
    "PYTHONPATH": "/workspace"
  },
  "ports": ["5000:5000"],
  "volumes": ["/workspace/data:/data"]
}
```

##### Common Fields

```json
{
  "name": "example-project",
  "base_image": "ubuntu:22.04",
  "setup_commands": [
    "apt install -y python3 python3-pip nodejs npm"
  ],
  "environment": {
    "PYTHONPATH": "/workspace",
    "NODE_ENV": "development",
    "PYTHONUNBUFFERED": "1"
  },
  "ports": [
    "3000:3000",
    "5000:5000",
    "8080:8080"
  ],
  "volumes": [
    "/workspace/data:/data",
    "/workspace/logs:/var/log/app"
  ],
  "dotfiles": ["~/.dotfiles"],
  "working_dir": "/workspace",
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
Regardless of configuration, coderaft always runs `apt update -y && apt full-upgrade -y` first when initializing any box to ensure the system is up to date. Your `setup_commands` will run after this system update.
:::

## Reproducible Installs
---

Coderaft automatically records package manager installs you run inside the box to `/workspace/coderaft.lock` (which is your project folder on the host).

Lock file paths:
- Inside box: `/workspace/coderaft.lock`
- On host: `~/coderaft/<project>/coderaft.lock`

The following commands are tracked when they succeed:

- `apt install ...` and `apt-get install ...`
- `pip install ...` and `pip3 install ...`
- `npm install ...`, `npm i ...`, `npm add ...`
- `yarn add ...` and `yarn global add ...`
- `pnpm add ...`, `pnpm install ...`, and `pnpm i ...`

On `coderaft up` and during `coderaft update` rebuilds, coderaft replays the commands from `coderaft.lock` before running `setup_commands`. This makes it easy to reproduce the exact environment or share it with teammates by committing `coderaft.lock` to your repo.

Notes:
- Only successful install commands are recorded, and duplicates are de-duplicated line-by-line.
- You can edit `coderaft.lock` manually to remove mistakes or add comments (lines starting with `#` are ignored).
- If you prefer explicit configuration, keep using `setup_commands` in `coderaft.json`; the lock file complements it for ad-hoc installs.

## Environment Snapshot
---

For a more comprehensive, shareable snapshot similar to Nix-style locks, use:

```bash
coderaft lock <project>
``

This writes a JSON snapshot (by default to `<workspace>/coderaft.lock.json`) that includes:

- Base image: name, digest (if available), and image ID
- Container configuration: working_dir, user, restart policy, network, ports, volumes, labels, environment, capabilities, resources (cpus/memory)
- Installed packages:
  - apt: manually installed packages pinned as `name=version`
  - pip: `pip freeze`
  - npm/yarn/pnpm: globally installed packages `name@version` (Yarn global versions are read from Yarn's global directory)
- Registries and sources for reproducibility:
  - pip: `index-url` and `extra-index-url`
  - npm/yarn/pnpm: global registry URLs
  - apt: full `sources.list` lines, snapshot base URL if present, and OS release codename
- Any `setup_commands` from your `coderaft.json` (for context)

Usage notes:
- Commit `coderaft.lock.json` to your repository to share environment details with teammates.
- This file is an authoritative snapshot for auditing/sharing. The current execution path for rebuilds remains `coderaft.json` + the simple `coderaft.lock` replay file. You can now also use:
  - `coderaft verify <project>` to validate a box matches the lock (fails fast on drift)
  - `coderaft apply <project>` to configure registries/sources and reconcile package sets to the lock
- Local app dependencies (e.g. non-global Node packages in your repo) are intentionally not included; rely on your project’s own lockfiles (package-lock.json, yarn.lock, pnpm-lock.yaml, requirements.txt/poetry.lock, etc.).

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

You can mount your personal dotfiles into the box to keep your editor/shell preferences:

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
  "base_image": "ubuntu:22.04",
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
      "container_name": "coderaft_my-project",
      "base_image": "ubuntu:22.04",
      "workspace_path": "/home/user/coderaft/my-project",
      "config_file": "/home/user/coderaft/my-project/coderaft.json"
    }
  },
  "settings": {
    "default_base_image": "ubuntu:22.04",
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
| `default_base_image` | string | `ubuntu:22.04` | Default base image for new projects |
| `auto_update` | boolean | `true` | Whether to run updates during initialization |
| `auto_stop_on_exit` | boolean | `true` | If enabled, coderaft stops a project's box automatically after exiting an interactive shell or finishing a one-off `run` command. Override per-invocation with `--keep-running`. |

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
- Missing required fields are validated before box creation
- Invalid port/volume formats are caught during validation
- Failed setup commands stop initialization with clear error messages

## Configuration Precedence
---

1. Project `coderaft.json` configuration (highest priority)
2. Global project settings in `~/.coderaft/config.json`
3. Global default settings
4. Built-in defaults (lowest priority)

This allows for flexible configuration management while maintaining backward compatibility.
