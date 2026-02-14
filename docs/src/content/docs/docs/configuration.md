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

Tracked: apt, pip, npm, yarn, pnpm

## Lock Files

Pin exact environment state:

```bash
coderaft lock <project>     # Create coderaft.lock.json
coderaft verify <project>   # Check for drift
coderaft apply <project>    # Reconcile to lock
```

Lock files include: base image digest, packages, registries, apt sources.
