package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var (
	verboseFlag bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all coderaft projects and their status",
	Long:  `Display all managed coderaft projects along with their island status.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		projects := cfg.GetProjects()
		if len(projects) == 0 {
			ui.Info("no coderaft projects found.")
			ui.Info("create a new project with: coderaft init <project-name>")
			return nil
		}

		islands, err := dockerClient.ListIslands()
		if err != nil {
			return fmt.Errorf("failed to list islands: %w", err)
		}

		islandStatus := make(map[string]string)
		for _, island := range islands {
			for _, name := range island.Names {

				cleanName := strings.TrimPrefix(name, "/")
				islandStatus[cleanName] = island.Status
			}
		}

		ui.Header("coderaft projects")
		if verboseFlag {
			fmt.Printf("%-20s %-20s %-15s %-12s %s\n", "PROJECT", "island", "STATUS", "CONFIG", "WORKSPACE")
			fmt.Printf("%-20s %-20s %-15s %-12s %s\n",
				strings.Repeat("-", 20),
				strings.Repeat("-", 20),
				strings.Repeat("-", 15),
				strings.Repeat("-", 12),
				strings.Repeat("-", 30))
		} else {
			fmt.Printf("%-20s %-20s %-15s %s\n", "PROJECT", "island", "STATUS", "WORKSPACE")
			fmt.Printf("%-20s %-20s %-15s %s\n",
				strings.Repeat("-", 20),
				strings.Repeat("-", 20),
				strings.Repeat("-", 15),
				strings.Repeat("-", 30))
		}

		for _, project := range projects {
			status := "not found"
			if islandStatus[project.IslandName] != "" {
				status = islandStatus[project.IslandName]
			}

			configStatus := "none"
			if project.ConfigFile != "" {
				configStatus = "coderaft.json"
			} else {

				projectConfig, err := configManager.LoadProjectConfig(project.WorkspacePath)
				if err == nil && projectConfig != nil {
					configStatus = "coderaft.json"
				}
			}

			if verboseFlag {
				fmt.Printf("%-20s %-20s %-15s %-12s %s\n",
					project.Name,
					project.IslandName,
					status,
					configStatus,
					project.WorkspacePath)
			} else {
				fmt.Printf("%-20s %-20s %-15s %s\n",
					project.Name,
					project.IslandName,
					status,
					project.WorkspacePath)
			}

			if verboseFlag {
				projectConfig, err := configManager.LoadProjectConfig(project.WorkspacePath)
				if err == nil && projectConfig != nil {
					if projectConfig.BaseImage != "" && projectConfig.BaseImage != project.BaseImage {
						ui.Item("base image: %s (override)", projectConfig.BaseImage)
					}
					if len(projectConfig.Ports) > 0 {
						ui.Item("ports: %s", strings.Join(projectConfig.Ports, ", "))
					}
					if len(projectConfig.SetupCommands) > 0 {
						ui.Item("setup commands: %d", len(projectConfig.SetupCommands))
					}
				}
			}
		}

		ui.Blank()
		ui.Info("total projects: %d", len(projects))

		if verboseFlag {

			if cfg.Settings != nil {
				ui.Blank()
				ui.Header("global settings")
				ui.Detail("default base image", cfg.Settings.DefaultBaseImage)
				ui.Detail("auto update", fmt.Sprintf("%t", cfg.Settings.AutoUpdate))
			}
		} else {
			ui.Blank()
			ui.Info("use --verbose for detailed information including configurations.")
		}

		return nil
	},
}

func init() {
	listCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Show detailed information including configuration details")
}
