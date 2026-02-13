package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"coderaft/internal/config"
)

func CreateTempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func testHomePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join("C:\\Users", "user")
	}
	return filepath.Join("/home", "user")
}

func CreateTestConfig() *config.Config {
	return &config.Config{
		Projects: make(map[string]*config.Project),
		Settings: &config.GlobalSettings{
			DefaultBaseImage:    "buildpack-deps:bookworm",
			DefaultEnvironment:  map[string]string{"PATH": "/usr/local/bin:/usr/bin:/bin"},
			ConfigTemplatesPath: "",
			AutoUpdate:          true,
			AutoStopOnExit:      true,
		},
	}
}

func CreateTestProject(name string) *config.Project {
	return &config.Project{
		Name:          name,
		IslandName:    "coderaft_" + name,
		BaseImage:     "buildpack-deps:bookworm",
		WorkspacePath: filepath.Join(testHomePath(), "coderaft", name),
		Status:        "stopped",
		ConfigFile:    filepath.Join(testHomePath(), "coderaft", name, "coderaft.json"),
	}
}

func CreateTestProjectConfig(name string) *config.ProjectConfig {
	return &config.ProjectConfig{
		Name:      name,
		BaseImage: "buildpack-deps:bookworm",
		SetupCommands: []string{
			"apt update",
			"apt install -y curl git",
		},
		Environment: map[string]string{
			"ENV":  "development",
			"USER": "developer",
		},
		Ports: []string{
			"8080:8080",
			"3000:3000",
		},
		Volumes: []string{
			"/island/data:/data",
		},
		WorkingDir: "/island",
		Shell:      "/bin/bash",
		User:       "root",
	}
}

func WriteJSONFile(t *testing.T, path string, data interface{}) {
	t.Helper()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	err = os.WriteFile(path, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to write JSON file: %v", err)
	}
}

func ReadJSONFile(t *testing.T, path string, dest interface{}) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	err = json.Unmarshal(data, dest)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func AssertError(t *testing.T, err error, expectedMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if expectedMessage != "" && !Contains(err.Error(), expectedMessage) {
		t.Fatalf("Expected error to contain %q, got %q", expectedMessage, err.Error())
	}
}

func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected value to not be nil")
	}
}

func AssertNil(t *testing.T, value interface{}) {
	t.Helper()
	if value != nil {
		t.Fatalf("Expected value to be nil, got %v", value)
	}
}

func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func CreateConfigManager(t *testing.T) (*config.ConfigManager, string) {
	t.Helper()

	tempDir := CreateTempDir(t)
	configDir := filepath.Join(tempDir, ".coderaft")

	cm, err := config.NewConfigManagerWithPath(configDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	return cm, tempDir
}

func ValidateProjectName(name string) bool {
	if name == "" {
		return false
	}

	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	return true
}
