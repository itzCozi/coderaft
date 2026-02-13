---
title: Templates & Setup
description: Built-in templates and custom setup configurations for coderaft
---

Coderaft provides a few built-in templates for common development environments. Use them as a starting point and customize via `coderaft.json`.

## Built-in Templates
---

#### Python
```json
{
  "name": "python-project",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y python3 python3-pip python3-venv python3-dev",
    "apt install -y build-essential git curl wget",
    "pip3 install --upgrade pip setuptools wheel",
    "pip3 install flask django fastapi requests pytest black flake8"
  ],
  "environment": {
    "PYTHONPATH": "/island",
    "PYTHONUNBUFFERED": "1",
    "PYTHONDONTWRITEBYTECODE": "1"
  },
  "ports": ["5000:5000", "8000:8000"],
  "working_dir": "/island"
}
```

Usage:
```bash
coderaft init myapp --template python
coderaft shell myapp
```

---

#### Node.js
```json
{
  "name": "nodejs-project",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "curl -fsSL https://deb.nodesource.com/setup_18.x | bash -",
    "apt install -y nodejs build-essential git curl wget",
    "npm install -g npm@latest",
    "npm install -g typescript ts-node nodemon",
    "npm install -g @vue/cli create-react-app"
  ],
  "environment": {
    "NODE_ENV": "development",
    "NPM_CONFIG_PREFIX": "/island/.npm-global"
  },
  "ports": ["3000:3000", "8080:8080"],
  "working_dir": "/island"
}
```

Usage:
```bash
coderaft init webapp --template nodejs
coderaft shell webapp
```

---

#### Go
```json
{
  "name": "go-project",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y wget git build-essential",
    "wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz",
    "tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz",
    "rm go1.21.0.linux-amd64.tar.gz"
  ],
  "environment": {
    "GOROOT": "/usr/local/go",
    "GOPATH": "/island/go",
    "PATH": "/usr/local/go/bin:/island/go/bin:$PATH"
  },
  "ports": ["8080:8080"],
  "working_dir": "/island"
}
```

Usage:
```bash
coderaft init service --template go
coderaft shell service
```

---

#### Web
```json
{
  "name": "web-project",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "curl -fsSL https://deb.nodesource.com/setup_18.x | bash -",
    "apt install -y python3 python3-pip nodejs nginx git curl wget",
    "apt install -y build-essential postgresql-client redis-tools",
    "pip3 install flask django fastapi gunicorn",
    "npm install -g typescript vue-cli create-react-app pm2"
  ],
  "environment": {
    "PYTHONPATH": "/island",
    "NODE_ENV": "development",
    "FLASK_ENV": "development"
  },
  "ports": [
    "80:80",
    "3000:3000",
    "5000:5000",
    "8000:8000",
    "8080:8080"
  ],
  "working_dir": "/island"
}
```

Usage:
```bash
coderaft init fullstack --template web
coderaft shell fullstack
```

## Template Usage
---

##### Creating from Templates

```bash
# List available templates (built-in + user)
coderaft templates list

# Create project from template
coderaft init myproject --template <template-name>

# Available templates: python, nodejs, go, web
coderaft init api --template nodejs
coderaft init ml-project --template python
coderaft init microservice --template go
coderaft init webapp --template web
```

##### Template Customization
Generate config from a template, edit, then create:
```bash
coderaft init myproject --config-only --template python
coderaft init myproject
```

## Custom Configurations
Create your own reusable configurations (example snippet):

```json
{
  "name": "data-science",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y python3 python3-pip jupyter",
    "pip3 install pandas numpy matplotlib seaborn scikit-learn",
    "pip3 install jupyter notebook jupyterlab",
    "pip3 install plotly dash streamlit"
  ],
  "environment": {
    "PYTHONPATH": "/island",
    "JUPYTER_ENABLE_LAB": "yes"
  },
  "ports": ["8888:8888", "8501:8501"],
  "working_dir": "/island"
}
```

Save and reuse custom templates locally:

```bash
# Save current folder's coderaft.json as a template named "data-science"
coderaft templates save data-science

# List templates (now includes data-science)
coderaft templates list

# Create a new coderaft.json from your template in a different project
mkdir ~/coderaft/analysis && cd ~/coderaft/analysis
coderaft templates create data-science AnalysisProject
```

