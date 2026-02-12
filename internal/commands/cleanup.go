package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var (
	dryRunFlag      bool
	allFlag         bool
	orphanedFlag    bool
	imagesFlag      bool
	volumesFlag     bool
	networksFlag    bool
	systemPruneFlag bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup [flags]",
	Short: "Clean up Docker resources and coderaft artifacts",
	Long: `Clean up various Docker resources and coderaft-related artifacts.
This command helps maintain a clean system by removing:

- Orphaned coderaft islands (not tracked in config)
- Unused Docker images
- Unused Docker volumes
- Unused Docker networks
- Dangling build artifacts

Examples:
  coderaft cleanup                    # Interactive cleanup menu
  coderaft cleanup --orphaned         # Remove orphaned islands only
  coderaft cleanup --images           # Remove unused images only
  coderaft cleanup --all              # Clean up everything
  coderaft cleanup --system-prune     # Run docker system prune
  coderaft cleanup --dry-run          # Show what would be cleaned`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		if !orphanedFlag && !imagesFlag && !volumesFlag && !networksFlag && !systemPruneFlag && !allFlag {
			return runInteractiveCleanup()
		}

		if allFlag {
			orphanedFlag = true
			imagesFlag = true
			volumesFlag = true
			networksFlag = true
		}

		var cleanupTasks []func() error

		if orphanedFlag {
			cleanupTasks = append(cleanupTasks, cleanupOrphanedFromCleanup)
		}

		if imagesFlag {
			cleanupTasks = append(cleanupTasks, cleanupUnusedImages)
		}

		if volumesFlag {
			cleanupTasks = append(cleanupTasks, cleanupUnusedVolumes)
		}

		if networksFlag {
			cleanupTasks = append(cleanupTasks, cleanupUnusedNetworks)
		}

		if systemPruneFlag {
			cleanupTasks = append(cleanupTasks, runSystemPrune)
		}

		for _, task := range cleanupTasks {
			if err := task(); err != nil {
				return err
			}
		}

		if len(cleanupTasks) > 0 {
			ui.Blank()
			ui.Success("cleanup completed")
		}

		return nil
	},
}

func runInteractiveCleanup() error {
	ui.Header("Coderaft Cleanup")
	ui.Blank()
	ui.Info("Available options:")
	ui.Info("  1. Clean up orphaned islands")
	ui.Info("  2. Remove unused Docker images")
	ui.Info("  3. Remove unused Docker volumes")
	ui.Info("  4. Remove unused Docker networks")
	ui.Info("  5. Docker system prune (comprehensive)")
	ui.Info("  6. Clean up everything (1-4)")
	ui.Info("  7. Show system status")
	ui.Info("  q. Quit")
	ui.Blank()

	reader := bufio.NewReader(os.Stdin)

	for {
		ui.Prompt("Select an option [1-7, q]: ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		switch response {
		case "1":
			return cleanupOrphanedFromCleanup()
		case "2":
			return cleanupUnusedImages()
		case "3":
			return cleanupUnusedVolumes()
		case "4":
			return cleanupUnusedNetworks()
		case "5":
			return runSystemPrune()
		case "6":
			ui.Blank()
			ui.Status("running comprehensive cleanup...")
			tasks := []func() error{
				cleanupOrphanedFromCleanup,
				cleanupUnusedImages,
				cleanupUnusedVolumes,
				cleanupUnusedNetworks,
			}
			for _, task := range tasks {
				if err := task(); err != nil {
					return err
				}
			}
			ui.Blank()
			ui.Success("comprehensive cleanup completed")
			return nil
		case "7":
			return showSystemStatus()
		case "q", "quit", "exit":
			ui.Info("cleanup cancelled.")
			return nil
		default:
			ui.Info("invalid option. please select 1-7 or q.")
		}
	}
}

func cleanupOrphanedFromCleanup() error {
	ui.Status("scanning for orphaned islands...")

	if dryRunFlag {
		ui.Info("dry run - no islands will be removed")
	}

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
			if strings.HasPrefix(cleanName, "coderaft_") && !trackedislands[cleanName] {
				orphanedislands = append(orphanedislands, cleanName)
			}
		}
	}

	if len(orphanedislands) == 0 {
		ui.Info("no orphaned islands found.")
		return nil
	}

	ui.Info("found %d orphaned island(s):", len(orphanedislands))
	for _, IslandName := range orphanedislands {
		ui.Item("%s", IslandName)
	}

	if dryRunFlag {
		ui.Blank()
		ui.Info("dry run: would remove %d orphaned islands", len(orphanedislands))
		return nil
	}

	if !forceFlag {
		ui.Prompt("\nRemove these orphaned islands? (y/N): ")
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
	ui.Summary("%d removed, %d failed", removed, failed)
	if failed > 0 {
		return fmt.Errorf("failed to remove %d island(s)", failed)
	}

	return nil
}

