---
title: Configuration
description: Project and global configuration
---

## Project Config (coderaft.json)

```json
{
  "name": "my-project",
  "base_image": "buildpack-deps:bookworm",
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

### All Fields

| Field | Description |
|-------|-------------|
| `name` | Project name |
| `base_image` | Docker image (default: buildpack-deps:bookworm) |
| `setup_commands` | Commands run on init |
| `environment` | Environment variables |
| `ports` | Port mappings (host:container) |
| `volumes` | Volume mounts |
| `dotfiles` | Dotfiles paths to mount |
| `working_dir` | Working directory (default: /island) |
| `shell` | Shell to use (default: /bin/bash) |
| `user` | Container user |
| `restart` | Restart policy |
| `resources` | `{"cpus": "2", "memory": "4g"}` |
| `capabilities` | Linux capabilities (e.g., `["SYS_PTRACE"]`) |
| `labels` | Docker labels (key-value pairs) |
| `network` | Docker network mode (e.g., `bridge`, `host`) |
| `health_check` | Container health check config |
| `gpus` | GPU access (e.g., `all` or device IDs) |

## Global Config (~/.coderaft/config.json)

```json
{
  "settings": {
    "default_base_image": "buildpack-deps:bookworm",
    "auto_stop_on_exit": true,
    "auto_update": false
  }
}
```

Modify by editing the file directly at `~/.coderaft/config.json`, or view current settings with:
```bash
coderaft config global
```

## Package History

Package installs are recorded to `coderaft.history`:

```bash
# Inside the island shell (after running coderaft shell)
coderaft history

# Or view directly on the host
cat ~/coderaft/<project>/coderaft.history
```

Location: `~/coderaft/<project>/coderaft.history`

### Tracked Package Managers

| Category | Package Managers |
|----------|------------------|
| **System** | apt, apt-get, dpkg, apk, dnf, yum, pacman, zypper, rpm, brew, snap, flatpak |
| **Python** | pip, pip3, pipx, poetry, uv, conda, mamba, micromamba |
| **Node.js** | npm, yarn, pnpm, bun, deno, corepack |
| **Languages** | cargo (Rust), go install (Go), gem (Ruby), composer (PHP) |
| **Version Managers** | nvm, pyenv, rustup, sdk (SDKMAN!), asdf |
| **Manual Installs** | wget, curl (all downloads), make install |

## Lock Files

Pin exact environment state:

```bash
coderaft lock <project>     # Create coderaft.lock.json
coderaft verify <project>   # Check for drift
coderaft apply <project>    # Reconcile to lock
```

Lock files include: base image digest, all installed packages from supported package managers, registry URLs, and apt/apk sources. The checksum enables fast drift detection.

## Secrets Management

Store sensitive environment variables in an encrypted vault:

```bash
# Initialize vault (one-time setup)
coderaft secrets init

# Store secrets
coderaft secrets set myproject API_KEY
coderaft secrets set myproject DATABASE_URL=postgres://localhost/db

# Import from .env file
coderaft secrets import myproject .env.production

# List secrets (keys only)
coderaft secrets list myproject

# Export for shell
eval $(coderaft secrets export myproject)
```

**Features:**
- AES-256-GCM encryption with PBKDF2 key derivation
- Master password protection (cannot be recovered if lost)
- `.env` file import/export support
- Stored at `~/.coderaft/secrets.vault.json`

Secrets are designed to be injected into islands as environment variables at runtime.

## Port Forwarding

View exposed ports for running islands:

```bash
# Show all ports for all islands
coderaft ports

# Show ports for specific project
coderaft ports myproject
```

**Features:**
- Auto-detects 20+ common services (PostgreSQL, Redis, MongoDB, etc.)
- Generates clickable URLs for web ports
- Tabular output with service hints

Configure ports in `coderaft.json`:
```json
{
  "ports": ["3000:3000", "5432:5432", "6379:6379"]
}
```

