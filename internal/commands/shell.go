package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"coderaft/internal/docker"
	"coderaft/internal/ui"
)

var keepRunningFlag bool

var shellCmd = &cobra.Command{
	Use:   "shell <project>",
	Short: "Open an interactive shell in the project island",
	Long:  `Attach an interactive bash shell to the specified project's island.`,
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
			return fmt.Errorf("island '%s' not found. Run 'coderaft init %s' to recreate", project.IslandName, projectName)
		}

		status, err := dockerClient.GetIslandStatus(project.IslandName)
		if err != nil {
			return fmt.Errorf("failed to get island status: %w", err)
		}

		if status != "running" {
			ui.Status("starting island '%s'...", project.IslandName)
			if err := dockerClient.StartIsland(project.IslandName); err != nil {
				return fmt.Errorf("failed to start island: %w", err)
			}
		}

		if !dockerClient.IsIslandInitialized(project.IslandName) {
			ui.Status("setting up coderaft commands in island...")
			if err := dockerClient.SetupCoderaftOnIsland(project.IslandName, projectName); err != nil {
				return fmt.Errorf("failed to setup coderaft in island: %w", err)
			}
		}

		ui.Status("attaching to island '%s'...", project.IslandName)
		if err := docker.AttachShell(project.IslandName); err != nil {
			return fmt.Errorf("failed to attach shell: %w", err)
		}

		if !keepRunningFlag {
			cfg, err := configManager.Load()
			if err == nil && cfg.Settings != nil && cfg.Settings.AutoStopOnExit {
				idle, idleErr := dockerClient.IsContainerIdle(project.IslandName)
				if idleErr != nil {
					ui.Warning("failed to check island idle status: %v", idleErr)
				} else if idle {
					ui.Status("stopping island '%s' (auto-stop: idle)...", project.IslandName)
					if err := dockerClient.StopIsland(project.IslandName); err != nil {
						ui.Warning("failed to stop island: %v", err)
					}
				}
			}
		}

		return nil
	},
}

func init() {
	shellCmd.Flags().BoolVar(&keepRunningFlag, "keep-running", false, "Keep the island running after exiting the shell")
}
