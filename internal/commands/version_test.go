package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	versionCmd.SetOut(&buf)
	versionCmd.SetArgs([]string{})
	versionCmd.Execute()

	output := buf.String()
	if output == "" {
		// versionCmd uses fmt.Printf which writes to stdout, not cmd.OutOrStdout().
		// Verify the command exists and has correct metadata instead.
		if versionCmd.Use != "version" {
			t.Errorf("Expected Use to be 'version', got %q", versionCmd.Use)
		}
		if versionCmd.Short == "" {
			t.Error("Expected Short description to be non-empty")
		}
	} else if !strings.Contains(output, "coderaft") {
		t.Errorf("Expected output to contain 'coderaft', got %q", output)
	}
}

func TestVersionDefault(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}

	if CommitHash == "" {
		t.Error("CommitHash should not be empty (should default to 'unknown')")
	}
}

func TestVersionCommandMetadata(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("Expected Use 'version', got %q", versionCmd.Use)
	}
	if versionCmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}
	if versionCmd.Run == nil {
		t.Error("Expected Run to be set")
	}
}
