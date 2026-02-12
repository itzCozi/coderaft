---
title: Troubleshooting
description: Common issues and solutions for coderaft
---

Common issues and quick fixes.

## Installation Issues
---

##### "Command not found: coderaft"

**Problem**: After installation, `coderaft` command is not recognized.

**Solutions**:
```bash
# Check if coderaft is in PATH
which coderaft

# Add to PATH if needed
export PATH="/usr/local/bin:$PATH"

# Make permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Verify installation
coderaft --help
```

##### "Docker is not installed or not running"

**Problem**: Coderaft can't connect to Docker daemon.

**Solutions**:
```bash
# Check Docker status
sudo systemctl status docker

# Start Docker if stopped
sudo systemctl start docker
sudo systemctl enable docker

# Check if user is in docker group
groups $USER

# Add user to docker group
sudo usermod -aG docker $USER
# Note: You must log out and back in for this to take effect

# Test Docker access
docker ps
```

##### "Permission denied while trying to connect to Docker"

**Problem**: User doesn't have permission to access Docker socket.

**Solutions**:
```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Restart terminal session or logout/login

# Alternatively, run with sudo (not recommended)
sudo coderaft init myproject
```

## Island Issues
---

##### "Island not found" or "No such Island"

**Problem**: Island was manually deleted or doesn't exist.

**Solutions**:
```bash
# Check what Islands exist
docker ps -a --filter "name=coderaft_"

# List coderaft projects
coderaft list

# Recreate missing Island
coderaft destroy myproject  # Clean up tracking
coderaft init myproject     # Recreate

# Or force recreate
coderaft init myproject --force
```

##### "Island won't start"

**Problem**: Island fails to start or immediately exits.

**Diagnosis**:
```bash
# Check Island status
docker ps -a --filter "name=coderaft_myproject"

# Check Island logs
docker logs coderaft_myproject

# Inspect Island configuration
docker inspect coderaft_myproject
```

**Solutions**:
```bash
# Try restarting Island
docker start coderaft_myproject

# If still fails, recreate Island
coderaft destroy myproject
coderaft init myproject

# Check Docker daemon
sudo systemctl restart docker
```

##### "Island stops immediately after starting"

**Problem**: Island keeps exiting instead of staying running.

**Solutions**:
```bash
# Check what command Island is running
docker inspect coderaft_myproject | grep -A 5 '"Cmd"'

# Island should run 'sleep infinity'
# If not, recreate:
coderaft destroy myproject
coderaft init myproject

# Check for resource constraints
docker stats --no-stream
```

## File Access Issues
---

##### "Files not showing up in Island"

**Problem**: Files created on host don't appear in `/workspace/` inside Island.

**Diagnosis**:
```bash
# Check mount point
docker inspect coderaft_myproject | grep -A 10 '"Mounts"'

# Should show: ~/coderaft/myproject -> /workspace
```

**Solutions**:
```bash
# Verify workspace directory exists
ls -la ~/coderaft/myproject/

# Create file on host and check in Island
echo "test" > ~/coderaft/myproject/test.txt
coderaft run myproject cat /workspace/test.txt

# If mount is wrong, recreate Island
coderaft destroy myproject
coderaft init myproject
```

##### "Permission denied accessing files"

**Problem**: Can't read/write files in Island workspace.

**Solutions**:
```bash
# Check file permissions
ls -la ~/coderaft/myproject/

# Fix ownership if needed
sudo chown -R $USER:$USER ~/coderaft/myproject/

# Check Island user
coderaft run myproject whoami
coderaft run myproject id

# If running as different user, use sudo inside Island
coderaft run myproject "sudo chown -R root:root /workspace/"
```

## Network and Port Issues
---

##### "Port already in use"

**Problem**: Can't bind to port specified in configuration.

**Solutions**:
```bash
# Check what's using the port
sudo netstat -tlnp | grep :5000
# or
sudo ss -tlnp | grep :5000

# Kill process using port
sudo kill -9 <PID>

# Or use different port in coderaft.json
# Change "5000:5000" to "5001:5000"

# Recreate Island with new config
coderaft destroy myproject
coderaft init myproject
```

##### "Can't access web application from host"

**Problem**: Web app running in Island but not accessible from host.

