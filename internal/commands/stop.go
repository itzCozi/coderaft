package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var stopCmd = &cobra.Command{
	Use:   "stop <project>",
	Short: "Stop a project's box",
	Long:  `Stop the Docker box for the specified project if it's running.`,
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

		exists, err = dockerClient.BoxExists(project.BoxName)
		if err != nil {
			return fmt.Errorf("failed to check box status: %w", err)
		}

		if !exists {
			ui.Info("box '%s' not found, nothing to stop.", project.BoxName)
			return nil
		}

		status, err := dockerClient.GetBoxStatus(project.BoxName)
		if err != nil {
			return fmt.Errorf("failed to get box status: %w", err)
		}

		if status != "running" {
			ui.Info("box '%s' is not running.", project.BoxName)
			return nil
		}

		ui.Status("stopping box '%s'...", project.BoxName)
		if err := dockerClient.StopBox(project.BoxName); err != nil {
			return fmt.Errorf("failed to stop box: %w", err)
		}

		ui.Success("stopped '%s'", project.BoxName)
		return nil
	},
}
