---
title: Quick Start
description: Get up and running with coderaft in minutes
---

Get started in minutes. Make sure you have [coderaft installed](/docs/install/) first.

## Clone an Existing Project

The fastest way to start working on any project:

```bash
coderaft clone https://github.com/user/repo
```

That's it! coderaft will:
1. Clone the repository
2. Auto-detect the tech stack (Python, Node.js, Go, etc.)
3. Create an isolated island with the right tools
4. Install dependencies

Ready to code in ~30 seconds.

```bash
# Enter the environment
coderaft shell repo

# Your code is at /island
cd /island
```

:::tip
Use `--depth 1` for faster cloning of large repositories.
:::

## Create a New Project

```bash
coderaft init my-app --template python
```

This creates a Python environment at `~/coderaft/my-app/`.

## Enter the Environment

```bash
coderaft shell my-app
```

You're now inside an isolated island. The prompt changes to show you're in the coderaft environment.

:::tip
By default, islands stop when you exit. Use `--keep-running` to keep them running.
:::

## Work Inside the Island

```bash
# Check available tools
python3 --version
pip3 --version

# Your files are at /island
cd /island

# Install packages (recorded for reproducibility)
pip3 install flask requests

# View recorded history
coderaft history
```

## Run Code

```bash
# Create a simple app
cat > /island/app.py << 'EOF'
from flask import Flask
app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello from coderaft!'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
EOF

# Run it
python3 app.py
```

## Manage Projects

```bash
# List projects
coderaft list

# Create more
coderaft init node-app --template nodejs
coderaft init go-service --template go

# Each is fully isolated
coderaft shell node-app
```

## Clean Up

```bash
# Stop and remove island (keeps files)
coderaft destroy my-app

# Or just stop it
coderaft stop my-app

# Files remain at ~/coderaft/my-app/
```

## Docker Access

Islands have Docker access by default via the mounted Docker socket. Build images, run containers, and use Docker Compose inside your island.

## Next Steps

- [CLI Reference](/docs/cli/)
- [Configuration](/docs/configuration/)
- [Templates](/docs/templates/)
- [Maintenance](/docs/maintenance/)
