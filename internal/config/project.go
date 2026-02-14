package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

var validLinuxCapabilities = map[string]bool{
	"ALL":              true,
	"AUDIT_CONTROL":    true,
	"AUDIT_READ":       true,
	"AUDIT_WRITE":      true,
	"BLOCK_SUSPEND":    true,
	"CHOWN":            true,
	"DAC_OVERRIDE":     true,
	"DAC_READ_SEARCH":  true,
	"FOWNER":           true,
	"FSETID":           true,
	"IPC_LOCK":         true,
	"IPC_OWNER":        true,
	"KILL":             true,
	"LEASE":            true,
	"LINUX_IMMUTABLE":  true,
	"MAC_ADMIN":        true,
	"MAC_OVERRIDE":     true,
	"MKNOD":            true,
	"NET_ADMIN":        true,
	"NET_BIND_SERVICE": true,
	"NET_BROADCAST":    true,
	"NET_RAW":          true,
	"SETFCAP":          true,
	"SETGID":           true,
	"SETPCAP":          true,
	"SETUID":           true,
	"SYS_ADMIN":        true,
	"SYS_BOOT":         true,
	"SYS_CHROOT":       true,
	"SYS_MODULE":       true,
	"SYS_NICE":         true,
	"SYS_PACCT":        true,
	"SYS_PTRACE":       true,
	"SYS_RAWIO":        true,
	"SYS_RESOURCE":     true,
	"SYS_TIME":         true,
	"SYS_TTY_CONFIG":   true,
	"SYSLOG":           true,
	"WAKE_ALARM":       true,
}

var imageRefPattern = regexp.MustCompile(`^([a-z0-9]+([._-][a-z0-9]+)*(:[0-9]+)?/)?([a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*)(:([\w][\w.-]{0,127}))?(@sha256:[a-f0-9]{64})?$`)

func (cm *ConfigManager) LoadProjectConfig(projectPath string) (*ProjectConfig, error) {

	candidates := []string{
		filepath.Join(projectPath, "coderaft.json"),
		filepath.Join(projectPath, "coderaft.project.json"),
		filepath.Join(projectPath, ".coderaft.json"),
	}

	var configPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			configPath = p
			break
		}
	}
	if configPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config file: %w", err)
	}

	var projectConfig ProjectConfig
	if err := json.Unmarshal(data, &projectConfig); err != nil {
		return nil, fmt.Errorf("failed to parse project config file: %w", err)
	}

	return &projectConfig, nil
}

func (cm *ConfigManager) SaveProjectConfig(projectPath string, config *ProjectConfig) error {

	candidates := []string{
		filepath.Join(projectPath, "coderaft.json"),
		filepath.Join(projectPath, "coderaft.project.json"),
		filepath.Join(projectPath, ".coderaft.json"),
	}
	configPath := candidates[0]
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			configPath = p
			break
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write project config file: %w", err)
	}

	return nil
}

