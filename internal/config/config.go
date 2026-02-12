package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"errors"

	"github.com/xeipuuv/gojsonschema"
)

type Config struct {
	Projects map[string]*Project `json:"projects"`
	Settings *GlobalSettings     `json:"settings,omitempty"`
}

type GlobalSettings struct {
	DefaultBaseImage    string            `json:"default_base_image,omitempty"`
	DefaultEnvironment  map[string]string `json:"default_environment,omitempty"`
	ConfigTemplatesPath string            `json:"config_templates_path,omitempty"`
	AutoUpdate          bool              `json:"auto_update,omitempty"`
	AutoStopOnExit      bool              `json:"auto_stop_on_exit,omitempty"`
	AutoApplyLock       bool              `json:"auto_apply_lock,omitempty"`
}

type Project struct {
	Name          string `json:"name"`
	BoxName       string `json:"box_name"`
	BaseImage     string `json:"base_image"`
	WorkspacePath string `json:"workspace_path"`
	Status        string `json:"status,omitempty"`
	ConfigFile    string `json:"config_file,omitempty"`
}

type ProjectConfig struct {
	Name          string            `json:"name"`
	BaseImage     string            `json:"base_image,omitempty"`
	SetupCommands []string          `json:"setup_commands,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	Ports         []string          `json:"ports,omitempty"`
	Volumes       []string          `json:"volumes,omitempty"`
	Dotfiles      []string          `json:"dotfiles,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	Shell         string            `json:"shell,omitempty"`
	User          string            `json:"user,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Network       string            `json:"network,omitempty"`
	Restart       string            `json:"restart,omitempty"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
	Resources     *Resources        `json:"resources,omitempty"`
	Gpus          string            `json:"gpus,omitempty"`
}

type HealthCheck struct {
	Test        []string `json:"test,omitempty"`
	Interval    string   `json:"interval,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
	StartPeriod string   `json:"start_period,omitempty"`
	Retries     int      `json:"retries,omitempty"`
}

type Resources struct {
	CPUs   string `json:"cpus,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type ConfigTemplate struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Config      ProjectConfig `json:"config"`
}

type ConfigManager struct {
	configPath string
}

func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".devbox")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	templatesDir := filepath.Join(configDir, "templates")
	_ = os.MkdirAll(templatesDir, 0755)

	configPath := filepath.Join(configDir, "config.json")
	return &ConfigManager{configPath: configPath}, nil
}

func (cm *ConfigManager) Load() (*Config, error) {
	config := &Config{
		Projects: make(map[string]*Project),
		Settings: &GlobalSettings{
			DefaultBaseImage: "ubuntu:22.04",
			AutoUpdate:       true,
			AutoStopOnExit:   true,
			AutoApplyLock:    true,
		},
	}

	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return config, nil
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) == 0 {
		return config, nil
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Settings == nil {
		config.Settings = &GlobalSettings{
			DefaultBaseImage: "ubuntu:22.04",
			AutoUpdate:       true,
			AutoStopOnExit:   true,
			AutoApplyLock:    true,
		}
	}

	return config, nil
}

func (cm *ConfigManager) Save(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (cm *ConfigManager) LoadProjectConfig(projectPath string) (*ProjectConfig, error) {

	candidates := []string{
		filepath.Join(projectPath, "devbox.json"),
		filepath.Join(projectPath, "devbox.project.json"),
		filepath.Join(projectPath, ".devbox.json"),
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
		filepath.Join(projectPath, "devbox.json"),
		filepath.Join(projectPath, "devbox.project.json"),
		filepath.Join(projectPath, ".devbox.json"),
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

	for _, port := range cfg.Ports {
		if !strings.Contains(port, ":") && !strings.Contains(port, "/") {

			return fmt.Errorf("invalid port mapping '%s' (expected host:container or container[/proto])", port)
		}
	}
	for _, volume := range cfg.Volumes {
		if !strings.Contains(volume, ":") {
			return fmt.Errorf("invalid volume mapping '%s' (expected host:container)", volume)
		}
	}
	if cfg.HealthCheck != nil {
		if len(cfg.HealthCheck.Test) > 0 && cfg.HealthCheck.Test[0] == "NONE" && len(cfg.HealthCheck.Test) > 1 {
			return fmt.Errorf("health_check.test cannot have arguments when set to NONE")
		}

		if cfg.HealthCheck.Interval != "" {
			if _, err := time.ParseDuration(strings.ReplaceAll(cfg.HealthCheck.Interval, "m", "m0s")); err != nil && !durationLike(cfg.HealthCheck.Interval) {

			}
		}
	}
	return nil
}

func durationLike(s string) bool {

	for _, suf := range []string{"ns", "us", "ms", "s", "m", "h"} {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func (cm *ConfigManager) GetDefaultProjectConfig(projectName string) *ProjectConfig {
	return &ProjectConfig{
		Name:        projectName,
		BaseImage:   "ubuntu:22.04",
		WorkingDir:  "/workspace",
		Shell:       "/bin/bash",
		User:        "root",
		Restart:     "unless-stopped",
		Environment: make(map[string]string),
		Labels:      make(map[string]string),

		Volumes:       []string{},
		SetupCommands: []string{},
	}
}

func (cm *ConfigManager) CreateProjectConfigFromTemplate(templateName, projectName string) (*ProjectConfig, error) {
	templates := map[string]*ProjectConfig{
		"python": {
			Name:      projectName,
			BaseImage: "ubuntu:22.04",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends python3 python3-pip python3-venv python3-dev build-essential ca-certificates",
				"pip3 install --upgrade pip setuptools wheel",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"PYTHONPATH":       "/workspace",
				"PYTHONUNBUFFERED": "1",
			},
			Ports:   []string{"8000:8000", "5000:5000"},
			Volumes: []string{},
		},
		"nodejs": {
			Name:      projectName,
			BaseImage: "ubuntu:22.04",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends curl ca-certificates gnupg build-essential",
				"curl -fsSL https://deb.nodesource.com/setup_18.x | bash -",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends nodejs",
				"npm install -g npm@latest",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"NODE_ENV": "development",
				"PATH":     "/workspace/node_modules/.bin:$PATH",
			},
			Ports:   []string{"3000:3000", "8080:8080"},
			Volumes: []string{},
		},
		"go": {
			Name:      projectName,
			BaseImage: "ubuntu:22.04",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends wget git build-essential ca-certificates",
				"wget -q -O /tmp/go.tar.gz https://go.dev/dl/go1.21.0.linux-amd64.tar.gz",
				"tar -C /usr/local -xzf /tmp/go.tar.gz && rm /tmp/go.tar.gz",
				"echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"GOPATH": "/workspace/go",
				"PATH":   "/usr/local/go/bin:$PATH",
			},
			Ports:   []string{"8080:8080"},
			Volumes: []string{},
		},
		"web": {
			Name:      projectName,
			BaseImage: "ubuntu:22.04",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends python3 python3-pip nodejs npm nginx git curl wget ca-certificates gnupg",
				"curl -fsSL https://deb.nodesource.com/setup_18.x | bash -",
				"pip3 install flask django fastapi",
				"npm install -g typescript vue-cli create-react-app",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"PYTHONPATH": "/workspace",
				"NODE_ENV":   "development",
			},
			Ports:   []string{"3000:3000", "5000:5000", "8000:8000", "80:80"},
			Volumes: []string{},
		},
	}

	template, exists := templates[templateName]
	if !exists {
		if t, err := cm.LoadUserTemplate(templateName); err == nil && t != nil {
			data, _ := json.Marshal(t.Config)
			var cfg ProjectConfig
			_ = json.Unmarshal(data, &cfg)
			cfg.Name = projectName
			return &cfg, nil
		}
		return nil, fmt.Errorf("template '%s' not found", templateName)
	}

	configData, _ := json.Marshal(template)
	var config ProjectConfig
	json.Unmarshal(configData, &config)
	config.Name = projectName

	return &config, nil
}

