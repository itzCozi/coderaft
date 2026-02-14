package docker

import (
	"bufio"
	"strings"
)

func (c *Client) GetAptSources(islandName string) (snapshotURL string, sources []string, release string) {

	out, _, err := c.ExecCapture(islandName, "cat /etc/apt/sources.list 2>/dev/null; echo; cat /etc/apt/sources.list.d/*.list 2>/dev/null || true")
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			sources = append(sources, line)
			if strings.Contains(line, "snapshot.debian.org") || strings.Contains(line, "snapshot.ubuntu.com") {

				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
						snapshotURL = p
						break
					}
				}
			}
		}
	}

	if relOut, _, err2 := c.ExecCapture(islandName, ". /etc/os-release 2>/dev/null; echo $VERSION_CODENAME"); err2 == nil {
		release = strings.TrimSpace(relOut)
	}
	return
}

func (c *Client) GetPipRegistries(islandName string) (indexURL string, extra []string) {

	out, _, err := c.ExecCapture(islandName, "(pip3 config debug || pip config debug) 2>/dev/null | sed -n 's/^ *index-url *= *//p; s/^ *extra-index-url *= *//p'")
	if err == nil && strings.TrimSpace(out) != "" {

		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if indexURL == "" && (strings.Contains(l, "://") || strings.HasPrefix(l, "file:")) {
				indexURL = l
			} else {
				extra = append(extra, l)
			}
		}
	}
	if indexURL == "" {

		if conf, _, err2 := c.ExecCapture(islandName, "grep -hE '^(index-url|extra-index-url)' /etc/pip.conf ~/.pip/pip.conf 2>/dev/null || true"); err2 == nil {
			for _, line := range strings.Split(conf, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "index-url") && indexURL == "" {
					if i := strings.Index(line, "="); i != -1 {
						indexURL = strings.TrimSpace(line[i+1:])
					}
				}
				if strings.HasPrefix(line, "extra-index-url") {
					if i := strings.Index(line, "="); i != -1 {
						extra = append(extra, strings.TrimSpace(line[i+1:]))
					}
				}
			}
		}
	}
	return
}

func (c *Client) GetNodeRegistries(islandName string) (npmReg, yarnReg, pnpmReg string) {
	if out, _, err := c.ExecCapture(islandName, "npm config get registry 2>/dev/null || true"); err == nil {
		npmReg = strings.TrimSpace(out)
	}
	if out, _, err := c.ExecCapture(islandName, "yarn config get npmRegistryServer 2>/dev/null || true"); err == nil {
		yarnReg = strings.TrimSpace(out)
	}
	if out, _, err := c.ExecCapture(islandName, "pnpm config get registry 2>/dev/null || true"); err == nil {
		pnpmReg = strings.TrimSpace(out)
	}
	return
}
