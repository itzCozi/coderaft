---
title: Frequently Asked Questions
description: Answers to common questions about coderaft
---

Below are answers to the most common questions. If you're stuck, also check out the Troubleshooting page or ask for help.

## General
---

##### What is coderaft?
Coderaft creates isolated development environments using Docker (called "Islands"). Your code stays on the host while tools and dependencies live inside a Island.

##### What platforms are supported?
Supported on Linux (Debian, Ubuntu, Fedora, Arch, Alpine, and more), macOS, and Windows. See the [Installation Guide](/docs/install/) for platform-specific instructions.

##### Where are my project files stored?
On the host in a simple folder (e.g., `~/coderaft/<project>`). Inside the Island, they're mounted at `/island` by default.

##### Does coderaft require root?
Docker usually requires membership in the `docker` group. Coderaft runs its Docker commands on the host; inside the Island it often runs as root for setup convenience.

##### Do I need Docker Desktop or just Docker Engine?
You need a working Docker daemon accessible from Linux. On native Linux, install Docker Engine. On Windows, run inside WSL2 (Ubuntu) and either enable Docker Desktop's WSL integration or install Docker in the WSL distro. Coderaft checks `docker version` before running.

##### Where does coderaft store its own configuration?
Global config lives at `~/.coderaft/config.json`. It tracks projects and settings like default base image and auto‑stop. A project‑local config file is `coderaft.json` in your workspace (optional but recommended).

##### What project name characters are allowed?
Alphanumeric, hyphen, and underscore only: `^[a-zA-Z0-9_-]+$`. Names become part of the Island name (e.g., `coderaft_my-project`).

## Usage
---

##### How do I create a project?
Run `coderaft init <project>`. Optionally use `--template` (python, nodejs, go, web) or `--generate-config` to produce a `coderaft.json`.

##### How do I enter the environment?
Use `coderaft shell <project>` for an interactive shell, or `coderaft run <project> "<command>"` for a one‑off command.

##### Where is the configuration file?
In your project directory as `coderaft.json`. You can generate one with `coderaft init <project> --generate-config` or customize templates.

##### Can I keep my Island running?
By default, global setting `auto_stop_on_exit` is enabled which prefers not restarting containers automatically. You can specify `"restart": "unless-stopped"` in `coderaft.json` to keep it running, or toggle the global setting.

##### How do I expose ports?
Add mappings like `"ports": ["3000:3000", "8000:8000"]` in `coderaft.json`. Recreate or restart the Island to apply changes.

##### How do I mount extra folders?
Use `"volumes"` in `coderaft.json`, for example: `"volumes": ["/var/run/docker.sock:/var/run/docker.sock", "/path/on/host:/path/in/Island"]`.

##### What does `coderaft up` do?
From a folder that contains `coderaft.json`, `coderaft up` starts the environment defined by that file so new teammates can run it without `init`. Use `--keep-running` to avoid auto‑stop, and `--dotfiles <path>` to mount local dotfiles.

##### How do I prevent the Island from stopping after I exit?
Use `--keep-running` with `coderaft shell`, `coderaft run`, or `coderaft up`. Or set `"restart": "unless-stopped"` in `coderaft.json`.

##### How do I share my setup with teammates?
Commit `coderaft.json` to your repo. Teammates clone the repo and run `coderaft up` (or `coderaft init <name> --generate-config` if they want a managed entry) to reproduce the environment.

## Templates & Packages
---

##### What templates are available?
Built-in templates: `python`, `nodejs`, `go`, and `web`. You can also create custom templates in `~/.coderaft/templates/` as JSON files.

##### Can I install Docker tools inside the Island?
Yes. By default, coderaft mounts the Docker socket and many templates install Docker CLI, enabling Docker‑in‑Docker workflows.

##### How do setup commands work?
`setup_commands` run inside the Island after creation. Use them to install packages and tools. For example:

```
{
  "setup_commands": [
    "apt install -y python3-pip",
    "pip3 install flask"
  ]
}
```

##### How do I create my own template?
Save a JSON file in `~/.coderaft/templates/<name>.json` with a `config` object that mirrors `coderaft.json` fields. List available templates with `coderaft templates list`, and use it via `coderaft init <project> --template <name>`.

##### Are package installs recorded anywhere?
Yes. Inside the Island, coderaft wraps common package managers (apt, pip/pip3, npm/yarn/pnpm/corepack) and appends successful install/remove commands to `/island/coderaft.history`. You can replay them during updates. To change or disable it, set the `CODERAFT_LOCKFILE` env var in `coderaft.json` (empty to disable, or set a custom path).

## Management
---

##### How do I list or remove projects?
Use `coderaft list` to list, and `coderaft destroy <project>` to remove the Island and clean up tracking. Your host files remain unless you delete them manually.

##### How do I update the base image or config?
Edit `coderaft.json` (e.g., `base_image`, ports, volumes). Then `coderaft destroy <project>` and `coderaft init <project>` to recreate with the new config.

##### Where is the global config stored?
In `~/.coderaft/config.json`. It tracks projects and global settings like default base image and auto‑update/auto‑stop behavior.

##### How do I update all environments to the latest base image?
Run `coderaft update` to update all, or `coderaft update <project>` for one. This pulls the latest base image, recreates the Island, and re-runs recorded/setup steps.

##### How do I rebuild everything from scratch?
`coderaft maintenance --rebuild` destroys and recreates all managed Islands using your current configs.

##### How do I clean up orphaned Islands?
If a Island exists but isn't tracked, run `coderaft destroy --cleanup-orphaned` to remove untracked `coderaft_*` containers.

##### How do I see more details in the list output?
Use `coderaft list --verbose` to include config presence, base image overrides, ports, and setup command counts.

##### Can I enable shell autocompletion?
Yes. Generate completion scripts with `coderaft completion bash|zsh|fish` and follow the printed instructions.

##### How do I change global defaults like base image or auto‑stop?
Edit `~/.coderaft/config.json` under `settings` (e.g., `default_base_image`, `auto_stop_on_exit`, `auto_update`).

## Advanced configuration
---

##### How do I mount my dotfiles into the Island?
Add a `dotfiles` entry in `coderaft.json` or pass `--dotfiles <path>` to `coderaft up`. Coderaft mounts the directory at `/dotfiles` and symlinks common files like `.gitconfig`, `.vimrc`, `.bashrc`, and `.config/*` into the root user's home.

##### How do I run as a non‑root user or change the shell/working directory?
Use these fields in `coderaft.json`:
- `"user"`: e.g., `"1000:1000"`
- `"shell"`: e.g., `"/bin/bash"`
- `"working_dir"`: default `/island`

##### Can I set CPU/memory limits?
Yes, via `resources`:

```
{
  "resources": { "cpus": "2", "memory": "4g" }
}
```

##### Do health checks exist?
Yes. Use `health_check` to configure Docker health checks:

```
{
  "health_check": {
    "test": ["CMD-SHELL", "curl -fsS http://localhost:8080/health || exit 1"],
    "interval": "30s",
    "timeout": "5s",
    "retries": 5
  }
}
```

##### How do I attach to a custom network or add capabilities/labels?
`coderaft.json` supports `network`, `capabilities` (for `--cap-add`), and `labels`:

```
{
  "network": "my-net",
  "capabilities": ["SYS_ADMIN"],
  "labels": { "com.example.owner": "team-a" }
}
```

## Getting Help
---

- GitHub: https://github.com/itzcozi/coderaft
- Telegram: http://t.me/coderaftcli
- Website & Docs: https://coderaft.ar0.eu
