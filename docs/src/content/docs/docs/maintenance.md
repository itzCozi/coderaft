---
title: Maintenance
description: Managing islands and system resources
---

## Cleanup Commands

```bash
# Remove stopped containers
coderaft cleanup

# Remove everything (containers + unused images)
coderaft cleanup --all

# Dry run
coderaft cleanup --dry-run
```

## Project Management

```bash
# List all projects
coderaft list

# Check status
coderaft status [project]

# Stop a project
coderaft stop <project>

# Remove a project
coderaft destroy <project>
```

## Backup & Restore

```bash
# Backup current state
coderaft backup <project>

# Backup to custom directory
coderaft backup <project> --output /path/to/backup

# Restore from backup directory
coderaft restore <project> <backup-dir>
```

## Updates

The CLI builds from source, so update by rebuilding:

```bash
cd /path/to/coderaft
git pull
go build -o coderaft ./cmd/coderaft
```

## Docker Maintenance

```bash
# Prune unused resources
docker system prune

# Disk usage
docker system df

# Remove all images
docker image prune -a
```
