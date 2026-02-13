package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage git hook integration for coderaft projects",
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install <project>",
	Short: "Install a git pre-commit hook that runs 'coderaft verify'",
	Long: `Install a git pre-commit hook in the project's workspace that
automatically runs 'coderaft verify <project>' before each commit.

If a lock file exists and the island has drifted from it, the commit will
be blocked with instructions to update the lock file or apply it.

The hook is a standard bash script and plays well with other hook managers
(husky, lefthook, etc.) — it exits 0 on success or when coderaft is not
installed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHooksInstall(args[0])
	},
}

var hooksRemoveCmd = &cobra.Command{
	Use:   "remove <project>",
	Short: "Remove the coderaft pre-commit hook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHooksRemove(args[0])
	},
}

const hookMarker = "# coderaft:pre-commit"

func hookScript(projectName string) string {
	return fmt.Sprintf(`#!/bin/sh
%s
# Automatically run 'coderaft verify' before committing.
# Remove this block or run 'coderaft hooks remove %s' to disable.
if command -v coderaft >/dev/null 2>&1; then
  if [ -f "coderaft.lock.json" ]; then
    echo "[coderaft] verifying island matches lock file..."
    if ! coderaft verify %s; then
      echo ""
      echo "[coderaft] island drift detected — commit blocked."
      echo "  Run 'coderaft lock %s' to update the lock file, or"
      echo "  Run 'coderaft apply %s' to reconcile the island."
      echo "  Use --no-verify to skip this check."
      exit 1
    fi
  fi
fi
`, hookMarker, projectName, projectName, projectName, projectName)
}

func runHooksInstall(projectName string) error {
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	proj, ok := cfg.GetProject(projectName)
	if !ok {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	gitDir := filepath.Join(proj.WorkspacePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("no .git directory found at %s — initialize a git repo first", proj.WorkspacePath)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	script := hookScript(projectName)

	
	if data, err := os.ReadFile(hookPath); err == nil {
		content := string(data)
		if strings.Contains(content, hookMarker) {
			ui.Warning("coderaft pre-commit hook is already installed")
			return nil
		}
		
		script = "\n" + script
		f, err := os.OpenFile(hookPath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return fmt.Errorf("failed to append to existing hook: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(script); err != nil {
			return fmt.Errorf("failed to write hook: %w", err)
		}
		ui.Success("appended coderaft verify hook to existing pre-commit hook")
		return nil
	}

	
	if err := os.WriteFile(hookPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write hook: %w", err)
	}
	ui.Success("installed pre-commit hook at %s", hookPath)
	return nil
}

func runHooksRemove(projectName string) error {
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	proj, ok := cfg.GetProject(projectName)
	if !ok {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	hookPath := filepath.Join(proj.WorkspacePath, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			ui.Warning("no pre-commit hook found")
			return nil
		}
		return fmt.Errorf("failed to read hook: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, hookMarker) {
		ui.Warning("no coderaft hook found in pre-commit file")
		return nil
	}

	
	
	lines := strings.Split(content, "\n")
	var kept []string
	skip := false
	depth := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == hookMarker {
			skip = true
			depth = 0
			continue
		}
		if skip {
			trimmed := strings.TrimSpace(line)
			
			if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "if[") {
				depth++
			}
			
			if trimmed == "fi" {
				if depth <= 1 {
					
					skip = false
					continue
				}
				depth--
			}
			continue
		}
		kept = append(kept, line)
	}

	result := strings.TrimSpace(strings.Join(kept, "\n"))
	if result == "" || result == "#!/bin/sh" {
		
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("failed to remove hook: %w", err)
		}
		ui.Success("removed pre-commit hook")
		return nil
	}

	if err := os.WriteFile(hookPath, []byte(result+"\n"), 0755); err != nil {
		return fmt.Errorf("failed to write hook: %w", err)
	}
	ui.Success("removed coderaft section from pre-commit hook")
	return nil
}

func init() {
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksRemoveCmd)
	rootCmd.AddCommand(hooksCmd)
}
