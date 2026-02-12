package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var stopCmd = &cobra.Command{
	Use:   "stop <project>",
	Short: "Stop a project's island",
	Long:  `Stop the Docker island for the specified project if it's running.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		if err := validateProjectName(projectName); err != nil {
			return err
		}

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		project, exists := cfg.GetProject(projectName)
		if !exists {
			return fmt.Errorf("project '%s' not found. Run 'coderaft init %s' first", projectName, projectName)
		}

		exists, err = dockerClient.IslandExists(project.IslandName)
		if err != nil {
			return fmt.Errorf("failed to check island status: %w", err)
		}

		if !exists {
			ui.Info("island '%s' not found, nothing to stop.", project.IslandName)
			return nil
		}

		status, err := dockerClient.GetIslandStatus(project.IslandName)
		if err != nil {
			return fmt.Errorf("failed to get island status: %w", err)
		}

		if status != "running" {
			ui.Info("island '%s' is not running.", project.IslandName)
			return nil
		}

		ui.Status("stopping island '%s'...", project.IslandName)
		if err := dockerClient.StopIsland(project.IslandName); err != nil {
			return fmt.Errorf("failed to stop island: %w", err)
		}

		ui.Success("stopped '%s'", project.IslandName)
		return nil
	},
}
