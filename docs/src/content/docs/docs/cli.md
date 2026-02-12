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
- Creates/starts a Island named `coderaft_<name>` where `<name>` comes from `coderaft.json`'s `name` (or the folder name)
- Applies ports, env, and volumes from configuration
- Runs a system update, then `setup_commands`
- Installs the coderaft wrapper for nice shell UX
 - Records package installations you perform inside the Island to `coderaft.lock` (apt/pip/npm/yarn/pnpm). On rebuilds, these commands are replayed to reproduce the environment.
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
- Sets working directory to `/workspace`
- Your project files are available at `/workspace`
- Exit with `exit`, `logout`, or `Ctrl+D`
- By default, the Island stops automatically after you exit the shell when global setting `auto_stop_on_exit` is enabled (default)
- Use `--keep-running` to keep the Island running after you exit the shell

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
coderaft run myproject "cd /workspace && python3 -m http.server 8000"

# Execute script
coderaft run myproject bash /workspace/setup.sh
```

**Notes:**
- Commands run in `/workspace` by default
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

Generate a comprehensive environment snapshot as `coderaft.lock.json` for a project. This is ideal for sharing/auditing the exact Island image, container configuration, and globally installed packages.

**Syntax:**
```bash
coderaft lock <project> [-o, --output <path>]
```

**Options:**
- `-o, --output <path>`: Write the lock file to a custom path. Defaults to `<workspace>/coderaft.lock.json`.

**Behavior:**
- Ensures the project's Island is running (starts it if needed).
- Inspects the container and its image to capture:
  - Base image: name, digest (if available), image ID
  - Container config: working_dir, user, restart policy, network, ports, volumes, labels, environment, capabilities, resources (cpus/memory)
  - Installed package snapshots:
    - apt: manually installed packages pinned as `name=version`
    - pip: `pip freeze` output
    - npm/yarn/pnpm: globally installed packages as `name@version` (Yarn global versions are detected from Yarn's global dir)
  - Registries and sources for reproducibility:
    - pip: `index-url` and `extra-index-url`
    - npm/yarn/pnpm: global registry URLs
    - apt: `sources.list` lines, snapshot base URL if present, and OS release codename
- If `coderaft.json` exists in the workspace, includes its `setup_commands` for context.

This snapshot is meant for sharing and audit. It does not currently drive `coderaft up` automatically; continue to use `coderaft.json` plus the simple `coderaft.lock` command list for replay. A future `coderaft restore` may apply `coderaft.lock.json` directly.

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
  "version": 1,
  "project": "myproject",
  "island_name": "coderaft_myproject",
  "created_at": "2025-09-18T20:41:51Z",
  "base_image": {
    "name": "ubuntu:22.04",
    "digest": "ubuntu@sha256:...",
    "id": "sha256:..."
  },
  "container": {
    "working_dir": "/workspace",
    "user": "root",
    "restart": "no",
    "network": "bridge",
    "ports": ["3000/tcp -> 0.0.0.0:3000"],
    "volumes": ["bind /host/path -> /workspace (rw=true)"],
    "environment": {"TZ": "UTC"},
    "labels": {"coderaft.project": "myproject"},
    "capabilities": ["SYS_PTRACE"],
    "resources": {"cpus": "2", "memory": "2048MB"}
  },
  "packages": {
    "apt": ["git=1:2.34.1-..."],
    "pip": ["requests==2.32.3"],
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

Validate that the running Island matches the `coderaft.lock.json` exactly. Fails fast on any drift.

**Syntax:**
```bash
coderaft verify <project>
```

**Checks:**
- Package sets: apt, pip, npm, yarn, pnpm (exact set match)
- Registries: pip index/extra-index, npm/yarn/pnpm registry URLs
- Apt sources: sources.list lines, snapshot base URL (if present), OS release codename

Returns non-zero on any mismatch and prints a concise drift report.

**Example:**
```bash
coderaft verify myproject
```

---

### `coderaft apply`

Apply the `coderaft.lock.json` to the running Island: configure registries and apt sources, then reconcile package sets to match the lock.

**Syntax:**
```bash
coderaft apply <project>
```

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

**Example:**
```bash
coderaft apply myproject
```

## Configuration Commands

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
 - Replays package install commands from `coderaft.lock` to restore your previously installed packages

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

- `DOCKER_HOST`: Docker daemon socket
- `CODERAFT_HOME`: Override default `~/.coderaft` directory
- `CODERAFT_WORKSPACE`: Override default `~/coderaft` workspace directory

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
/workspace/                 # Mounted from ~/coderaft/<project>/
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
- **Base Image**: `ubuntu:22.04` (configurable)
- **Working Directory**: `/workspace`
- **Mount**: `~/coderaft/<project>` → `/workspace`
- **Restart Policy**: `unless-stopped` (or `no` when `auto_stop_on_exit` is enabled and no explicit policy is set)
- **Command**: `sleep infinity` (keeps Island alive)

**Docker Commands Equivalent:**
```bash
# coderaft init myproject
docker create --name coderaft_myproject \
  --restart unless-stopped \
  -v ~/coderaft/myproject:/workspace \
  -w /workspace \
  ubuntu:22.04 sleep infinity

# coderaft shell myproject
docker start coderaft_myproject
docker exec -it coderaft_myproject bash

# coderaft run myproject <command>
docker exec coderaft_myproject <command>

# coderaft destroy myproject
docker stop coderaft_myproject
docker rm coderaft_myproject
```
