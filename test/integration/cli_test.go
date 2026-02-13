package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if err := buildCoderaftBinary(); err != nil {
		panic("Failed to build coderaft binary for testing: " + err.Error())
	}

	code := m.Run()

	cleanupTestBinary()

	os.Exit(code)
}

func testBinaryName() string {
	if runtime.GOOS == "windows" {
		return "coderaft-test.exe"
	}
	return "coderaft-test"
}

func buildCoderaftBinary() error {
	binaryName := testBinaryName()
	cmd := exec.Command("go", "build", "-o", binaryName, "./cmd/coderaft")
	cmd.Dir = getProjectRoot()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build coderaft binary: %w", err)
	}
	return nil
}

func cleanupTestBinary() {
	testBinary := filepath.Join(getProjectRoot(), testBinaryName())
	os.Remove(testBinary)
}

func getProjectRoot() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}

func getTestBinaryPath() string {
	basePath := getProjectRoot()
	return filepath.Join(basePath, testBinaryName())
}

func TestVersionCommand(t *testing.T) {
	cmd := exec.Command(getTestBinaryPath(), "version")
	output, err := cmd.CombinedOutput()

	outputStr := strings.TrimSpace(string(output))

	if err != nil && !strings.Contains(outputStr, "coderaft") {
		t.Fatalf("Failed to run version command: %v, Output: %s", err, outputStr)
	}

	expectedPrefix := "coderaft (v"
	if strings.Contains(outputStr, expectedPrefix) && strings.Contains(outputStr, "1.0") {
		return
	}

	if strings.Contains(outputStr, "only runs on") {
		t.Skip("Skipping version test on unsupported OS")
		return
	}

	t.Errorf("Expected output to contain version info, got %q", outputStr)
}

func TestHelpCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"root help", []string{"--help"}},
		{"help command", []string{"help"}},
		{"init help", []string{"init", "--help"}},
		{"version help", []string{"version", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getTestBinaryPath(), tt.args...)
			output, err := cmd.CombinedOutput()

			outputStr := string(output)

			if !strings.Contains(outputStr, "coderaft") && !strings.Contains(outputStr, "Usage:") {
				if len(output) == 0 && err != nil {
					t.Fatalf("Failed to run help command %v: %v", tt.args, err)
				}
			}

			if strings.Contains(outputStr, "coderaft") || strings.Contains(outputStr, "Usage:") {
				return
			}

			t.Errorf("Expected help output to contain usage info, got %q", outputStr)
		})
	}
}

func TestInvalidCommand(t *testing.T) {
	cmd := exec.Command(getTestBinaryPath(), "invalid-command")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("Expected invalid command to return error")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "unknown command") && !strings.Contains(outputStr, "Error") {
		t.Errorf("Expected error message for invalid command, got %q", outputStr)
	}
}

func TestInitCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		shouldError bool
		errorText   string
	}{
		{
			name:        "no project name",
			args:        []string{"init"},
			shouldError: true,
			errorText:   "arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getTestBinaryPath(), tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected command to fail but it succeeded. Output: %s", string(output))
					return
				}

				outputStr := string(output)

				validErrors := []string{"arg", "required", "Usage:"}
				hasValidError := false
				for _, validError := range validErrors {
					if strings.Contains(outputStr, validError) {
						hasValidError = true
						break
					}
				}

				if !hasValidError {
					t.Errorf("Expected error to contain one of %v, got %q", validErrors, outputStr)
				}
			} else {
				if err != nil {
					t.Errorf("Expected command to succeed but it failed: %v. Output: %s", err, string(output))
				}
			}
		})
	}
}

func TestConfigCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		shouldError bool
		errorText   string
	}{
		{
			name:        "no subcommand",
			args:        []string{"config"},
			shouldError: true,
			errorText:   "arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getTestBinaryPath(), tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected command to fail but it succeeded. Output: %s", string(output))
					return
				}

				outputStr := string(output)

				validErrors := []string{"arg", "required", "Usage:"}
				hasValidError := false
				for _, validError := range validErrors {
					if strings.Contains(outputStr, validError) {
						hasValidError = true
						break
					}
				}

				if !hasValidError {
					t.Errorf("Expected error to contain one of %v, got %q", validErrors, outputStr)
				}
			} else {
				if err != nil {
					t.Errorf("Expected command to succeed but it failed: %v. Output: %s", err, string(output))
				}
			}
		})
	}
}
