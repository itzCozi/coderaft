---
title: Quick Start Guide
description: Get up and running with coderaft in minutes
---

This guide will get you up and running with coderaft in just a few minutes. You'll create your first isolated development environment and learn the basic workflow.

## Prerequisites
---

Before starting, make sure you have coderaft installed. If you haven't installed it yet, follow the [Installation Guide](/docs//install/) first.

## Create Your First Project
---

Let's create a Python development environment:

```bash
coderaft init my-python-app --template python
```

This command:
- Creates a new project called `my-python-app`
- Uses the Python template (includes Python 3, pip, and common tools)
- Sets up a Docker island with Ubuntu 22.04
- Creates a workspace directory at `~/coderaft/my-python-app/`

## Enter Your Development Environment
---

```bash
coderaft shell my-python-app
```

You're now inside an isolated Ubuntu Island! Notice how your prompt changes to indicate you're in the coderaft environment.
By default, the Island will stop automatically when you exit the shell. To keep it running after you exit, pass `--keep-running`.

## Explore the Environment
---

Inside the Island, you can:

```bash
# Check what's available
python3 --version
pip3 --version
which python3

# Your workspace is mounted at /workspace
cd /workspace
ls -la

# Install additional packages
apt update
apt install tree htop

# Install Python packages
pip3 install requests flask

# These installs are automatically recorded to /workspace/coderaft.lock
# so the environment can be reproduced on rebuild or by teammates.
```

## Create and Run Code
---

Create a simple Python application:

```bash
# Create a simple web app
cat > /workspace/app.py << 'EOF'
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello from coderaft! 🚀'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, debug=True)
EOF

# Run your app
python3 app.py
```

## Manage Your Projects
---

```bash
# List all your projects
coderaft list

# Create more projects
coderaft init node-app --template nodejs
coderaft init go-service --template go

# Each project is completely isolated
coderaft shell node-app    # Node.js environment
coderaft shell go-service  # Go environment
```

## Clean Up
---

When you're done with a project:

```bash
# Stop and remove the Island (keeps your files)
coderaft destroy my-python-app

# Or just stop the Island without removing it
coderaft stop my-python-app

# Your files are still in ~/coderaft/my-python-app/
ls ~/coderaft/my-python-app/

# To recreate the environment later:
coderaft init my-python-app --template python
```

## Docker Access
---

By default, all coderaft environments have access to the host's Docker daemon, allowing you to:

- Build and manage Docker islandes/containers from within your coderaft environment
- Run Docker commands without additional configuration
- Execute Docker Compose for multi-Island (multi-container) applications

This works by mounting the host's Docker socket (`/var/run/docker.sock`) in your coderaft island and installing the Docker CLI tools automatically.

## Next Steps
---

Now that you understand the basics:

1. **Explore the commands**: [Command Reference](/docs/cli/)
2. **Learn about configuration**: [Configuration Guide](/docs/configuration/)
3. **Explore templates**: Try different [project templates](/docs/templates/)
4. **Customize**: Create a custom `coderaft.json` config file
5. **Maintenance**: [Cleanup and Maintenance](/docs/maintenance/)
