package config

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestNewConfigManager(t *testing.T) {
	cm, err := NewConfigManager()
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	if cm == nil {
		t.Fatal("Config manager should not be nil")
	}

	if cm.configPath == "" {
		t.Error("Config path should not be empty")
	}

	expectedPath := ".coderaft"
	if !filepath.IsAbs(cm.configPath) || !contains(cm.configPath, expectedPath) {
		t.Errorf("Config path should contain %q, got %q", expectedPath, cm.configPath)
	}
}

func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		healthCheck HealthCheck
		wantJSON    string
	}{
		{
			name: "complete health check",
			healthCheck: HealthCheck{
				Test:        []string{"CMD", "curl", "-f", "http://localhost:8080/health"},
				Interval:    "30s",
				Timeout:     "10s",
				StartPeriod: "60s",
				Retries:     3,
			},
			wantJSON: `{"test":["CMD","curl","-f","http://localhost:8080/health"],"interval":"30s","timeout":"10s","start_period":"60s","retries":3}`,
		},
		{
			name:        "empty health check",
			healthCheck: HealthCheck{},
			wantJSON:    `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.healthCheck)
			if err != nil {
				t.Fatalf("Failed to marshal health check: %v", err)
			}

			if string(jsonData) != tt.wantJSON {
				t.Errorf("Expected JSON %q, got %q", tt.wantJSON, string(jsonData))
			}

			var hc HealthCheck
			err = json.Unmarshal(jsonData, &hc)
			if err != nil {
				t.Fatalf("Failed to unmarshal health check: %v", err)
			}

			if hc.Interval != tt.healthCheck.Interval {
				t.Errorf("Expected interval %q, got %q", tt.healthCheck.Interval, hc.Interval)
			}
		})
	}
}

func TestResources(t *testing.T) {
	tests := []struct {
		name      string
		resources Resources
		wantJSON  string
	}{
		{
			name: "with CPU and memory",
			resources: Resources{
				CPUs:   "2.0",
				Memory: "4g",
			},
			wantJSON: `{"cpus":"2.0","memory":"4g"}`,
		},
		{
			name:      "empty resources",
			resources: Resources{},
			wantJSON:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.resources)
			if err != nil {
				t.Fatalf("Failed to marshal resources: %v", err)
			}

			if string(jsonData) != tt.wantJSON {
				t.Errorf("Expected JSON %q, got %q", tt.wantJSON, string(jsonData))
			}
		})
	}
}

func TestProjectConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ProjectConfig
		wantErr bool
	}{
		{
			name: "valid project config",
			config: ProjectConfig{
				Name:      "test-project",
				BaseImage: "ubuntu:22.04",
				SetupCommands: []string{
					"apt update",
					"apt install -y python3",
				},
				Environment: map[string]string{
					"PYTHONPATH": "/workspace",
					"ENV":        "development",
				},
				Ports:      []string{"8080:8080", "3000:3000"},
				WorkingDir: "/workspace",
			},
			wantErr: false,
		},
		{
			name: "empty project config",
			config: ProjectConfig{
				Name: "empty-project",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				var pc ProjectConfig
				err = json.Unmarshal(jsonData, &pc)
				if err != nil {
					t.Errorf("Failed to unmarshal: %v", err)
					return
				}

				if pc.Name != tt.config.Name {
					t.Errorf("Expected name %q, got %q", tt.config.Name, pc.Name)
				}

				if pc.BaseImage != tt.config.BaseImage {
					t.Errorf("Expected base image %q, got %q", tt.config.BaseImage, pc.BaseImage)
				}

				if len(pc.SetupCommands) != len(tt.config.SetupCommands) {
					t.Errorf("Expected %d setup commands, got %d", len(tt.config.SetupCommands), len(pc.SetupCommands))
				}
			}
		})
	}
}

func TestProject(t *testing.T) {
	project := &Project{
		Name:          "test-project",
		BoxName:       "test-project-box",
		BaseImage:     "ubuntu:22.04",
		WorkspacePath: "/home/user/coderaft/test-project",
		Status:        "running",
		ConfigFile:    "/home/user/coderaft/test-project/coderaft.json",
	}

	jsonData, err := json.Marshal(project)
	if err != nil {
		t.Fatalf("Failed to marshal project: %v", err)
	}

	var p Project
	err = json.Unmarshal(jsonData, &p)
	if err != nil {
		t.Fatalf("Failed to unmarshal project: %v", err)
	}

	if p.Name != project.Name {
		t.Errorf("Expected name %q, got %q", project.Name, p.Name)
	}

	if p.BoxName != project.BoxName {
		t.Errorf("Expected box name %q, got %q", project.BoxName, p.BoxName)
	}

	if p.Status != project.Status {
		t.Errorf("Expected status %q, got %q", project.Status, p.Status)
	}
}

func TestGlobalSettings(t *testing.T) {
	settings := &GlobalSettings{
		DefaultBaseImage:    "ubuntu:20.04",
		DefaultEnvironment:  map[string]string{"PATH": "/usr/local/bin"},
		ConfigTemplatesPath: "/home/user/.coderaft/templates",
		AutoUpdate:          true,
		AutoStopOnExit:      false,
	}

	jsonData, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("Failed to marshal global settings: %v", err)
	}

	var gs GlobalSettings
	err = json.Unmarshal(jsonData, &gs)
	if err != nil {
		t.Fatalf("Failed to unmarshal global settings: %v", err)
	}

	if gs.DefaultBaseImage != settings.DefaultBaseImage {
		t.Errorf("Expected default base image %q, got %q", settings.DefaultBaseImage, gs.DefaultBaseImage)
	}

	if gs.AutoUpdate != settings.AutoUpdate {
		t.Errorf("Expected auto update %v, got %v", settings.AutoUpdate, gs.AutoUpdate)
	}

	if gs.AutoStopOnExit != settings.AutoStopOnExit {
		t.Errorf("Expected auto stop on exit %v, got %v", settings.AutoStopOnExit, gs.AutoStopOnExit)
	}
}

func TestConfigTemplate(t *testing.T) {
	template := ConfigTemplate{
		Name:        "python-dev",
		Description: "Python development environment",
		Config: ProjectConfig{
			Name:      "python-project",
			BaseImage: "ubuntu:22.04",
			SetupCommands: []string{
				"apt update",
				"apt install -y python3 python3-pip",
			},
			Environment: map[string]string{
				"PYTHONPATH": "/workspace",
			},
			Ports:      []string{"5000:5000"},
			WorkingDir: "/workspace",
		},
	}

	jsonData, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("Failed to marshal config template: %v", err)
	}

	var ct ConfigTemplate
	err = json.Unmarshal(jsonData, &ct)
	if err != nil {
		t.Fatalf("Failed to unmarshal config template: %v", err)
	}

	if ct.Name != template.Name {
		t.Errorf("Expected name %q, got %q", template.Name, ct.Name)
	}

	if ct.Description != template.Description {
		t.Errorf("Expected description %q, got %q", template.Description, ct.Description)
	}

	if ct.Config.Name != template.Config.Name {
		t.Errorf("Expected config name %q, got %q", template.Config.Name, ct.Config.Name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAtIndex(s, substr)))
}

func containsAtIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
