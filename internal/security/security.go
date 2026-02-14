package security

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var Timeouts = struct {
	DockerStartup    time.Duration
	DockerPing       time.Duration
	ContainerExec    time.Duration
	Apply            time.Duration
	UserConfirmation time.Duration
	PollInterval     time.Duration
	MaxPollInterval  time.Duration
}{
	DockerStartup:    60 * time.Second,
	DockerPing:       10 * time.Second,
	ContainerExec:    30 * time.Second,
	Apply:            300 * time.Second,
	UserConfirmation: 30 * time.Second,
	PollInterval:     25 * time.Millisecond,
	MaxPollInterval:  500 * time.Millisecond,
}

var Limits = struct {
	MaxProjectNameLength int
	MaxSetupBatchSize    int
	DefaultWorkerCount   int
}{
	MaxProjectNameLength: 64,
	MaxSetupBatchSize:    10,
	DefaultWorkerCount:   4,
}

var ReservedProjectNames = map[string]bool{
	"coderaft":   true,
	"docker":     true,
	"system":     true,
	"admin":      true,
	"root":       true,
	"container":  true,
	"island":     true,
	"help":       true,
	"version":    true,
	"completion": true,
}

var SensitiveEnvPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)password`),
	regexp.MustCompile(`(?i)passwd`),
	regexp.MustCompile(`(?i)secret`),
	regexp.MustCompile(`(?i)token`),
	regexp.MustCompile(`(?i)api[_-]?key`),
	regexp.MustCompile(`(?i)auth`),
	regexp.MustCompile(`(?i)credential`),
	regexp.MustCompile(`(?i)private[_-]?key`),
	regexp.MustCompile(`(?i)access[_-]?key`),
	regexp.MustCompile(`(?i)aws[_-]?`),
	regexp.MustCompile(`(?i)gcp[_-]?`),
	regexp.MustCompile(`(?i)azure[_-]?`),
	regexp.MustCompile(`(?i)ssh[_-]?`),
	regexp.MustCompile(`(?i)bearer`),
	regexp.MustCompile(`(?i)jwt`),
	regexp.MustCompile(`(?i)oauth`),
	regexp.MustCompile(`(?i)encryption`),
	regexp.MustCompile(`(?i)decryption`),
	regexp.MustCompile(`(?i)signing`),
	regexp.MustCompile(`(?i)database[_-]?url`),
	regexp.MustCompile(`(?i)db[_-]?url`),
	regexp.MustCompile(`(?i)connection[_-]?string`),
}

var SensitivePaths = []string{
	"/etc/passwd",
	"/etc/shadow",
	"/etc/sudoers",
	"/etc/ssh",
	"/root/.ssh",
	"/.ssh",
	"/var/run/docker.sock",
}

var dangerousShellCharsPattern = regexp.MustCompile(`[;&|$\x60\\<>(){}[\]!#*?~^]`)

func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	if len(name) > Limits.MaxProjectNameLength {
		return fmt.Errorf("project name cannot exceed %d characters", Limits.MaxProjectNameLength)
	}

	if len(name) > 0 && !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		return fmt.Errorf("project name must start with a letter")
	}

	validPattern := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	if !validPattern.MatchString(name) {
		return fmt.Errorf("project name can only contain alphanumeric characters, hyphens, and underscores, and must start with a letter")
	}

	if ReservedProjectNames[strings.ToLower(name)] {
		return fmt.Errorf("'%s' is a reserved name and cannot be used as a project name", name)
	}

	return nil
}

func SanitizePath(userPath string, basePath string) (string, error) {
	if userPath == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	if strings.HasPrefix(userPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		userPath = filepath.Join(home, userPath[1:])
	}

	cleanPath := filepath.Clean(userPath)

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	if basePath != "" {
		absBasePath, err := filepath.Abs(basePath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve base path: %w", err)
		}

		relPath, err := filepath.Rel(absBasePath, absPath)
		if err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}

		if strings.HasPrefix(relPath, "..") {
			return "", fmt.Errorf("path traversal detected: path escapes base directory")
		}
	}

	return absPath, nil
}

func SanitizePathForError(fullPath string) string {

	cwd, err := os.Getwd()
	if err == nil {
		relPath, err := filepath.Rel(cwd, fullPath)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			return relPath
		}
	}

	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(fullPath, home) {
		return "~" + strings.TrimPrefix(fullPath, home)
	}

	dir := filepath.Dir(fullPath)
	for _, sensitivePath := range []string{"/etc", "/var", "/root", "C:\\Windows", "C:\\Users"} {
		if strings.HasPrefix(dir, sensitivePath) {
			return filepath.Base(fullPath)
		}
	}

	return fullPath
}

func IsSensitiveEnvVar(name string) bool {
	for _, pattern := range SensitiveEnvPatterns {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}

func FilterSensitiveEnvVars(envMap map[string]string) map[string]string {
	filtered := make(map[string]string, len(envMap))
	for k, v := range envMap {
		if IsSensitiveEnvVar(k) {
			filtered[k] = "[REDACTED]"
		} else {
			filtered[k] = v
		}
	}
	return filtered
}

func IsSensitivePath(path string) bool {

	cleanPath := filepath.ToSlash(filepath.Clean(path))
	for _, sensitive := range SensitivePaths {
		if strings.HasPrefix(cleanPath, sensitive) || cleanPath == sensitive {
			return true
		}
	}
	return false
}

func ValidateVolumePath(volumeSpec string) error {
	parts := strings.SplitN(volumeSpec, ":", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid volume specification: expected 'source:target'")
	}

	source := parts[0]

	if len(parts) >= 3 && len(parts[0]) == 1 && ((parts[0][0] >= 'A' && parts[0][0] <= 'Z') || (parts[0][0] >= 'a' && parts[0][0] <= 'z')) {
		source = parts[0] + ":" + parts[1]
	}

	if strings.HasPrefix(source, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			source = filepath.Join(home, source[1:])
		}
	}

	if IsSensitivePath(source) {
		return fmt.Errorf("mounting sensitive system path '%s' is not allowed", SanitizePathForError(source))
	}

	return nil
}

func SanitizeShellArg(s string) string {

	s = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)

	if !dangerousShellCharsPattern.MatchString(s) {
		return s
	}

	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

func ValidateShellCommand(cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command cannot be empty")
	}

	for i, arg := range cmd {

		if strings.Contains(arg, "$(") || strings.Contains(arg, "`") {

			if i == 0 && (strings.HasPrefix(arg, "$(") || strings.HasPrefix(arg, "`")) {
				return fmt.Errorf("command substitution at start of command is not allowed for security reasons")
			}
		}
	}

	return nil
}

func WrapShellCommand(command string) string {

	return ". /root/.bashrc >/dev/null 2>&1 || true; set -e; " + command
}
