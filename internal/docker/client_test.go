package docker

import (
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	if client == nil {
		t.Fatal("Client should not be nil")
	}
}

func TestClientClose(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

func TestIsDockerAvailable(t *testing.T) {
	err := IsDockerAvailable()

	if err != nil {
		t.Logf("Docker not available (this is expected in some test environments): %v", err)

		if !strings.Contains(err.Error(), "docker") {
			t.Errorf("Expected error to mention docker, got: %v", err)
		}
	} else {
		t.Log("Docker is available")
	}
}

func TestEscapeShellVar(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"my-project", "my-project"},
		{"project_name", "project_name"},
		{`say "hi"`, `say \"hi\"`},
		{"$HOME", `\$HOME`},
		{"`cmd`", "\\`cmd\\`"},
		{`back\slash`, `back\\slash`},
		{"with\nnewline", "withnewline"},
		{"with\r\ncrlf", "withcrlf"},
		{"normal_project123", "normal_project123"},
	}
	for _, tt := range tests {
		got := escapeShellVar(tt.in)
		if got != tt.want {
			t.Errorf("escapeShellVar(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
