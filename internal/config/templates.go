package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultNodeVersion = "22"
	DefaultGoVersion   = "1.24.0"
)

func getNodeVersion() string {
	if v := os.Getenv("CODERAFT_NODE_VERSION"); v != "" {
		return strings.TrimPrefix(v, "v")
	}
	return DefaultNodeVersion
}

func getGoVersion() string {
	if v := os.Getenv("CODERAFT_GO_VERSION"); v != "" {
		return strings.TrimPrefix(v, "go")
	}
	return DefaultGoVersion
}

func (cm *ConfigManager) templatesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".coderaft", "templates"), nil
}

func (cm *ConfigManager) CreateProjectConfigFromTemplate(templateName, projectName string) (*ProjectConfig, error) {
	nodeVersion := getNodeVersion()
	goVersion := getGoVersion()

	templates := map[string]*ProjectConfig{
		"python": {
			Name:      projectName,
			BaseImage: "buildpack-deps:bookworm",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends python3 python3-pip python3-venv python3-dev build-essential ca-certificates",
				"pip3 install --upgrade pip setuptools wheel",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"PYTHONPATH":       "/island",
				"PYTHONUNBUFFERED": "1",
			},
			Ports:   []string{"8000:8000", "5000:5000"},
			Volumes: []string{},
		},
		"nodejs": {
			Name:      projectName,
			BaseImage: "buildpack-deps:bookworm",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends curl ca-certificates gnupg build-essential",
				fmt.Sprintf("curl -fsSL https://deb.nodesource.com/setup_%s.x | bash -", nodeVersion),
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends nodejs",
				"npm install -g npm@latest",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"NODE_ENV": "development",
				"PATH":     "/island/node_modules/.bin:$PATH",
			},
			Ports:   []string{"3000:3000", "8080:8080"},
			Volumes: []string{},
		},
		"go": {
			Name:      projectName,
			BaseImage: "buildpack-deps:bookworm",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends wget git build-essential ca-certificates",
				fmt.Sprintf("wget -q -O /tmp/go.tar.gz https://go.dev/dl/go%s.linux-amd64.tar.gz", goVersion),
				"tar -C /usr/local -xzf /tmp/go.tar.gz && rm /tmp/go.tar.gz",
				"echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"GOPATH": "/island/go",
				"PATH":   "/usr/local/go/bin:$PATH",
			},
			Ports:   []string{"8080:8080"},
			Volumes: []string{},
		},
		"web": {
			Name:      projectName,
			BaseImage: "buildpack-deps:bookworm",
			SetupCommands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends python3 python3-pip nodejs npm nginx git curl wget ca-certificates gnupg",
				fmt.Sprintf("curl -fsSL https://deb.nodesource.com/setup_%s.x | bash -", nodeVersion),
				"pip3 install flask django fastapi",
				"npm install -g typescript @vue/cli create-next-app",
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			},
			Environment: map[string]string{
				"PYTHONPATH": "/island",
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

	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") || name == "." {
		return nil, fmt.Errorf("invalid template name: %s", name)
	}
	path := filepath.Join(dir, name+".json")

	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)) {
		return nil, fmt.Errorf("invalid template path")
	}
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

	if strings.Contains(tpl.Name, "/") || strings.Contains(tpl.Name, "\\") || strings.Contains(tpl.Name, "..") || tpl.Name == "." {
		return fmt.Errorf("invalid template name: %s", tpl.Name)
	}
	b, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}
	path := filepath.Join(dir, tpl.Name+".json")

	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)) {
		return fmt.Errorf("invalid template path")
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}
	return nil
}

func (cm *ConfigManager) DeleteUserTemplate(name string) error {
	dir, err := cm.templatesDir()
	if err != nil {
		return fmt.Errorf("failed to get templates directory: %w", err)
	}

	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") || name == "." {
		return fmt.Errorf("invalid template name: %s", name)
	}
	path := filepath.Join(dir, name+".json")

	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)) {
		return fmt.Errorf("invalid template path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("template '%s' not found", name)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	return nil
}