func (cm *ConfigManager) GetAvailableTemplates() []string {
	builtins := []string{"python", "nodejs", "go", "web"}

	user := cm.ListUserTemplates()
	if len(user) == 0 {
		return builtins
	}
	return append(builtins, user...)
}

func (cm *ConfigManager) templatesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".devbox", "templates"), nil
}

func (cm *ConfigManager) ListUserTemplates() []string {
	dir, err := cm.templatesDir()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".json") {
			name = name[:len(name)-5]
			names = append(names, name)
		}
	}
	return names
}

func (cm *ConfigManager) LoadUserTemplate(name string) (*ConfigTemplate, error) {
	dir, err := cm.templatesDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get templates directory: %w", err)
	}
	path := filepath.Join(dir, name+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}
	var tpl ConfigTemplate
	if err := json.Unmarshal(b, &tpl); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template: %w", err)
	}
	return &tpl, nil
}

func (cm *ConfigManager) SaveUserTemplate(tpl *ConfigTemplate) error {
	dir, err := cm.templatesDir()
	if err != nil {
		return fmt.Errorf("failed to get templates directory: %w", err)
	}
	if tpl.Name == "" {
		return fmt.Errorf("template name is required")
	}
	b, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, tpl.Name+".json"), b, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}
	return nil
}

func (cm *ConfigManager) DeleteUserTemplate(name string) error {
	dir, err := cm.templatesDir()
	if err != nil {
		return fmt.Errorf("failed to get templates directory: %w", err)
	}
	path := filepath.Join(dir, name+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("template '%s' not found", name)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	return nil
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
		project.ConfigFile = filepath.Join(project.WorkspacePath, "devbox.json")
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
	return "ubuntu:22.04"
}

const ProjectConfigJSONSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "Devbox Project Config",
	"type": "object",
	"required": ["name"],
	"properties": {
		"name": {"type": "string", "minLength": 1},
		"base_image": {"type": "string"},
		"setup_commands": {"type": "array", "items": {"type": "string"}},
		"environment": {"type": "object", "additionalProperties": {"type": "string"}},
		"ports": {"type": "array", "items": {"type": "string"}},
		"volumes": {"type": "array", "items": {"type": "string"}},
		"dotfiles": {"type": "array", "items": {"type": "string"}},
		"working_dir": {"type": "string"},
		"shell": {"type": "string"},
		"user": {"type": "string"},
		"capabilities": {"type": "array", "items": {"type": "string"}},
		"labels": {"type": "object", "additionalProperties": {"type": "string"}},
		"network": {"type": "string"},
		"restart": {"type": "string"},
		"health_check": {
			"type": "object",
			"properties": {
				"test": {"type": "array", "items": {"type": "string"}},
				"interval": {"type": "string"},
				"timeout": {"type": "string"},
				"start_period": {"type": "string"},
				"retries": {"type": "integer", "minimum": 0}
			},
			"additionalProperties": false
		},
		"resources": {
			"type": "object",
			"properties": {
				"cpus": {"type": "string"},
				"memory": {"type": "string"}
			},
			"additionalProperties": false
		},
		"gpus": {"type": "string"}
	},
	"additionalProperties": false
}`
