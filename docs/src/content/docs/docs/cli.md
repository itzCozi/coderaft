---
title: CLI Reference
description: Comprehensive reference for all coderaft commands and options
tableOfContents:
  minHeadingLevel: 2
  maxHeadingLevel: 4
---

Complete reference for all coderaft commands, options, and usage patterns.

## Global Options

---

All commands support these global options:

- `--help, -h`: Show help information
- `--verbose`: Show detailed progress messages

## Core Commands

---

### `coderaft status`

Show detailed container status and resource usage for a project. With no project specified, prints a quick overview of all coderaft containers.

**Syntax:**
```bash
coderaft status [project]
```

**Behavior:**
- With a project: shows state, uptime, CPU%, memory usage/%, network I/O, block I/O, PIDs, ports, and mounts
- Without a project: lists all coderaft containers with status and image

**Examples:**
```bash
# Overview of all coderaft containers
coderaft status

# Detailed status for a specific project
coderaft status myproject
```

---

### `coderaft up`

Start a coderaft environment from a shared coderaft.json in the current directory. Perfect for onboarding: clone the repo and run `coderaft up`.

**Syntax:**
```bash
coderaft up [--dotfiles <path>] [--keep-running]
```

**Options:**
- `--dotfiles <path>`: Mount a local dotfiles directory into common locations inside the Island
- `--keep-running`: Keep the Island running after setup completes (overrides auto-stop-on-idle)

**Behavior:**
- Reads `./coderaft.json`
- Creates/starts an Island named `coderaft_<name>` where `<name>` comes from `coderaft.json`'s `name` (or the folder name)
- Applies ports, env, and volumes from configuration
- Runs a system update, then `setup_commands`
- Installs the coderaft wrapper for nice shell UX
- Records package installations you perform inside the Island to `coderaft.history` (apt/pip/npm/yarn/pnpm). On rebuilds, these commands are replayed to reproduce the environment.
- If global setting `auto_stop_on_exit` is enabled (default), `coderaft up` stops the container right away if it is idle (no exposed ports and only the init process running). Use `--keep-running` to leave it running.
- When `auto_stop_on_exit` is enabled and your `coderaft.json` does not specify a `restart` policy, coderaft uses `--restart no` to prevent the container from auto-restarting after being stopped.

**Examples:**
```bash
# Start from current folder's coderaft.json
coderaft up

# Mount your dotfiles
coderaft up --dotfiles ~/.dotfiles
```

---

### `coderaft init`

Create a new coderaft project with its own Docker island.

**Syntax:**
```bash
coderaft init <project> [flags]
```