func cleanupUnusedImages() error {
	ui.Status("scanning for unused images...")

	if dryRunFlag {
		ui.Info("dry run - dangling images that would be removed:")
		if err := dockerClient.RunDockerCommand([]string{"images", "-f", "dangling=true"}); err != nil {
			return fmt.Errorf("failed to list dangling images: %w", err)
		}
	} else {
		if !forceFlag {
			ui.Prompt("Remove unused Docker images? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				ui.Info("image cleanup cancelled.")
				return nil
			}
		}

		ui.Status("removing unused images...")
		if err := dockerClient.RunDockerCommand([]string{"image", "prune", "-f"}); err != nil {
			return fmt.Errorf("failed to prune images: %w", err)
		}
		ui.Success("unused images removed")
	}

	return nil
}

func cleanupUnusedVolumes() error {
	ui.Status("scanning for unused volumes...")

	if dryRunFlag {
		ui.Info("dry run - dangling volumes that would be removed:")
		if err := dockerClient.RunDockerCommand([]string{"volume", "ls", "-f", "dangling=true"}); err != nil {
			return fmt.Errorf("failed to list dangling volumes: %w", err)
		}
	} else {
		if !forceFlag {
			ui.Prompt("Remove unused Docker volumes? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				ui.Info("volume cleanup cancelled.")
				return nil
			}
		}

		ui.Status("removing unused volumes...")
		if err := dockerClient.RunDockerCommand([]string{"volume", "prune", "-f"}); err != nil {
			return fmt.Errorf("failed to prune volumes: %w", err)
		}
		ui.Success("unused volumes removed")
	}

	return nil
}

func cleanupUnusedNetworks() error {
	ui.Status("scanning for unused networks...")

	if dryRunFlag {
		ui.Info("dry run - custom networks (unused ones would be removed):")
		if err := dockerClient.RunDockerCommand([]string{"network", "ls", "--filter", "type=custom"}); err != nil {
			return fmt.Errorf("failed to list custom networks: %w", err)
		}
	} else {
		if !forceFlag {
			ui.Prompt("Remove unused Docker networks? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				ui.Info("network cleanup cancelled.")
				return nil
			}
		}

		ui.Status("removing unused networks...")
		if err := dockerClient.RunDockerCommand([]string{"network", "prune", "-f"}); err != nil {
			return fmt.Errorf("failed to prune networks: %w", err)
		}
		ui.Success("unused networks removed")
	}

	return nil
}

func runSystemPrune() error {
	ui.Status("running comprehensive system cleanup...")

	if dryRunFlag {
		ui.Info("dry run - Docker disk usage:")
		if err := dockerClient.RunDockerCommand([]string{"system", "df"}); err != nil {
			return fmt.Errorf("failed to show Docker disk usage: %w", err)
		}
	} else {
		if !forceFlag {
			ui.Prompt("Run Docker system prune (removes all unused resources)? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				ui.Info("system prune cancelled.")
				return nil
			}
		}

		ui.Status("running system prune...")
		if err := dockerClient.RunDockerCommand([]string{"system", "prune", "-f"}); err != nil {
			return fmt.Errorf("failed to run system prune: %w", err)
		}
		ui.Success("system prune completed")
	}

	return nil
}

func showSystemStatus() error {
	ui.Header("System Status")
	ui.Blank()

	ui.Info("Disk Usage:")
	if err := dockerClient.RunDockerCommand([]string{"system", "df"}); err != nil {
		ui.Error("failed to get disk usage: %v", err)
	}

	ui.Blank()
	ui.Info("Coderaft islands:")
	islands, err := dockerClient.ListIslands()
	if err != nil {
		ui.Error("failed to list islands: %v", err)
	} else {
		ui.Info("active islands: %d", len(islands))
		for _, island := range islands {
			for _, name := range island.Names {
				ui.Item("%s (%s)", strings.TrimPrefix(name, "/"), island.Status)
			}
		}
	}

	ui.Blank()
	ui.Info("Tracked Projects:")
	cfg, err := configManager.Load()
	if err != nil {
		ui.Error("failed to load config: %v", err)
	} else {
		projects := cfg.GetProjects()
		ui.Info("tracked projects: %d", len(projects))
		for name, project := range projects {
			ui.Item("%s -> %s", name, project.IslandName)
		}
	}

	ui.Blank()
	ui.Info("Docker Version:")
	if err := dockerClient.RunDockerCommand([]string{"version", "--format", "{{.Server.Version}}"}); err != nil {
		ui.Error("failed to get Docker version: %v", err)
	}

	return nil
}

func init() {
	cleanupCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "n", false, "Show what would be cleaned without actually removing anything")
	cleanupCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Clean up all unused resources (islands, images, volumes, networks)")
	cleanupCmd.Flags().BoolVar(&orphanedFlag, "orphaned", false, "Clean up orphaned coderaft islands only")
	cleanupCmd.Flags().BoolVar(&imagesFlag, "images", false, "Clean up unused Docker images only")
	cleanupCmd.Flags().BoolVar(&volumesFlag, "volumes", false, "Clean up unused Docker volumes only")
	cleanupCmd.Flags().BoolVar(&networksFlag, "networks", false, "Clean up unused Docker networks only")
	cleanupCmd.Flags().BoolVar(&systemPruneFlag, "system-prune", false, "Run Docker system prune for comprehensive cleanup")
	cleanupCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force cleanup without confirmation prompts")
}
