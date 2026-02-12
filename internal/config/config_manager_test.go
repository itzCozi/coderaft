package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager_LoadAndSave(t *testing.T) {
	tempDir := t.TempDir()

	cm := &ConfigManager{
		configPath: filepath.Join(tempDir, "config.json"),
	}

	config, err := cm.Load()
	if err != nil {
		t.Fatalf("Failed to load config when file doesn't exist: %v", err)
	}

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if config.Projects == nil {
		t.Error("Projects map should not be nil")
	}

	if config.Settings == nil {
		t.Error("Settings should not be nil")
	}

	if config.Settings.DefaultBaseImage != "ubuntu:22.04" {
		t.Errorf("Expected default base image 'ubuntu:22.04', got %q", config.Settings.DefaultBaseImage)
	}

	if !config.Settings.AutoUpdate {
		t.Error("Expected AutoUpdate to be true by default")
	}

	if !config.Settings.AutoStopOnExit {
		t.Error("Expected AutoStopOnExit to be true by default")
	}

	testProject := &Project{
		Name:          "test-project",
		IslandName:       "test-project-island",
		BaseImage:     "ubuntu:22.04",
		WorkspacePath: "/home/user/coderaft/test-project",
		Status:        "stopped",
	}
	config.Projects["test-project"] = testProject

	err = cm.Save(config)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		t.Error("Config file should have been created")
	}

	loadedConfig, err := cm.Load()
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if len(loadedConfig.Projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(loadedConfig.Projects))
	}

	loadedProject, exists := loadedConfig.Projects["test-project"]
	if !exists {
		t.Fatal("Test project should exist in loaded config")
	}

	if loadedProject.Name != testProject.Name {
		t.Errorf("Expected project name %q, got %q", testProject.Name, loadedProject.Name)
	}

	if loadedProject.IslandName != testProject.IslandName {
		t.Errorf("Expected island name %q, got %q", testProject.IslandName, loadedProject.IslandName)
	}
}

func TestConfigManager_LoadProjectConfig(t *testing.T) {
	tempDir := t.TempDir()

	cm := &ConfigManager{
		configPath: filepath.Join(tempDir, "config.json"),
	}

	projectConfig, err := cm.LoadProjectConfig(tempDir)
	if err != nil {
		t.Fatalf("Failed to load project config when file doesn't exist: %v", err)
	}

	if projectConfig != nil {
		t.Error("Project config should be nil when file doesn't exist")
	}

	testProjectConfig := &ProjectConfig{
		Name:      "test-project",
		BaseImage: "ubuntu:22.04",
		SetupCommands: []string{
			"apt update",
			"apt install -y python3",
		},
		Environment: map[string]string{
			"PYTHONPATH": "/workspace",
		},
		Ports:      []string{"5000:5000"},
		WorkingDir: "/workspace",
	}

	configPath := filepath.Join(tempDir, "coderaft.json")
	data, err := json.MarshalIndent(testProjectConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal project config: %v", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write project config file: %v", err)
	}

	loadedProjectConfig, err := cm.LoadProjectConfig(tempDir)
	if err != nil {
		t.Fatalf("Failed to load project config: %v", err)
	}

	if loadedProjectConfig == nil {
		t.Fatal("Loaded project config should not be nil")
	}

	if loadedProjectConfig.Name != testProjectConfig.Name {
		t.Errorf("Expected name %q, got %q", testProjectConfig.Name, loadedProjectConfig.Name)
	}

	if loadedProjectConfig.BaseImage != testProjectConfig.BaseImage {
		t.Errorf("Expected base image %q, got %q", testProjectConfig.BaseImage, loadedProjectConfig.BaseImage)
	}

	if len(loadedProjectConfig.SetupCommands) != len(testProjectConfig.SetupCommands) {
		t.Errorf("Expected %d setup commands, got %d", len(testProjectConfig.SetupCommands), len(loadedProjectConfig.SetupCommands))
	}
}

func TestConfigManager_LoadEmptyConfig(t *testing.T) {
	tempDir := t.TempDir()

	cm := &ConfigManager{
		configPath: filepath.Join(tempDir, "config.json"),
	}

	err := os.WriteFile(cm.configPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty config file: %v", err)
	}

	config, err := cm.Load()
	if err != nil {
		t.Fatalf("Failed to load empty config: %v", err)
	}

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if config.Projects == nil {
		t.Error("Projects map should not be nil")
	}

	if config.Settings == nil {
		t.Error("Settings should not be nil")
	}
}

func TestConfigManager_LoadInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()

	cm := &ConfigManager{
		configPath: filepath.Join(tempDir, "config.json"),
	}

	invalidJSON := `{"projects": {invalid json}`
	err := os.WriteFile(cm.configPath, []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid config file: %v", err)
	}

	config, err := cm.Load()
	if err == nil {
		t.Error("Expected error when loading invalid JSON config")
	}

	if config != nil {
		t.Error("Config should be nil when JSON is invalid")
	}

	if !contains(err.Error(), "failed to parse config file") {
		t.Errorf("Expected error to mention parsing failure, got %q", err.Error())
	}
}

func TestConfig_GetProject(t *testing.T) {
	config := &Config{
		Projects: map[string]*Project{
			"project1": {
				Name:    "project1",
				IslandName: "project1-island",
			},
			"project2": {
				Name:    "project2",
				IslandName: "project2-island",
			},
		},
	}

	project, exists := config.GetProject("project1")
	if !exists {
		t.Error("Expected project1 to exist")
	}

	if project == nil {
		t.Fatal("Project should not be nil")
	}

	if project.Name != "project1" {
		t.Errorf("Expected project name 'project1', got %q", project.Name)
	}

	project, exists = config.GetProject("nonexistent")
	if exists {
		t.Error("Expected nonexistent project to not exist")
	}

	if project != nil {
		t.Error("Project should be nil for non-existing project")
	}
}