**Options:**
- `--force, -f`: Force initialization, overwriting existing project
- `--template, -t <template>`: Initialize from template (python, nodejs, go, web)
- `--generate-config, -g`: Generate coderaft.json configuration file
- `--config-only, -c`: Generate configuration file only (don't create Island)

**Examples:**
```bash
# Basic project
coderaft init myproject

# Python project with template
coderaft init python-app --template python

# Force overwrite existing project
coderaft init myproject --force

# Generate config file only
coderaft init myproject --config-only --template nodejs

# Create with custom configuration
coderaft init webapp --generate-config
```

**Templates:**
- `python`: Python 3, pip, venv, development tools
- `nodejs`: Node.js 18, npm, build tools
- `go`: Go 1.21, git, build tools
- `web`: Python + Node.js + nginx for full-stack development

---

### `coderaft shell`

Open an interactive bash shell in the project's Island.

**Syntax:**
```bash
coderaft shell <project> [--keep-running]
```

**Examples:**
```bash
# Enter project environment
coderaft shell myproject

# Start stopped Island and enter shell
coderaft shell python-app
```

**Notes:**
- Automatically starts the Island if stopped
- Sets working directory to `/island`
- Your project files are available at `/island`
- Exit with `exit`, `logout`, or `Ctrl+D`
- By default, the Island stops automatically after you exit the shell when global setting `auto_stop_on_exit` is enabled (default)
- Use `--keep-running` to keep the Island running after you exit the shell

**Island commands:**

Once inside the shell, the `coderaft` command is available as a lightweight wrapper with these subcommands:

| Command | Alias | Description |
|---------|-------|-------------|
| `coderaft exit` | `quit` | Exit the island shell |
| `coderaft status` | `info` | Show island name, project, hostname, user, working directory |
| `coderaft history` | `log` | Print recorded package install history from `coderaft.history` |
| `coderaft files` | `ls` | List project files in `/island` |
| `coderaft disk` | `usage` | Show island root filesystem and `/island` disk usage |
| `coderaft env` | — | Print all `CODERAFT_*` environment variables |
| `coderaft help` | `-h` | Show available island commands |
| `coderaft version` | — | Show island wrapper version |

---

### `coderaft run`

Run an arbitrary command inside the project's Island.

**Syntax:**
```bash
coderaft run <project> <command> [args...] [--keep-running]
```

**Examples:**
```bash
# Run single command
coderaft run myproject python3 --version

# Run with arguments
coderaft run myproject apt install -y htop

# Complex command with pipes
coderaft run myproject "cd /island && python3 -m http.server 8000"

# Execute script
coderaft run myproject bash /island/setup.sh
```

**Notes:**
- Commands run in `/island` by default
- Use quotes for complex commands with pipes, redirects, etc.
- Island starts automatically if stopped
- By default, the Island stops automatically after the command finishes when global setting `auto_stop_on_exit` is enabled (default)
- Use `--keep-running` to keep the Island running after the command finishes

---

### `coderaft stop`

Stop a project's Island if it's running.

**Syntax:**
```bash
coderaft stop <project>
```

**Examples:**
```bash
# Stop a running Island
coderaft stop myproject

# Stop another project's Island
coderaft stop webapp
```

**Notes:**
- Safe to run if the Island is already stopped (no-op)
- Complements the default auto-stop behavior after `shell` and `run`

---

### `coderaft destroy`

Stop and remove the project's Island.

**Syntax:**
```bash
coderaft destroy <project> [flags]
```

**Options:**
- `--force, -f`: Force destruction without confirmation

**Examples:**
```bash
# Destroy with confirmation
coderaft destroy myproject

# Force destroy without prompt
coderaft destroy myproject --force
```

**Notes:**
- Preserves project files in `~/coderaft/<project>/`
- Island can be recreated with `coderaft init`
- Use `rm -rf ~/coderaft/<project>/` to remove files

---

### `coderaft list`

Show all managed projects and their Island status.

**Syntax:**
```bash
coderaft list [flags]
```

**Options:**
- `--verbose, -v`: Show detailed information including configuration

**Examples:**
```bash
# Basic list
coderaft list

# Detailed information
coderaft list --verbose
```

**Output Format:**
```
CODERAFT PROJECTS
PROJECT              Island                  STATUS          CONFIG       WORKSPACE
--------------------  --------------------  ---------------  ------------  ------------------------------
myproject            coderaft_myproject     Up 2 hours      coderaft.json  /home/user/coderaft/myproject
webapp               coderaft_webapp        Exited          none         /home/user/coderaft/webapp

Total projects: 2
```

---

### `coderaft lock`

Generate a deterministic, checksummed environment snapshot as `coderaft.lock.json` for a project. This is ideal for sharing/auditing the exact Island image, container configuration, and globally installed packages.

**Syntax:**
```bash
coderaft lock <project> [-o, --output <path>]
```

**Options:**
- `-o, --output <path>`: Write the lock file to a custom path. Defaults to `<workspace>/coderaft.lock.json`.

**Behavior:**
- Ensures the project's Island is running (starts it if needed).
- Inspects the container and its image to capture:
  - Base image: name, digest, image ID
  - Container config: working_dir, user, restart policy, network, ports, volumes, labels, environment, capabilities, resources (cpus/memory)
  - Installed package snapshots (all sorted alphabetically for determinism):
    - apt: manually installed packages pinned as `name=version`
    - pip: `pip freeze` output
    - npm/yarn/pnpm: globally installed packages as `name@version`
  - Registries and sources:
    - pip: `index-url` and `extra-index-url`
    - npm/yarn/pnpm: global registry URLs
    - apt: `sources.list` lines, snapshot base URL, OS release codename
- Computes a SHA-256 checksum over all reproducibility-critical fields (base image, packages, registries, apt sources).
- If `coderaft.json` exists in the workspace, includes its `setup_commands` for context.

Use `coderaft apply` to reconcile an island to a lock file and `coderaft verify` to check for drift.

**Examples:**
```bash
# Write snapshot into the project workspace
coderaft lock myproject

# Write snapshot to a custom file
coderaft lock myproject -o ./env/coderaft.lock.json
```

**Sample Output (excerpt):**
```json
{
  "version": 2,
  "project": "myproject",
  "ISLAND_NAME": "coderaft_myproject",
  "created_at": "2026-02-12T20:41:51Z",
  "checksum": "sha256:a1b2c3d4e5f6...",
  "base_image": {
    "name": "buildpack-deps:bookworm",
    "digest": "buildpack-deps@sha256:...",
    "id": "sha256:..."
  },
  "container": {
    "working_dir": "/island",
    "user": "root",
    "restart": "no",
    "network": "bridge",
    "ports": ["3000/tcp -> 0.0.0.0:3000"],
    "volumes": ["bind /host/path -> /island (rw=true)"],
    "environment": {"TZ": "UTC"},
    "labels": {"coderaft.project": "myproject"},
    "capabilities": ["SYS_PTRACE"],
    "resources": {"cpus": "2", "memory": "2048MB"}
  },
  "packages": {
    "apt": ["build-essential=12.9ubuntu3", "git=1:2.34.1-..."],
    "pip": ["flask==3.1.0", "requests==2.32.3"],
    "npm": ["typescript@5.6.2"],
    "yarn": ["eslint@9.1.0"],
    "pnpm": []
  },
  "registries": {
    "pip_index_url": "https://pypi.org/simple",
    "pip_extra_index_urls": ["https://mirror.example/simple"],
    "npm_registry": "https://registry.npmjs.org/",
    "yarn_registry": "https://registry.yarnpkg.com",
    "pnpm_registry": "https://registry.npmjs.org/"
  },
  "apt_sources": {
    "snapshot_url": "https://snapshot.debian.org/archive/debian/20240915T000000Z/",
    "sources_lists": [
      "deb https://snapshot.debian.org/archive/debian/20240915T000000Z/ bullseye main"
    ],
    "pinned_release": "jammy"
  },
  "setup_commands": [
    "apt install -y python3 python3-pip"
  ]
}
```

---

### `coderaft verify`

Validate that the running Island matches the `coderaft.lock.json` exactly. Reports detailed per-package drift.

**Syntax:**
```bash
coderaft verify <project>
```

**Checks:**
- Base image digest (if recorded in lock)
- Package sets: apt, pip, npm, yarn, pnpm — with per-package detail:
  - Packages **added** on the island but not in the lock
  - Packages **removed** from the island but present in the lock
  - Packages with **changed versions**
- Registries: pip index/extra-index, npm/yarn/pnpm registry URLs
- Apt sources: sources.list lines, snapshot base URL (if present), OS release codename
- Lock checksum (v2+): recomputed from live state for a fast-path comparison

Returns non-zero on any mismatch and prints a categorized drift report.

**Example:**
```bash
coderaft verify myproject
```

**Sample drift output:**
```
ERROR  verification failed — 3 drift(s) detected:
  apt packages drifted: +1 added, -0 removed, ~1 changed
    + vim=2:8.2.3995-1ubuntu2
    ~ git: 1:2.34.1-1ubuntu1.10 → 1:2.34.1-1ubuntu1.11
  pip packages drifted: +0 added, -1 removed, ~0 changed
    - flask==3.0.0
```

---

### `coderaft apply`

Apply the `coderaft.lock.json` to the running Island: configure registries and apt sources, then reconcile package sets to match the lock.

**Syntax:**
```bash
coderaft apply <project> [--dry-run]
```

**Options:**
- `--dry-run`: Preview the registry/source commands and package reconciliation steps without modifying the island.

**Behavior:**
- Registries:
  - Writes `/etc/pip.conf` with `index-url`/`extra-index-url` from lock
  - Runs `npm/yarn/pnpm` config to set global registry URLs
- Apt sources:
  - Backs up and rewrites `/etc/apt/sources.list`, clears `/etc/apt/sources.list.d/*.list`
  - Optionally sets a default release hint, then `apt update`
- Reconciliation:
  - APT: install exact versions from lock, remove extras, autoremove
  - Pip: install missing exact versions, uninstall extras
  - npm/yarn/pnpm (global): add missing exact versions, remove extras

Exits non-zero if application fails at any step.

**Examples:**
```bash
# Apply the lock file
coderaft apply myproject

# Preview what would change
coderaft apply myproject --dry-run
```

### `coderaft diff`

Compare the `coderaft.lock.json` with the live state of the running Island and display a colorized, human-readable diff.

**Syntax:**
```bash
coderaft diff <project>
```

**Behavior:**
- Reads `coderaft.lock.json` from the project workspace (errors if missing; suggests `coderaft lock` first)
- Starts the island if not already running
- Gathers live container state in parallel: packages (apt, pip, npm, yarn, pnpm), registries, apt sources, base image digest, and container config
- Compares each section against the lock file and prints grouped differences with `+` (added), `-` (removed), `~` (changed) markers
- If the island matches the lock file, prints "no differences"
- This is a **read-only** operation — nothing is modified

**Example:**
```bash
coderaft diff myproject
```

**Sample output:**
```
Base Image:
  ~ digest: sha256:abc123... → sha256:def456...

Packages (apt):
  + vim=2:8.2.3995-1ubuntu2
  ~ git: 1:2.34.1-1ubuntu1.10 → 1:2.34.1-1ubuntu1.11

Packages (pip):
  - flask==3.0.0
```

**Notes:**
- Use `coderaft verify` for a pass/fail check (suitable for CI)
- Use `coderaft apply` to reconcile the island to match the lock
- Use `coderaft diff` for a detailed visual comparison

---

## Configuration Commands

---

### `coderaft backup`

Backup a project's Island state and configuration into a portable directory.

**Syntax:**
```bash
coderaft backup <project> [--output <dir>]
```

**Options:**
- `--output, -o <dir>`: Output directory for backup (default: `<workspace>/.coderaft_backups/<timestamp>`)

**Behavior:**
- Commits the running Island to a Docker image
- Saves the image as `image.tar`
- Writes a `metadata.json` manifest with project name, island name, image tag, coderaft config, and lock file contents

**Example:**
```bash
# Backup with automatic timestamps
coderaft backup myproject

# Specify a custom backup directory
coderaft backup myproject --output /tmp/myproject-backup
```

---

### `coderaft restore`

Restore a project's Island from a backup directory created by `coderaft backup`.

**Syntax:**
```bash
coderaft restore <project> <backup-dir> [--force]
```

**Options:**
- `--force, -f`: Overwrite the existing Island if it already exists

**Behavior:**
- Loads `image.tar` from the backup directory
- Reads `metadata.json` for the image tag
- Creates a new Island from the restored image
- Starts the Island and sets up the coderaft wrapper

**Example:**
```bash
# Restore from a backup
coderaft restore myproject ~/backups/myproject/20250101-120000

# Force overwrite an existing island
coderaft restore myproject ~/backups/myproject/20250101-120000 --force
```

---

### `coderaft export`

Create a portable `.tar.gz` archive containing the Island image snapshot, configuration, and lock file. The archive can be transferred to another machine and restored with `coderaft restore`.

**Syntax:**
```bash
coderaft export <project> [--output <path>]
```

**Options:**
- `--output, -o <path>`: Output file path (default: `<workspace>/<project>-export-<timestamp>.tar.gz`)

**Behavior:**
- Commits the running container to a temporary Docker image
- Saves the image as `image.tar`
- Bundles into a `.tar.gz` archive containing:
  - `image.tar` — Docker image snapshot
  - `coderaft.json` — project configuration (if present)
  - `coderaft.lock.json` — environment lock file (if present)
  - `manifest.json` — metadata (version, project name, image tag, island name, export timestamp)
- Removes the temporary export image after archiving

**Examples:**
```bash
# Export with default filename
coderaft export myproject

# Export to a specific path
coderaft export myproject -o ./myproject-portable.tar.gz
```

**Notes:**
- The island must exist before exporting (run `coderaft up` or `coderaft init` first)
- Use `coderaft restore` to import the archive on another machine

---

### `coderaft devcontainer generate`

Generate a VS Code `.devcontainer/devcontainer.json` from the current project's `coderaft.json`.

**Syntax:**
```bash
coderaft devcontainer generate
```

**Behavior:**
- Reads `coderaft.json` from the current directory
- Maps base image, ports, environment variables, volumes, and setup commands into a `devcontainer.json`
- Writes `.devcontainer/devcontainer.json`

**Example:**
```bash
cd ~/coderaft/myproject
coderaft devcontainer generate

# Then open in VS Code → "Reopen in Container"
```

---

### `coderaft templates`

Manage coderaft project templates (built-in and user-defined).

**Subcommands:**

#### `coderaft templates list`
List available templates (built-in + user templates in `~/.coderaft/templates`).

**Syntax:**
```bash
coderaft templates list
```

#### `coderaft templates show`
Show a template’s JSON (name, description, and config).

**Syntax:**
```bash
coderaft templates show <name>
```

#### `coderaft templates create`
Create `coderaft.json` in the current directory from a template.

**Syntax:**
```bash
coderaft templates create <name> [project]
```

**Examples:**
```bash
cd ~/coderaft/myapp
coderaft templates create python MyApp

# If project name omitted, folder name is used
coderaft templates create nodejs
```

#### `coderaft templates save`
Save the current folder’s `coderaft.json` as a reusable user template in `~/.coderaft/templates/<name>.json`.

**Syntax:**
```bash
coderaft templates save <name>
```

#### `coderaft templates delete`
Delete a user template by name.

**Syntax:**
```bash
coderaft templates delete <name>
```

---

### `coderaft config`

Manage coderaft configurations.

**Subcommands:**

#### `coderaft config generate`
Generate coderaft.json configuration file for a project.

**Syntax:**
```bash
coderaft config generate <project> [flags]
```

**Options:**
- `--template, -t <template>`: Use template configuration

**Examples:**
```bash
# Generate basic config
coderaft config generate myproject

# Generate with template
coderaft config generate myproject --template python
```

#### `coderaft config validate`
Validate project configuration file.

**Syntax:**
```bash
coderaft config validate <project>
```

#### `coderaft config show`
Display project configuration details.

**Syntax:**
```bash
coderaft config show <project>
```

Note: Template listing and management has moved to the top-level `coderaft templates` command.

#### `coderaft config schema`
Print the JSON Schema for coderaft.json, useful for editor validation and autocompletion.

**Syntax:**
```bash
coderaft config schema
```

#### `coderaft config global`
Show global coderaft configuration.

**Syntax:**
```bash
coderaft config global
```

## Maintenance Commands

---

### `coderaft version`

Display the version information for coderaft.

**Syntax:**
```bash
coderaft version
```

**Examples:**
```bash
# Display version information
coderaft version
```

**Output Format:**
```
coderaft (v1.0)
```

---

### `coderaft cleanup`

Clean up Docker resources and coderaft artifacts.

**Syntax:**
```bash
coderaft cleanup [flags]
```

**Options:**
- `--orphaned`: Remove orphaned containers only
- `--images`: Remove unused images only
- `--volumes`: Remove unused volumes only
- `--networks`: Remove unused networks only
- `--system-prune`: Run docker system prune
- `--all`: Clean up everything
- `--dry-run`: Show what would be cleaned (no changes)
- `--force`: Skip confirmation prompts

**Examples:**
```bash
# Interactive cleanup menu
coderaft cleanup

# Clean specific resources
coderaft cleanup --orphaned
coderaft cleanup --images

# Comprehensive cleanup
coderaft cleanup --all

# Preview cleanup actions
coderaft cleanup --dry-run --all

# Cleanup without prompts
coderaft cleanup --all --force
```

---

### `coderaft maintenance`

Perform maintenance tasks on coderaft projects and Islands.

**Syntax:**
```bash
coderaft maintenance [flags]
```

**Options:**
- `--status`: Show detailed system status
- `--health-check`: Check health of all projects
- `--update`: Update all Islands
- `--restart`: Restart stopped Islands
- `--rebuild`: Rebuild all Islands
- `--auto-repair`: Auto-fix common issues
- `--force`: Skip confirmation prompts

**Examples:**
```bash
# Interactive maintenance menu
coderaft maintenance

# Individual tasks
coderaft maintenance --health-check
coderaft maintenance --update
coderaft maintenance --restart

# Combined operations
coderaft maintenance --health-check --update --restart

# Auto-repair issues
coderaft maintenance --auto-repair

# Force operations without prompts
coderaft maintenance --force --rebuild
```

---

### `coderaft update`

Pull the latest base image(s) and rebuild environment Island(es).

This command replaces Islands to ensure they are based on the newest upstream images, while preserving your workspace files on the host.

**Syntax:**
```bash
coderaft update [project]
```

**Behavior:**
- When a project is specified, only that environment is updated
- With no project, all registered projects are updated
- Pulls the latest base image, recreates the Island with current coderaft.json config, and re-runs setup commands
- Replays package install commands from `coderaft.history` to restore your previously installed packages

**Options:**
- None currently. Uses your existing configuration in `coderaft.json` if present.

**Examples:**
```bash
# Update a single project
coderaft update myproject

# Update all projects
coderaft update
```

**Notes:**
- Your files remain in ~/coderaft/<project>/ and are re-mounted into the new Island
- If the project has a coderaft.json, its settings (ports, env, volumes, etc.) are applied on rebuild
- System packages inside the Island are updated as part of the rebuild
- If the Island exists, it will be stopped and replaced; if missing, it will be created

### `coderaft hooks`

Manage git pre-commit hook integration that runs `coderaft verify` before each commit to detect environment drift.

**Subcommands:**

#### `coderaft hooks install`

Install a git pre-commit hook that runs `coderaft verify` before each commit.

**Syntax:**
```bash
coderaft hooks install <project>
```

**Behavior:**
- Locates the `.git` directory in the project workspace (errors if not a git repo)
- If a pre-commit hook already exists, appends the coderaft hook block (compatible with husky, lefthook, etc.)
- If no pre-commit hook exists, creates a new one
- The hook checks for `coderaft.lock.json` and runs `coderaft verify`
- If verification fails, the commit is **blocked** with instructions to run `coderaft lock` or `coderaft apply`
- Users can bypass with `git commit --no-verify`

**Example:**
```bash
coderaft hooks install myproject
```

#### `coderaft hooks remove`

Remove the coderaft pre-commit hook.

**Syntax:**
```bash
coderaft hooks remove <project>
```

**Behavior:**
- Strips the coderaft hook block from the pre-commit file
- If nothing meaningful remains after removal, deletes the hook file entirely
- Otherwise, preserves other hook content

**Example:**
```bash
coderaft hooks remove myproject
```

---

## Exit Codes

---

Coderaft uses standard exit codes:

- `0`: Success
- `1`: General error
- `2`: Invalid arguments or usage
- `125`: Docker daemon not running
- `126`: Container not executable
- `127`: Container/command not found

## Environment Variables

---

Coderaft respects these environment variables:

##### Host-side (affects the coderaft CLI)

| Variable | Default | Description |
|----------|---------|-------------|
| `DOCKER_HOST` | system default | Docker daemon socket |
| `CODERAFT_HOME` | `~/.coderaft` | Override default coderaft configuration directory |
| `CODERAFT_WORKSPACE` | `~/coderaft` | Override default project workspace directory |
| `CODERAFT_ENGINE` | `docker` | Container engine binary. Set to `podman` or another docker-compatible CLI to use an alternative engine |
| `CODERAFT_STOP_TIMEOUT` | `2` (seconds) | Timeout for `docker stop` when stopping an island. Set to `0` for immediate kill |
| `CODERAFT_DISABLE_PARALLEL` | `false` | Set to `true` to disable parallel operations (falls back to sequential execution) |
| `CODERAFT_MAX_WORKERS` | `4` | Maximum number of general parallel workers |
| `CODERAFT_SETUP_WORKERS` | `3` | Number of parallel workers for setup commands |
| `CODERAFT_QUERY_WORKERS` | `5` | Number of parallel workers for package query operations (used by `lock`, `diff`, `verify`) |

##### Island-side (inside the container)

| Variable | Default | Description |
|----------|---------|-------------|
| `CODERAFT_ISLAND_NAME` | *(set automatically)* | Name of the current island |
| `CODERAFT_PROJECT_NAME` | *(set automatically)* | Name of the current project |
| `CODERAFT_HISTORY` | `/island/coderaft.history` | Path where package install/remove commands are recorded. Set to empty to disable recording |
| `CODERAFT_LOCKFILE` | *(deprecated)* | Legacy name for `CODERAFT_HISTORY`. Honored if set and `CODERAFT_HISTORY` has not been explicitly overridden |

## Project Structure

---

When you create a project, coderaft sets up:

```
~/coderaft/<project>/          # Project workspace (host)
├── coderaft.json             # Configuration file (optional)
├── your-files...           # Your project files
└── ...

~/.coderaft/                  # Global configuration
├── config.json            # Global settings and project registry
└── ...
```

**Inside Island:**
```
/island/                 # Mounted from ~/coderaft/<project>/
├── coderaft.json            # Same files as host
├── your-files...
└── ...
```

## Shell Completion

---

### `coderaft completion`

Generate completion scripts for your shell to enable tab autocompletion for coderaft commands, flags, project names, and template names.

**Syntax:**
```bash
coderaft completion [bash|zsh|fish]
```

**Supported Shells:**
- **Bash**: Autocompletion for commands, flags, project names, and templates (Linux)
- **Zsh**: Full autocompletion with descriptions (Linux)
- **Fish**: Intelligent completion with suggestions (Linux)

**Setup Instructions:**

**Bash:**
```bash
# Load completion for current session
source <(coderaft completion bash)

# Install for all sessions (Linux)
sudo coderaft completion bash > /etc/bash_completion.d/coderaft
```

**Zsh:**
```bash
# Enable completion if not already enabled
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Install for all sessions
coderaft completion zsh > "${fpath[1]}/_coderaft"

# Restart your shell or source ~/.zshrc
```

**Fish:**
```bash
# Load completion for current session
coderaft completion fish | source

# Install for all sessions
coderaft completion fish > ~/.config/fish/completions/coderaft.fish
```



**What Gets Completed:**
- Command names (`init`, `shell`, `run`, `list`, etc.)
- Command flags (`--template`, `--force`, `--keep-running`)
- Project names for commands like `shell`, `run`, `stop`, `destroy`
- Template names for `--template` flag and `templates show/delete`

**Examples:**
```bash
# Tab completion examples (press TAB after typing)
coderaft <TAB>                    # Shows: init, shell, run, list, etc.
coderaft shell <TAB>              # Shows: your-project-names
coderaft init myapp --template <TAB>  # Shows: python, nodejs, go, web
coderaft templates show <TAB>     # Shows: available-template-names
```

## Docker Integration

---

Coderaft creates Islands (Docker containers) with these characteristics:

- **Name**: `coderaft_<project>`
- **Base Image**: `buildpack-deps:bookworm` (configurable)
- **Working Directory**: `/island`
- **Mount**: `~/coderaft/<project>` → `/island`
- **Restart Policy**: `unless-stopped` (or `no` when `auto_stop_on_exit` is enabled and no explicit policy is set)
- **Command**: `sleep infinity` (keeps Island alive)

**Docker Commands Equivalent:**
```bash
# coderaft init myproject
docker create --name coderaft_myproject \
  --restart unless-stopped \
  -v ~/coderaft/myproject:/island \
  -w /island \
  buildpack-deps:bookworm sleep infinity

# coderaft shell myproject
docker start coderaft_myproject
docker exec -it coderaft_myproject bash

# coderaft run myproject <command>
docker exec coderaft_myproject <command>

# coderaft destroy myproject
docker stop coderaft_myproject
docker rm coderaft_myproject
```
