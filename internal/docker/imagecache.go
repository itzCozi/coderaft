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
)

// ImageCache manages Dockerfile-based image building with layer caching.
// Instead of running setup commands via sequential `docker exec` calls
// (which spawns a new process per command and gets no caching), we generate
// a Dockerfile with all setup baked in and build it once. Docker's build
// cache ensures subsequent builds with the same commands are instant.
type ImageCache struct{}

func NewImageCache() *ImageCache {
	return &ImageCache{}
}

// BuildImageConfig holds parameters for building a cached devbox image.
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

// Fingerprint generates a deterministic hash of the build config.
// When the fingerprint matches an existing image, we skip the build entirely.
func (cfg *BuildImageConfig) Fingerprint() string {
	h := sha256.New()
	h.Write([]byte(cfg.BaseImage))

	for _, cmd := range cfg.SetupCommands {
		h.Write([]byte(cmd))
	}

	// Sort env keys for deterministic hashing
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

// GenerateDockerfile creates an optimized Dockerfile from the build config.
// Key optimizations:
//   - Groups apt commands into a single RUN layer (fewer layers = smaller image)
//   - Cleans apt cache in the same layer (keeps image small)
//   - Uses --no-install-recommends to avoid unnecessary packages
//   - Batches non-apt commands to reduce layers
func (ic *ImageCache) GenerateDockerfile(cfg *BuildImageConfig) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("FROM %s\n\n", cfg.BaseImage))

	// Set environment variables early so they're available during build
	for k, v := range cfg.Environment {
		b.WriteString(fmt.Sprintf("ENV %s=%q\n", k, v))
	}
	if len(cfg.Environment) > 0 {
		b.WriteString("\n")
	}

	// For labels
	for k, v := range cfg.Labels {
		b.WriteString(fmt.Sprintf("LABEL %s=%q\n", k, v))
	}

	// Separate apt commands from other commands to optimize layer caching.
	// APT commands are merged into a single RUN with cache cleanup.
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
			// Skip system upgrades in cached builds - base image is already up to date
			// and upgrades invalidate the entire cache on every security patch
			continue
		case strings.HasPrefix(cmdLower, "apt install ") || strings.HasPrefix(cmdLower, "apt-get install "):
			// Extract package names from apt install commands
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

	// Single optimized APT layer: update + install + cleanup all in one RUN
	if len(aptInstallPkgs) > 0 || hasAptUpdate {
		b.WriteString("# System packages (single cached layer)\n")
		b.WriteString("RUN apt-get update -y \\\n")
		if len(aptInstallPkgs) > 0 {
			sort.Strings(aptInstallPkgs) // Sort for cache determinism
			b.WriteString("    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \\\n")
			for i, pkg := range aptInstallPkgs {
				if i < len(aptInstallPkgs)-1 {
					b.WriteString(fmt.Sprintf("       %s \\\n", pkg))
				} else {
					b.WriteString(fmt.Sprintf("       %s \\\n", pkg))
				}
			}
		}
		// Clean up apt cache in the same layer to keep image small
		b.WriteString("    && apt-get clean \\\n")
		b.WriteString("    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*\n\n")
	}

	// Batch remaining commands into groups to reduce layers
	if len(otherCommands) > 0 {
		b.WriteString("# Setup commands\n")

		// Group into batches of 5 to balance rebuild granularity vs layer count
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

	// Set working directory
	workDir := "/workspace"
	if cfg.WorkingDir != "" {
		workDir = cfg.WorkingDir
	}
	b.WriteString(fmt.Sprintf("WORKDIR %s\n", workDir))

	// Use a lightweight init process
	b.WriteString("CMD [\"sleep\", \"infinity\"]\n")

	return b.String()
}

// BuildCachedImage generates a Dockerfile and builds it with BuildKit caching.
// Returns the image tag. If a cached image with matching fingerprint exists,
// the build completes instantly thanks to Docker layer cache.
func (ic *ImageCache) BuildCachedImage(cfg *BuildImageConfig) (string, error) {
	fingerprint := cfg.Fingerprint()
	imageTag := fmt.Sprintf("devbox-cache/%s:%s", cfg.ProjectName, fingerprint)

	// Check if image already exists (instant cache hit)
	checkCmd := exec.Command(dockerCmd(), "images", "-q", imageTag)
	output, err := checkCmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		fmt.Printf("Using cached image %s\n", imageTag)
		return imageTag, nil
	}

	// Create temp build context
	tmpDir, err := os.MkdirTemp("", "devbox-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create build directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfile := ic.GenerateDockerfile(cfg)
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Build with BuildKit for better caching and parallel layer builds
	args := []string{"build", "-t", imageTag, "-f", dockerfilePath}

	// Use inline cache for layer reuse across builds
	args = append(args, "--build-arg", "BUILDKIT_INLINE_CACHE=1")

	args = append(args, tmpDir)

	fmt.Printf("Building cached image (fingerprint: %s)...\n", fingerprint)
	cmd := exec.Command(dockerCmd(), args...)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to build cached image: %w", err)
	}

	fmt.Printf("Image cached as %s\n", imageTag)
	return imageTag, nil
}

// CleanupImageCache removes all cached devbox images for a project.
func (ic *ImageCache) CleanupImageCache(projectName string) error {
	prefix := fmt.Sprintf("devbox-cache/%s", projectName)
	cmd := exec.Command(dockerCmd(), "images", "--format", "{{.Repository}}:{{.Tag}}", prefix)
	output, err := cmd.Output()
	if err != nil {
		return nil // No images to clean
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "<none>:<none>" {
			continue
		}
		fmt.Printf("Removing cached image: %s\n", line)
		exec.Command(dockerCmd(), "rmi", line).Run()
	}
	return nil
}

// extractAptPackages parses an apt install command and returns the package names.
func extractAptPackages(cmd string) []string {
	// Remove common prefixes
	cmd = strings.TrimPrefix(cmd, "DEBIAN_FRONTEND=noninteractive ")
	cmd = strings.TrimPrefix(cmd, "apt install ")
	cmd = strings.TrimPrefix(cmd, "apt-get install ")

	var pkgs []string
	for _, token := range strings.Fields(cmd) {
		// Skip flags
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

// MountConsistencyFlag returns the optimal bind mount consistency mode
// for the current OS. On macOS, Docker bind mounts are notoriously slow
// without delegated/cached mode. On Linux, native mounts are fast.
// On Windows with WSL2, consistent mode is fine.
func MountConsistencyFlag() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS Docker Desktop: delegated = host writes propagate to container
		// eventually (fastest), cached = container reads from cache (fast)
		return ":delegated"
	default:
		// Linux: native bind mounts, Windows WSL2: already fast
		return ""
	}
}