**Solutions**:
```bash
# Ensure app binds to 0.0.0.0, not localhost
# In your app: app.run(host='0.0.0.0', port=5000)

# Check port mapping in Island
docker port coderaft_myproject

# Verify ports in coderaft.json
cat ~/coderaft/myproject/coderaft.json

# Test from inside Island
coderaft run myproject "curl http://localhost:5000"

# Test from host
curl http://localhost:5000
```

## Configuration Issues
---

##### "Invalid JSON in coderaft.json"

**Problem**: Configuration file has syntax errors.

**Solutions**:
```bash
# Validate JSON syntax
cat ~/coderaft/myproject/coderaft.json | python3 -m json.tool

# Or use coderaft validation
coderaft config validate myproject

# Fix common JSON errors:
# - Missing commas between elements
# - Trailing commas
# - Unquoted strings
# - Mismatched brackets/braces
```

##### "Setup commands fail during initialization"

**Problem**: Commands in `setup_commands` array fail.

**Diagnosis**:
```bash
# Check Island logs during init
docker logs coderaft_myproject

# Test commands manually
coderaft shell myproject
# Run each setup command individually
```

**Solutions**:
```bash
# Common fixes:
# 1. Add 'apt update' before package installs (though coderaft does this automatically)
# 2. Use full package names
# 3. Add '-y' flag to apt commands
# 4. Check command syntax

# Example working setup_commands:
{
  "setup_commands": [
    "apt install -y python3-pip nodejs npm",
    "pip3 install flask requests",
    "npm install -g typescript"
  ]
}

# Test commands step by step
coderaft shell myproject
apt install -y python3-pip  # Should work
pip3 install flask          # Should work
```

## Performance Issues
---

##### "Island startup is slow"

**Problem**: Takes a long time to start Islands or run commands.

**Solutions**:
```bash
# Check Docker performance
docker system df
docker system prune  # Clean up unused resources

# Monitor during startup
time coderaft shell myproject

# Check system resources
docker stats --no-stream
top
```

##### "High disk usage"

**Problem**: Docker/coderaft using too much disk space.

**Solutions**:
```bash
# Check disk usage
coderaft cleanup --dry-run --all
docker system df -v

# Clean up unused resources
coderaft cleanup --all
docker system prune -a

# Check individual Islands
docker exec coderaft_myproject du -sh /var/cache/apt
coderaft run myproject "apt autoclean"
```

## Recovery Procedures
---

##### "Complete reset of coderaft"

If everything is broken, start fresh:

```bash
# Stop all coderaft Islands
docker stop $(docker ps -q --filter "name=coderaft_")

# Remove all coderaft Islands
docker rm $(docker ps -aq --filter "name=coderaft_")

# Clean up Docker resources
docker system prune -a

# Remove coderaft configuration
rm -rf ~/.coderaft/

# Keep or remove project files (your choice)
# rm -rf ~/coderaft/  # This deletes your code!

# Reinstall coderaft if needed
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/install.sh | bash
```

##### "Recover project after Island deletion"

If Island was deleted but files remain:

```bash
# Check if files exist
ls ~/coderaft/myproject/

# Recreate Island
coderaft init myproject

# If you had custom configuration
# Edit ~/coderaft/myproject/coderaft.json
# Then recreate:
coderaft destroy myproject
coderaft init myproject
```

##### "Fix corrupted configuration"

If global configuration is corrupted:

```bash
# Backup existing config
cp ~/.coderaft/config.json ~/.coderaft/config.json.backup

# Reset configuration
rm ~/.coderaft/config.json

# Recreate projects
coderaft init project1
coderaft init project2
# etc.
```

## Getting Help
---

##### Debug Information

When reporting issues, include:

```bash
# System information
uname -a
cat /etc/os-release

# Docker information
docker --version
docker info

# Coderaft information
coderaft --version
coderaft list --verbose

# Island information (if applicable)
docker logs coderaft_myproject
docker inspect coderaft_myproject

# Configuration
cat ~/.coderaft/config.json
cat ~/coderaft/myproject/coderaft.json
```

##### Log Files

Useful log locations:
- Docker daemon: `journalctl -u docker.service`
- Island logs: `docker logs coderaft_<project>`
- System messages: `/var/log/syslog`

##### Common Commands for Diagnosis

```bash
# Check Docker daemon
sudo systemctl status docker

# List all Islands
docker ps -a

# Check Docker disk usage
docker system df

# Test Docker functionality
docker run hello-world

# Check coderaft projects
coderaft list
coderaft maintenance --health-check

# Check system resources
df -h
free -h
```
