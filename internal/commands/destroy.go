package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var destroyForce bool

var destroyCmd = &cobra.Command{
	Use:   "destroy <project>",
	Short: "Stop and remove a project island",
	Long: `Stop and remove the Docker island for the specified project.
Removes empty project directories automatically.

Special usage:
  coderaft destroy --cleanup-orphaned  Remove islands not tracked in config`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		if projectName == "--cleanup-orphaned" {
			return cleanupOrphanedislands()
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

		if !destroyForce {
			ui.Info("this will destroy the island '%s' for project '%s'.", project.IslandName, projectName)
			ui.Info("empty project directories will be automatically removed.")
			ui.Prompt("Are you sure? (y/N): ")

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

		exists, err = dockerClient.IslandExists(project.IslandName)
		if err != nil {
			return fmt.Errorf("failed to check island status: %w", err)
		}

		if exists {

			ui.Status("stopping and removing island '%s'...", project.IslandName)
			if err := dockerClient.RemoveIsland(project.IslandName); err != nil {
				ui.Warning("failed to remove island: %v", err)

			}
		} else {
			ui.Info("island '%s' not found (already removed)", project.IslandName)
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
				if runtime.GOOS == "windows" {
					ui.Info("  rmdir /s /q \"%s\"", project.WorkspacePath)
				} else {
					ui.Info("  rm -rf %s", project.WorkspacePath)
				}
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
	if err != nil {
		return false, fmt.Errorf("failed to read directory names: %w", err)
	}
	return false, nil
}

func cleanupOrphanedislands() error {
	ui.Status("cleaning up orphaned coderaft islands...")

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	islands, err := dockerClient.ListIslands()
	if err != nil {
		return fmt.Errorf("failed to list islands: %w", err)
	}

	trackedislands := make(map[string]bool)
	for _, project := range cfg.GetProjects() {
		trackedislands[project.IslandName] = true
	}

	var orphanedislands []string
	for _, island := range islands {
		for _, name := range island.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if !trackedislands[cleanName] {
				orphanedislands = append(orphanedislands, cleanName)
			}
		}
	}

	if len(orphanedislands) == 0 {
		ui.Info("no orphaned islands found.")
		return nil
	}

	ui.Info("found %d orphaned coderaft island(s):", len(orphanedislands))
	for _, IslandName := range orphanedislands {
		ui.Item(IslandName)
	}

	if !destroyForce {
		ui.Blank()
		ui.Prompt("Remove these orphaned islands? (y/N): ")
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
	for _, IslandName := range orphanedislands {
		ui.Status("removing %s...", IslandName)
		if err := dockerClient.RemoveIsland(IslandName); err != nil {
			ui.Error("failed to remove %s: %v", IslandName, err)
			failed++
		} else {
			ui.Info("removed %s", IslandName)
			removed++
		}
	}

	ui.Blank()
	ui.Summary("cleanup: %d removed, %d failed", removed, failed)
	if failed > 0 {
		return fmt.Errorf("failed to remove %d island(s)", failed)
	}

	return nil
}
