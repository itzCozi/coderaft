---
title: Templates
description: Built-in and custom project templates
---

## Built-in Templates

### Python

```bash
coderaft init myapp --template python
```

Includes: Python 3, pip, venv, flask, django, fastapi, pytest, black, flake8

Ports: 5000, 8000

### Node.js

```bash
coderaft init myapp --template nodejs
```

Includes: Node.js 18, npm, typescript, ts-node, nodemon, vue-cli, create-react-app

Ports: 3000, 8080

### Go

```bash
coderaft init myapp --template go
```

Includes: Go 1.21, git, build tools

Ports: 8080

### Web (Full-stack)

```bash
coderaft init myapp --template web
```

Includes: Python + Node.js + nginx + postgresql-client + redis-tools

Ports: 80, 3000, 5000, 8000, 8080

## Custom Templates

Create custom templates in `~/.coderaft/templates/`:

```json
// ~/.coderaft/templates/rust.json
{
  "name": "rust-template",
  "description": "Rust development environment",
  "config": {
    "base_image": "buildpack-deps:bookworm",
    "setup_commands": [
      "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y",
      "source $HOME/.cargo/env"
    ],
    "environment": {
      "PATH": "/root/.cargo/bin:$PATH"
    }
  }
}
```

### Template Commands

```bash
coderaft templates list           # List all templates
coderaft templates show <name>    # View template contents
coderaft init myapp -t <name>     # Use template
```
