---
title: Troubleshooting
description: Common issues and fixes
---

## Quick Fixes

| Problem | Solution |
|---------|----------|
| Container won't start | `coderaft cleanup --all` then retry |
| Volume permission errors | Add `user: "1000:1000"` to config |
| Port already in use | Change port or `docker ps` to find conflict |
| Changes not persisting | Check volume mounts in config |
| Slow performance | Adjust `resources` in config |

## Common Errors

### "Cannot connect to Docker daemon"

```bash
# Start Docker service
sudo systemctl start docker

# Or check Docker Desktop is running (Windows/macOS)
```

### "Image not found"

```bash
# Pull image manually
docker pull buildpack-deps:bookworm
```

### "Container already exists"

```bash
coderaft destroy my-project
coderaft init my-project
```

### "Permission denied"

```bash
# Add user to docker group
sudo usermod -aG docker $USER
# Log out and back in
```

## Debug Commands

```bash
# Check status
coderaft status

# View Docker logs
docker logs <container-id>

# Inspect container
docker inspect <container-id>

# Reset everything
coderaft cleanup --all
```

## Still Stuck?

1. Check [FAQ](/docs/faq/)
2. Try `coderaft cleanup --all` and reinitialize
3. Open an issue with `coderaft status` output
