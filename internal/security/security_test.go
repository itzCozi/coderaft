package security

import (
	"strings"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		errMsg  string
	}{
		{"myproject", false, ""},
		{"my-project", false, ""},
		{"my_project", false, ""},
		{"MyProject123", false, ""},
		{"project1", false, ""},
		{"", true, "cannot be empty"},
		{"123project", true, "must start with a letter"},
		{"-project", true, "must start with a letter"},
		{"_project", true, "must start with a letter"},
		{"project@name", true, "can only contain"},
		{"project name", true, "can only contain"},
		{"docker", true, "reserved name"},
		{"coderaft", true, "reserved name"},
		{"system", true, "reserved name"},
		{strings.Repeat("a", 65), true, "cannot exceed 64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProjectName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateProjectName(%q) error = %v, want error containing %q", tt.name, err, tt.errMsg)
				}
			}
		})
	}
}

func TestIsSensitiveEnvVar(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"PATH", false},
		{"HOME", false},
		{"USER", false},
		{"PASSWORD", true},
		{"DB_PASSWORD", true},
		{"API_KEY", true},
		{"api_key", true},
		{"SECRET_KEY", true},
		{"AWS_ACCESS_KEY_ID", true},
		{"GITHUB_TOKEN", true},
		{"AUTH_TOKEN", true},
		{"SSH_KEY", true},
		{"DATABASE_URL", true},
		{"CONNECTION_STRING", true},
		{"MY_APP_CONFIG", false},
		{"DEBUG", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSensitiveEnvVar(tt.name); got != tt.want {
				t.Errorf("IsSensitiveEnvVar(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestFilterSensitiveEnvVars(t *testing.T) {
	input := map[string]string{
		"PATH":         "/usr/bin",
		"HOME":         "/home/user",
		"API_KEY":      "super-secret-key",
		"DB_PASSWORD":  "secret123",
		"NODE_ENV":     "development",
		"GITHUB_TOKEN": "ghp_xxxx",
	}

	result := FilterSensitiveEnvVars(input)

	if result["PATH"] != "/usr/bin" {
		t.Errorf("PATH should not be redacted, got %q", result["PATH"])
	}
	if result["HOME"] != "/home/user" {
		t.Errorf("HOME should not be redacted, got %q", result["HOME"])
	}
	if result["NODE_ENV"] != "development" {
		t.Errorf("NODE_ENV should not be redacted, got %q", result["NODE_ENV"])
	}
	if result["API_KEY"] != "[REDACTED]" {
		t.Errorf("API_KEY should be redacted, got %q", result["API_KEY"])
	}
	if result["DB_PASSWORD"] != "[REDACTED]" {
		t.Errorf("DB_PASSWORD should be redacted, got %q", result["DB_PASSWORD"])
	}
	if result["GITHUB_TOKEN"] != "[REDACTED]" {
		t.Errorf("GITHUB_TOKEN should be redacted, got %q", result["GITHUB_TOKEN"])
	}
}

func TestIsSensitivePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/etc/passwd", true},
		{"/etc/shadow", true},
		{"/root/.ssh", true},
		{"/var/run/docker.sock", true},
		{"/home/user/code", false},
		{"/tmp/test", false},
		{"/island/project", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsSensitivePath(tt.path); got != tt.want {
				t.Errorf("IsSensitivePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidateVolumePath(t *testing.T) {
	tests := []struct {
		volumeSpec string
		wantErr    bool
	}{
		{"/home/user/code:/island", false},
		{"/tmp:/tmp", false},
		{"/etc/passwd:/passwd", true},
		{"/var/run/docker.sock:/var/run/docker.sock", true},
		{"/root/.ssh:/ssh", true},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.volumeSpec, func(t *testing.T) {
			err := ValidateVolumePath(tt.volumeSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVolumePath(%q) error = %v, wantErr %v", tt.volumeSpec, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeShellArg(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		{"hello world", "hello world"},
		{"with;semicolon", "'with;semicolon'"},
		{"with|pipe", "'with|pipe'"},
		{"$(command)", "'$(command)'"},
		{"`command`", "'`command`'"},
		{"$variable", "'$variable'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeShellArg(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeShellArg(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizePathForError(t *testing.T) {

	result := SanitizePathForError("/some/path/to/file.txt")
	if result == "" {
		t.Error("SanitizePathForError returned empty string")
	}
}

func TestWrapShellCommand(t *testing.T) {
	cmd := "echo hello"
	wrapped := WrapShellCommand(cmd)

	if !strings.Contains(wrapped, "bashrc") {
		t.Error("WrapShellCommand should source bashrc")
	}
	if !strings.Contains(wrapped, "set -e") {
		t.Error("WrapShellCommand should set -e for fail-fast")
	}
	if !strings.Contains(wrapped, cmd) {
		t.Error("WrapShellCommand should contain the original command")
	}
}
