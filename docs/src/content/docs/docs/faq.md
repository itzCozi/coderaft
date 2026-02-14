---
title: FAQ
description: Frequently asked questions about coderaft
---

Quick answers to common questions. See also [Troubleshooting](/docs/troubleshooting/) if you're stuck.

## General

##### What is coderaft?
A tool that creates isolated development environments using Docker containers (called "islands"). Code stays on host, tools run in the island.

##### What platforms are supported?
Linux (Debian, Ubuntu, Fedora, Arch, Alpine, and more), macOS, and Windows. See [Installation](/docs/install/).

##### Where are project files stored?
On host: `~/coderaft/<project>/`. Inside island: `/island/`.

##### Does coderaft require root?
No, but Docker requires membership in the `docker` group. Inside islands, commands run as root by default for setup convenience.

##### Where is configuration stored?
- Global: `~/.coderaft/config.json`
- Project: `coderaft.json` in your workspace
- Secrets: `~/.coderaft/secrets.vault.json` (encrypted)

## Usage

##### How do I create a project?
```bash
coderaft init <project> [--template python|nodejs|go|web]
```

##### How do I enter the environment?
```bash
coderaft shell <project>
```

##### How do I run a command?
```bash
coderaft run <project> "<command>"
```

##### How do I expose ports?
Add to `coderaft.json`:
```json
{ "ports": ["3000:3000", "8000:8000"] }
```

##### How do I see what ports are exposed?
```bash
coderaft ports [project]
```
Shows all exposed ports with clickable URLs and auto-detected service names.

##### How do I store secrets/API keys?
```bash
coderaft secrets init           # One-time setup
coderaft secrets set <project> API_KEY
coderaft secrets import <project> .env
```
Secrets are stored encrypted (AES-256) and can be exported for use.

##### How do I share with teammates?
Commit `coderaft.json`. Teammates run `coderaft up`.

##### How do I stop auto-stop behavior?
Use `--keep-running` or set `"restart": "unless-stopped"` in config.

## Management

##### How do I list projects?
```bash
coderaft list
```

##### How do I remove a project?
```bash
coderaft destroy <project>
```
Files remain on host unless manually deleted.

##### How do I update environments?
```bash
coderaft update [project]
```

##### Are package installs recorded?
Yes. Installs from 30+ package managers (apt, pip, npm, cargo, go, gem, brew, conda, etc.) are logged to `/island/coderaft.history`. Downloads via wget/curl and `make install` are also tracked. These commands are replayed on rebuild.

## Advanced

##### Can I set resource limits?
```json
{ "resources": { "cpus": "2", "memory": "4g" } }
```

##### Can I use Podman instead of Docker?
Set `CODERAFT_ENGINE=podman`.

##### How do I mount dotfiles?
```bash
coderaft up --dotfiles ~/.dotfiles
```
Or add `"dotfiles": ["~/.dotfiles"]` to config.

## Help

- Docs: [coderaft.ar0.eu](https://coderaft.ar0.eu)
- GitHub: [itzcozi/coderaft](https://github.com/itzcozi/coderaft)