##### Database Development (example)

```json
{
  "name": "database-dev",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y postgresql-client mysql-client redis-tools",
    "apt install -y python3 python3-pip",
    "pip3 install psycopg2-binary pymongo redis sqlalchemy"
  ],
  "environment": {
    "PGHOST": "localhost",
    "REDIS_URL": "redis://localhost:6379"
  },
  "ports": ["5432:5432", "6379:6379", "27017:27017"],
  "working_dir": "/island"
}
```

##### DevOps/Infrastructure (example)

```json
{
  "name": "devops",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "apt install -y curl wget git jq unzip",
    "curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl",
    "chmod +x kubectl && mv kubectl /usr/local/bin/",
    "curl -fsSL https://get.docker.com | sh",
    "pip3 install awscli ansible terraform"
  ],
  "environment": {
    "KUBECONFIG": "/island/.kube/config"
  },
  "volumes": ["/var/run/docker.sock:/var/run/docker.sock"],
  "working_dir": "/island"
}
```

##### Mobile Development (example)

```json
{
  "name": "mobile-dev",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "curl -fsSL https://deb.nodesource.com/setup_18.x | bash -",
    "apt install -y nodejs default-jdk android-tools-adb",
    "npm install -g @ionic/cli cordova react-native-cli",
    "npm install -g expo-cli @react-native-community/cli"
  ],
  "environment": {
    "ANDROID_HOME": "/island/android-sdk",
    "JAVA_HOME": "/usr/lib/jvm/default-java"
  },
  "ports": ["19000:19000", "19001:19001", "19002:19002"],
  "working_dir": "/island"
}
```

## Template Development
1. Start with working configuration:
```bash
coderaft init test-project --generate-config
# Edit ~/coderaft/test-project/coderaft.json
coderaft destroy test-project && coderaft init test-project
```

2. Test thoroughly:
```bash
coderaft shell test-project
# Test all tools and commands
# Verify environment variables
# Check port access
```

3. Document your template:
```json
{
  "name": "my-template",
  "description": "Custom template for X development",
  "base_image": "ubuntu:latest",
  "setup_commands": [
    "# Add descriptive comments",
    "apt install -y tool1 tool2"
  ]
}
```

##### Sharing Templates

Save template configurations in your project repository:

```
project/
├── coderaft.json          # Main configuration
├── templates/           # Additional templates
│   ├── development.json
│   ├── testing.json
│   └── production.json
└── scripts/
    └── setup.sh         # Additional setup scripts
```

User templates are stored under:

```
~/.coderaft/templates/
```

Each template is a JSON file named `<template>.json` with this shape:

```json
{
  "name": "my-template",
  "description": "Custom template for X development",
  "config": {
    "name": "project-name",
    "base_image": "ubuntu:latest",
    "setup_commands": ["apt install -y ..."],
    "environment": {},
    "ports": [],
    "volumes": [],
    "working_dir": "/island"
  }
}
```

You can remove a user template with:

```bash
coderaft templates delete my-template
```

## Best Practices
---

##### Setup Command Guidelines

1. **Always use -y flag** for apt commands
2. **Group related commands** for better readability
3. **Use full paths** for downloaded files
4. **Clean up temporary files** after installation
5. **Test commands individually** before adding to config

##### Environment Variables

1. **Use meaningful names** that describe purpose
2. **Set PATH modifications** in environment section
3. **Include language-specific variables** (PYTHONPATH, GOPATH, etc.)
4. **Use /island** as base for project-specific paths

##### Port Configuration

1. **Map common development ports** (3000, 5000, 8000, 8080)
2. **Include language-specific ports** (Python: 5000, 8000; Node.js: 3000, 8080)
3. **Reserve ports for databases** if needed (5432, 3306, 6379, 27017)
4. **Use consistent port mapping** across similar projects

##### Volume Management

1. **Mount data directories** outside /island for persistence
2. **Use absolute paths** for volume mappings
3. **Consider cache directories** for package managers
4. **Mount configuration files** if shared across projects
