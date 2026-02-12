package commands

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			if CommitHash != "" && CommitHash != "unknown" {
				buf.WriteString(fmt.Sprintf("coderaft (v%s, commit %s)\n", Version, CommitHash))
			} else {
				buf.WriteString(fmt.Sprintf("coderaft (v%s)\n", Version))
			}
		},
	}

	cmd.Run(cmd, []string{})

	output := strings.TrimSpace(buf.String())
	if !strings.Contains(output, "coderaft (v"+Version) {
		t.Errorf("Expected output to contain version string, got %q", output)
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