func (cm *ConfigManager) ValidateProjectConfig(cfg *ProjectConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	sch := gojsonschema.NewStringLoader(ProjectConfigJSONSchema)
	docBytes, _ := json.Marshal(cfg)
	doc := gojsonschema.NewBytesLoader(docBytes)
	res, err := gojsonschema.Validate(sch, doc)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}
	if !res.Valid() {
		var b strings.Builder
		b.WriteString("project config invalid:\n")
		for _, e := range res.Errors() {
			b.WriteString(" - ")
			b.WriteString(e.String())
			b.WriteString("\n")
		}
		return errors.New(strings.TrimSpace(b.String()))
	}

	if cfg.BaseImage != "" {

		if !imageRefPattern.MatchString(strings.ToLower(cfg.BaseImage)) {

			if !strings.Contains(cfg.BaseImage, "/") && !strings.Contains(cfg.BaseImage, ":") {

			} else if strings.HasPrefix(cfg.BaseImage, "sha256:") {
				return fmt.Errorf("invalid base_image '%s': use image:tag@sha256:... format instead of bare digest", cfg.BaseImage)
			}
		}
	}

	if cfg.User != "" {

		userPattern := regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_-]{0,31}(:[a-zA-Z0-9_][a-zA-Z0-9_-]{0,31})?$|^[0-9]+(:[0-9]+)?$`)
		if !userPattern.MatchString(cfg.User) {
			return fmt.Errorf("invalid user '%s': expected 'username', 'uid', 'uid:gid', or 'username:group'", cfg.User)
		}
	}

	for _, cap := range cfg.Capabilities {
		capUpper := strings.ToUpper(cap)
		if !validLinuxCapabilities[capUpper] {
			return fmt.Errorf("invalid capability '%s': not a recognized Linux capability", cap)
		}
	}

	if cfg.Network != "" {
		validNetworks := map[string]bool{
			"bridge": true, "host": true, "none": true, "container": true,
		}
		netMode := strings.ToLower(cfg.Network)
		if !validNetworks[netMode] && !strings.HasPrefix(netMode, "container:") {

			if strings.ContainsAny(cfg.Network, " \t\n") {
				return fmt.Errorf("invalid network '%s': network name cannot contain whitespace", cfg.Network)
			}
		}
	}

	for _, port := range cfg.Ports {
		if !strings.Contains(port, ":") && !strings.Contains(port, "/") {

			return fmt.Errorf("invalid port mapping '%s' (expected host:island or island[/proto])", port)
		}
	}
	for _, volume := range cfg.Volumes {
		if !strings.Contains(volume, ":") {
			return fmt.Errorf("invalid volume mapping '%s' (expected host:island)", volume)
		}
	}
	if cfg.HealthCheck != nil {
		if len(cfg.HealthCheck.Test) > 0 && cfg.HealthCheck.Test[0] == "NONE" && len(cfg.HealthCheck.Test) > 1 {
			return fmt.Errorf("health_check.test cannot have arguments when set to NONE")
		}

		if cfg.HealthCheck.Interval != "" {
			if _, err := time.ParseDuration(strings.ReplaceAll(cfg.HealthCheck.Interval, "m", "m0s")); err != nil && !durationLike(cfg.HealthCheck.Interval) {
				return fmt.Errorf("invalid health_check.interval %q: %w", cfg.HealthCheck.Interval, err)
			}
		}
	}
	return nil
}

func durationLike(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, suf := range []string{"ns", "us", "ms", "s", "m", "h"} {
		if strings.HasSuffix(s, suf) {
			prefix := s[:len(s)-len(suf)]
			if len(prefix) == 0 {
				return false
			}

			hasDot := false
			for _, c := range prefix {
				if c == '.' {
					if hasDot {
						return false
					}
					hasDot = true
				} else if c < '0' || c > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (cm *ConfigManager) GetDefaultProjectConfig(projectName string) *ProjectConfig {
	return &ProjectConfig{
		Name:        projectName,
		BaseImage:   "buildpack-deps:bookworm",
		WorkingDir:  "/island",
		Shell:       "/bin/bash",
		User:        "root",
		Restart:     "unless-stopped",
		Environment: make(map[string]string),
		Labels:      make(map[string]string),

		Volumes:       []string{},
		SetupCommands: []string{},
	}
}

func (config *Config) AddProject(project *Project) {
	if config.Projects == nil {
		config.Projects = make(map[string]*Project)
	}
	config.Projects[project.Name] = project
}

func (config *Config) RemoveProject(name string) {
	if config.Projects != nil {
		delete(config.Projects, name)
	}
}

func (config *Config) GetProject(name string) (*Project, bool) {
	if config.Projects == nil {
		return nil, false
	}
	project, exists := config.Projects[name]
	return project, exists
}

func (config *Config) GetProjects() map[string]*Project {
	if config.Projects == nil {
		return make(map[string]*Project)
	}
	return config.Projects
}

func (config *Config) MergeProjectConfig(project *Project, projectConfig *ProjectConfig) {
	if projectConfig == nil {
		return
	}

	if projectConfig.BaseImage != "" {
		project.BaseImage = projectConfig.BaseImage
	}

	if projectConfig.Name != "" {
		project.ConfigFile = filepath.Join(project.WorkspacePath, "coderaft.json")
	}
}

func (config *Config) GetEffectiveBaseImage(project *Project, projectConfig *ProjectConfig) string {
	if projectConfig != nil && projectConfig.BaseImage != "" {
		return projectConfig.BaseImage
	}
	if project.BaseImage != "" {
		return project.BaseImage
	}
	if config.Settings != nil && config.Settings.DefaultBaseImage != "" {
		return config.Settings.DefaultBaseImage
	}
	return "buildpack-deps:bookworm"
}
