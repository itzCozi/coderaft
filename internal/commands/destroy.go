package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy <project>",
	Short: "Stop and remove a project box",
	Long: `Stop and remove the Docker box for the specified project.
Removes empty project directories automatically.

Special usage:
  coderaft destroy --cleanup-orphaned  Remove boxes not tracked in config`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		if projectName == "--cleanup-orphaned" {
			return cleanupOrphanedboxes()
		}

		if err := validateProjectName(projectName); err != nil {
			return err
		}

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		project, exists := cfg.GetProject(projectName)
		if !exists {
			return fmt.Errorf("project '%s' not found", projectName)
		}

		if !forceFlag {
			ui.Info("this will destroy the box '%s' for project '%s'.", project.BoxName, projectName)
			ui.Info("empty project directories will be automatically removed.")
			ui.Prompt("are you sure? (y/N): ")

			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				ui.Info("destruction cancelled.")
				return nil
			}
		}

		exists, err = dockerClient.BoxExists(project.BoxName)
		if err != nil {
			return fmt.Errorf("failed to check box status: %w", err)
		}

		if exists {

			ui.Status("stopping and removing box '%s'...", project.BoxName)
			if err := dockerClient.RemoveBox(project.BoxName); err != nil {
				ui.Warning("failed to remove box: %v", err)

			}
		} else {
			ui.Info("box '%s' not found (already removed)", project.BoxName)
		}

		cfg.RemoveProject(projectName)
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		ui.Success("project '%s' destroyed", projectName)

		if _, err := os.Stat(project.WorkspacePath); err == nil {

			isEmpty, err := isDirEmpty(project.WorkspacePath)
			if err != nil {
				ui.Warning("failed to check if directory is empty: %v", err)
				ui.Detail("files", project.WorkspacePath)
			} else if isEmpty {
				ui.Status("removing empty project directory: %s", project.WorkspacePath)
				if err := os.RemoveAll(project.WorkspacePath); err != nil {
					ui.Warning("failed to remove empty directory: %v", err)
				} else {
					ui.Info("empty project directory removed.")
				}
			} else {
				ui.Detail("files preserved", project.WorkspacePath)
				ui.Blank()
				ui.Info("to completely remove the project files:")
				ui.Info("  rm -rf %s", project.WorkspacePath)
			}
		}

		return nil
	},
}

func isDirEmpty(dirPath string) (bool, error) {
	f, err := os.Open(dirPath)
	if err != nil {
		return false, fmt.Errorf("failed to open directory: %w", err)
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, fmt.Errorf("failed to read directory names: %w", err)
}

func cleanupOrphanedboxes() error {
	ui.Status("cleaning up orphaned coderaft boxes...")

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	boxes, err := dockerClient.ListBoxes()
	if err != nil {
		return fmt.Errorf("failed to list boxes: %w", err)
	}

	trackedBoxes := make(map[string]bool)
	for _, project := range cfg.GetProjects() {
		trackedBoxes[project.BoxName] = true
	}

	var orphanedBoxes []string
	for _, box := range boxes {
		for _, name := range box.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if !trackedBoxes[cleanName] {
				orphanedBoxes = append(orphanedBoxes, cleanName)
			}
		}
	}

	if len(orphanedBoxes) == 0 {
		ui.Info("no orphaned boxes found.")
		return nil
	}

	ui.Info("found %d orphaned coderaft box(s):", len(orphanedBoxes))
	for _, boxName := range orphanedBoxes {
		ui.Item(boxName)
	}

	if !forceFlag {
		ui.Blank()
		ui.Prompt("remove these orphaned boxes? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			ui.Info("cleanup cancelled.")
			return nil
		}
	}

	var removed, failed int
	for _, boxName := range orphanedBoxes {
		ui.Status("removing %s...", boxName)
		if err := dockerClient.RemoveBox(boxName); err != nil {
			ui.Error("failed to remove %s: %v", boxName, err)
			failed++
		} else {
			ui.Info("removed %s", boxName)
			removed++
		}
	}

	ui.Blank()
	ui.Summary("cleanup: %d removed, %d failed", removed, failed)
	if failed > 0 {
		return fmt.Errorf("failed to remove %d box(s)", failed)
	}

	return nil
}
