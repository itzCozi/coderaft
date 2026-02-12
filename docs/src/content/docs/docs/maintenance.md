---
title: Cleanup and Maintenance
description: Keep your coderaft development environment healthy and optimized
---

This guide covers coderaft's cleanup and maintenance features to keep your development environment healthy and optimized.

## Cleanup Command
---

The `coderaft cleanup` command helps maintain a clean system by removing various Docker resources and coderaft artifacts.

##### Interactive Cleanup

```bash
coderaft cleanup
```

This opens an interactive menu with the following options:

1. **Clean up orphaned coderaft Islands** - Remove Islands not tracked in config
2. **Remove unused Docker images** - Remove dangling and unused images
3. **Remove unused Docker volumes** - Remove unused volumes
4. **Remove unused Docker networks** - Remove unused networks
5. **Run Docker system prune** - Comprehensive cleanup of all unused resources
6. **Clean up everything** - Combines options 1-4
7. **Show system status** - Display disk usage and system information

##### Command-Line Flags

```bash
# Specific cleanup tasks
coderaft cleanup --orphaned           # Remove orphaned Islands only
coderaft cleanup --images             # Remove unused images only
coderaft cleanup --volumes            # Remove unused volumes only
coderaft cleanup --networks           # Remove unused networks only
coderaft cleanup --system-prune       # Run docker system prune
coderaft cleanup --all                # Clean up everything

# Safety and information
coderaft cleanup --dry-run            # Show what would be cleaned (no changes)
coderaft cleanup --force              # Skip confirmation prompts
```

##### Examples

```bash
# See what would be cleaned without making changes
coderaft cleanup --dry-run --all

# Clean only orphaned Islands
coderaft cleanup --orphaned

# Comprehensive cleanup with confirmation
coderaft cleanup --all

# Quick cleanup without prompts
coderaft cleanup --all --force
```

## Maintenance Command
---

The `coderaft maintenance` command provides system health monitoring, updates, and repair functionality.

##### Interactive Maintenance

```bash
coderaft maintenance
```

This opens an interactive menu with these options:

1. **Check system status** - Show Docker status, projects, and disk usage
2. **Perform health check** - Check health of all projects
3. **Update system packages** - Update packages in all Islands
4. **Restart stopped Islands** - Start any stopped coderaft Islands
5. **Rebuild all Islands** - Recreate Islands from latest base images
6. **Auto-repair common issues** - Automatically fix detected problems
7. **Full maintenance** - Combines health check, updates, and restarts

##### Command-Line Flags

```bash
# Individual maintenance tasks
coderaft maintenance --status         # Show detailed system status
coderaft maintenance --health-check   # Check health of all projects
coderaft maintenance --update         # Update all Islands
coderaft maintenance --restart        # Restart stopped Islands
coderaft maintenance --rebuild        # Rebuild all Islands
coderaft maintenance --auto-repair    # Auto-fix common issues

# Control flags
coderaft maintenance --force          # Skip confirmation prompts
```

##### Examples

```bash
# Check system health
coderaft maintenance --health-check

# Update all Islands
coderaft maintenance --update

# Rebuild all Islands (with confirmation)
coderaft maintenance --rebuild

# Quick full maintenance without prompts
coderaft maintenance --force --health-check --update --restart
```

## Update Command
---

Use the `coderaft update` command to rebuild environment Islands from the latest base images. This is the recommended way to apply upstream image updates or configuration changes that affect the base image or setup commands.

##### Why use `coderaft update`?

- Pulls the newest base image(s)
- Recreates the Island with your current `coderaft.json` configuration
- Automatically runs a full system update inside the Island
- Re-runs your `setup_commands` to ensure tools are present
- Preserves your project files on the host at `~/coderaft/<project>/`

##### Usage

```bash
# Update a single project
coderaft update myproject

# Update all projects
coderaft update
```

##### When to use maintenance vs update

- `coderaft maintenance --update`: Update system packages inside existing Islands
- `coderaft update`: Rebuild Islands from the latest base images and re-apply configuration

If you're changing `base_image` in `coderaft.json` or want to ensure you are using the latest upstream image, use `coderaft update`.

## Health Checks
---

The health check system monitors:

- **Island Status**: Whether Islands are running or stopped
- **Island Responsiveness**: Whether Islands respond to commands
- **Workspace Directories**: Whether project directories exist
- **Configuration Files**: Whether coderaft.json files are valid

Health check results show:
- ✅ **Healthy**: Island running and responsive
- ⚠️ **Unhealthy**: Island stopped or unresponsive
- ❌ **Missing**: Island or workspace missing

## Auto-Repair
---

The auto-repair feature automatically fixes common issues:

- **Missing workspace directories**: Creates missing project directories
- **Missing Islands**: Recreates Islands from configuration
- **Stopped Islands**: Starts stopped Islands
- **Unresponsive Islands**: Restarts Islands that don't respond

## System Updates
---

The update feature:
1. Runs `apt update -y` to refresh package lists
2. Runs `apt full-upgrade -y` to install updates
3. Runs `apt autoremove -y` to remove unnecessary packages
4. Runs `apt autoclean` to clean package cache

Updates are applied to all tracked Islands that are running or can be started.

## Island Rebuilding
---

The rebuild feature:
1. Stops and removes existing Islands
2. Pulls latest base images
3. Recreates Islands with current configuration
4. Runs system updates
5. Executes setup commands from coderaft.json
6. Sets up coderaft environment

:::caution
Rebuilding preserves your project files but recreates the Island environment.
:::

## Monitoring
---

##### System Status
```bash
# Quick status overview
coderaft list

# Detailed system information
coderaft maintenance --status

# Docker resource usage
docker system df
```

##### Island Health
```bash
# Check all project health
coderaft maintenance --health-check

# Check specific Island
docker inspect coderaft_myproject

# View Island logs
docker logs coderaft_myproject
```

##### Resource Usage
```bash
# Live Island stats
docker stats

# Disk usage by type
docker system df -v

# List all coderaft Islands
docker ps -a --filter "name=coderaft_"
```
