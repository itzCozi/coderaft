package testutil

import (
	"path/filepath"
	"testing"
)

func TestCreateTestConfig(t *testing.T) {
	cfg := CreateTestConfig()

	AssertNotNil(t, cfg)
	AssertNotNil(t, cfg.Projects)
	AssertNotNil(t, cfg.Settings)

	AssertEqual(t, "ubuntu:noble", cfg.Settings.DefaultBaseImage)
	AssertEqual(t, true, cfg.Settings.AutoUpdate)
	AssertEqual(t, true, cfg.Settings.AutoStopOnExit)
}

func TestCreateTestProject(t *testing.T) {
	projectName := "test-project"
	project := CreateTestProject(projectName)

	AssertNotNil(t, project)
	AssertEqual(t, projectName, project.Name)
	AssertEqual(t, "coderaft_"+projectName, project.IslandName)
	AssertEqual(t, "ubuntu:noble", project.BaseImage)
	AssertEqual(t, "stopped", project.Status)
}

func TestCreateTestProjectConfig(t *testing.T) {
	projectName := "test-config"
	projectConfig := CreateTestProjectConfig(projectName)

	AssertNotNil(t, projectConfig)
	AssertEqual(t, projectName, projectConfig.Name)
	AssertEqual(t, "ubuntu:noble", projectConfig.BaseImage)
	AssertEqual(t, "/island", projectConfig.WorkingDir)
	AssertEqual(t, "/bin/bash", projectConfig.Shell)

	if len(projectConfig.SetupCommands) == 0 {
		t.Error("Expected setup commands to be non-empty")
	}

	if len(projectConfig.Environment) == 0 {
		t.Error("Expected environment to be non-empty")
	}

	if len(projectConfig.Ports) == 0 {
		t.Error("Expected ports to be non-empty")
	}
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, "test", "test")
	AssertEqual(t, 123, 123)
	AssertEqual(t, true, true)
}

func TestAssertNotNil(t *testing.T) {
	AssertNotNil(t, "test")
	AssertNotNil(t, 123)
	AssertNotNil(t, []string{})
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"contains at start", "hello world", "hello", true},
		{"contains in middle", "hello world", "lo wo", true},
		{"contains at end", "hello world", "world", true},
		{"exact match", "test", "test", true},
		{"not contains", "hello world", "xyz", false},
		{"empty substr", "hello", "", true},
		{"empty string", "", "test", false},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("Contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid alphanumeric", "project123", true},
		{"valid with hyphens", "my-project", true},
		{"valid with underscores", "my_project", true},
		{"valid mixed", "my-project_123", true},
		{"empty name", "", false},
		{"with spaces", "my project", false},
		{"with special chars", "my@project", false},
		{"with dots", "my.project", false},
		{"uppercase", "MyProject", true},
		{"single char", "a", true},
		{"number only", "123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("ValidateProjectName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCreateTempDir(t *testing.T) {
	tempDir := CreateTempDir(t)

	if tempDir == "" {
		t.Error("Expected non-empty temp directory path")
	}

	if !filepath.IsAbs(tempDir) {
		t.Error("Expected absolute path for temp directory")
	}
}

func TestCreateConfigManager(t *testing.T) {
	cm, tempDir := CreateConfigManager(t)

	AssertNotNil(t, cm)

	if tempDir == "" {
		t.Error("Expected non-empty temp directory path")
	}
}

func TestWriteAndReadJSONFile(t *testing.T) {
	tempDir := CreateTempDir(t)
	testFile := filepath.Join(tempDir, "test.json")

	testData := map[string]interface{}{
		"name":    "test",
		"version": 123,
		"active":  true,
	}

	WriteJSONFile(t, testFile, testData)

	var readData map[string]interface{}
	ReadJSONFile(t, testFile, &readData)

	if readData["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", readData["name"])
	}

	if readData["version"] != float64(123) {
		t.Errorf("Expected version 123, got %v", readData["version"])
	}

	if readData["active"] != true {
		t.Errorf("Expected active true, got %v", readData["active"])
	}
}
