package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"coderaft/internal/ui"
)

type ImageCache struct {
	imageExistsFunc func(ref string) bool
}

func NewImageCache() *ImageCache {
	return &ImageCache{}
}

func NewImageCacheWithSDK(existsFn func(ref string) bool) *ImageCache {
	return &ImageCache{imageExistsFunc: existsFn}
}

type BuildImageConfig struct {
	BaseImage     string
	SetupCommands []string
	Environment   map[string]string
	Labels        map[string]string
	WorkingDir    string
	Shell         string
	User          string
	ProjectName   string
}

func (cfg *BuildImageConfig) Fingerprint() string {
	h := sha256.New()
	h.Write([]byte(cfg.BaseImage))

	for _, cmd := range cfg.SetupCommands {
		h.Write([]byte(cmd))
	}

	envKeys := make([]string, 0, len(cfg.Environment))
	for k := range cfg.Environment {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		h.Write([]byte(k + "=" + cfg.Environment[k]))
	}

	h.Write([]byte(cfg.WorkingDir))
	h.Write([]byte(cfg.Shell))
	h.Write([]byte(cfg.User))

	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (ic *ImageCache) GenerateDockerfile(cfg *BuildImageConfig) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("FROM %s\n\n", cfg.BaseImage))

	for k, v := range cfg.Environment {
		b.WriteString(fmt.Sprintf("ENV %s=%q\n", k, v))
	}
	if len(cfg.Environment) > 0 {
		b.WriteString("\n")
	}

	for k, v := range cfg.Labels {
		b.WriteString(fmt.Sprintf("LABEL %s=%q\n", k, v))
	}

	var aptInstallPkgs []string
	var otherCommands []string
	var hasAptUpdate bool

	for _, cmd := range cfg.SetupCommands {
		cmdLower := strings.ToLower(strings.TrimSpace(cmd))

		switch {
		case cmdLower == "apt update -y" || cmdLower == "apt-get update -y" ||
			cmdLower == "apt update" || cmdLower == "apt-get update":
			hasAptUpdate = true
		case cmdLower == "apt full-upgrade -y" || cmdLower == "apt-get upgrade -y" ||
			cmdLower == "apt-get dist-upgrade -y":

			continue
		case strings.HasPrefix(cmdLower, "apt install ") || strings.HasPrefix(cmdLower, "apt-get install "):

			pkgs := extractAptPackages(cmd)
			aptInstallPkgs = append(aptInstallPkgs, pkgs...)
		case strings.HasPrefix(cmdLower, "debian_frontend=noninteractive apt install") ||
			strings.HasPrefix(cmdLower, "debian_frontend=noninteractive apt-get install"):
			pkgs := extractAptPackages(cmd)
			aptInstallPkgs = append(aptInstallPkgs, pkgs...)
		default:
			otherCommands = append(otherCommands, cmd)
		}
	}

	if len(aptInstallPkgs) > 0 || hasAptUpdate {
		b.WriteString("# System packages (single cached layer)\n")
		b.WriteString("RUN apt-get update -y \\\n")
		if len(aptInstallPkgs) > 0 {
			sort.Strings(aptInstallPkgs)
			b.WriteString("    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \\\n")
			for i, pkg := range aptInstallPkgs {
				if i < len(aptInstallPkgs)-1 {
					b.WriteString(fmt.Sprintf("       %s \\\n", pkg))
				} else {
					b.WriteString(fmt.Sprintf("       %s \\\n", pkg))
				}
			}
		}

		b.WriteString("    && apt-get clean \\\n")
		b.WriteString("    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*\n\n")
	}

	if len(otherCommands) > 0 {
		b.WriteString("# Setup commands\n")

		batchSize := 5
		for i := 0; i < len(otherCommands); i += batchSize {
			end := i + batchSize
			if end > len(otherCommands) {
				end = len(otherCommands)
			}
			batch := otherCommands[i:end]

			b.WriteString("RUN ")
			for j, cmd := range batch {
				if j > 0 {
					b.WriteString(" \\\n    && ")
				}
				b.WriteString(cmd)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	workDir := "/workspace"
	if cfg.WorkingDir != "" {
		workDir = cfg.WorkingDir
	}
	b.WriteString(fmt.Sprintf("WORKDIR %s\n", workDir))

	b.WriteString("CMD [\"sleep\", \"infinity\"]\n")

	return b.String()
}

func (ic *ImageCache) BuildCachedImage(cfg *BuildImageConfig) (string, error) {
	fingerprint := cfg.Fingerprint()
	imageTag := fmt.Sprintf("coderaft-cache/%s:%s", cfg.ProjectName, fingerprint)

	if ic.imageExistsFunc != nil {
		if ic.imageExistsFunc(imageTag) {
			ui.Status("using cached image %s", imageTag)
			return imageTag, nil
		}
	} else {
		checkCmd := exec.Command(dockerCmd(), "images", "-q", imageTag)
		output, err := checkCmd.Output()
		if err == nil && len(strings.TrimSpace(string(output))) > 0 {
			ui.Status("using cached image %s", imageTag)
			return imageTag, nil
		}
	}

	tmpDir, err := os.MkdirTemp("", "coderaft-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create build directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfile := ic.GenerateDockerfile(cfg)
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	args := []string{"build", "-t", imageTag, "-f", dockerfilePath}

	args = append(args, "--build-arg", "BUILDKIT_INLINE_CACHE=1")

	args = append(args, tmpDir)

	ui.Status("building cached image (fingerprint: %s)...", fingerprint)
	cmd := exec.Command(dockerCmd(), args...)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to build cached image: %w", err)
	}

	ui.Success("image cached as %s", imageTag)
	return imageTag, nil
}

func (ic *ImageCache) CleanupImageCache(projectName string) error {
	prefix := fmt.Sprintf("coderaft-cache/%s", projectName)
	cmd := exec.Command(dockerCmd(), "images", "--format", "{{.Repository}}:{{.Tag}}", prefix)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "<none>:<none>" {
			continue
		}
		ui.Status("removing cached image: %s", line)
		exec.Command(dockerCmd(), "rmi", line).Run()
	}
	return nil
}

func extractAptPackages(cmd string) []string {

	cmd = strings.TrimPrefix(cmd, "DEBIAN_FRONTEND=noninteractive ")
	cmd = strings.TrimPrefix(cmd, "apt install ")
	cmd = strings.TrimPrefix(cmd, "apt-get install ")

	var pkgs []string
	for _, token := range strings.Fields(cmd) {

		if strings.HasPrefix(token, "-") {
			continue
		}
		token = strings.TrimSpace(token)
		if token != "" {
			pkgs = append(pkgs, token)
		}
	}
	return pkgs
}

func MountConsistencyFlag() string {
	switch runtime.GOOS {
	case "darwin":

		return ":delegated"
	default:

		return ""
	}
}
