---
title: Installation Guide
description: How to install coderaft on your Debian/Ubuntu system
---

```bash
# GitHub (Primaty)
curl -fsSL https://raw.githubusercontent.com/itzcozi/coderaft/main/install.sh | bash

# Mirror (CDN)
curl -fsSL https://coderaft.ar0.eu/install.sh | bash
```

:::note

We recommend GitHub as a reliable hosting solution; many providers may not fully trust our domain. Use the GitHub link for faster downloads and better reliability.
:::

This script will automatically:
- Check system compatibility (Debian/Ubuntu only)
- Install Go, Docker, make, and git if needed
- Clone the repository and build coderaft
- Install coderaft to `/usr/local/bin`
- Set up proper permissions

<sub>Already done here? Head over to the [Quick Start Guide](/docs/start/) to learn how to use coderaft.</sub>

## Manual Build from Source
---

If you prefer to build coderaft manually or the automatic script doesn't work for your system:

### Install Dependencies
```bash
sudo apt update \
	&& sudo apt install -y docker.io golang-go make git \
	&& sudo systemctl enable --now docker \
	&& sudo usermod -aG docker $USER
# Note: log out/in (or run `newgrp docker`) for group changes to take effect.
```

### Build and Install
```bash
# Clone the repository
git clone https://github.com/itzcozi/coderaft.git
cd coderaft

# Build the binary
make build

# Install to system (requires sudo)
sudo make install
```

## File Locations
---

- **Project files**: `~/coderaft/<project>/` (on host)
- **Island workspace**: `/workspace/` (inside Island)
- **Configuration**: `~/.coderaft/config.json`

## Next Steps
---

Now that you have coderaft installed, quickly get started by following the [Quick Start Guide](/docs/start/).
