package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"devbox/internal/ui"
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
	Short: "Clean up Docker resources and devbox artifacts",
	Long: `Clean up various Docker resources and devbox-related artifacts.
This command helps maintain a clean system by removing:

- Orphaned devbox boxes (not tracked in config)
- Unused Docker images
- Unused Docker volumes
- Unused Docker networks
- Dangling build artifacts

Examples:
  devbox cleanup                    # Interactive cleanup menu
  devbox cleanup --orphaned         # Remove orphaned boxes only
  devbox cleanup --images           # Remove unused images only
  devbox cleanup --all              # Clean up everything
  devbox cleanup --system-prune     # Run docker system prune
  devbox cleanup --dry-run          # Show what would be cleaned`,
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
	ui.Header("Devbox Cleanup")
	ui.Blank()
	ui.Info("Available options:")
	ui.Info("  1. Clean up orphaned boxes")
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
	ui.Status("scanning for orphaned boxes...")

	if dryRunFlag {
		ui.Info("dry run - no boxes will be removed")
	}

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	boxes, err := dockerClient.ListBoxes()
	if err != nil {
		return fmt.Errorf("failed to list boxes: %w", err)
	}

	trackedboxes := make(map[string]bool)
	for _, project := range cfg.GetProjects() {
		trackedboxes[project.BoxName] = true
	}

	var orphanedboxes []string
	for _, box := range boxes {
		for _, name := range box.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, "devbox_") && !trackedboxes[cleanName] {
				orphanedboxes = append(orphanedboxes, cleanName)
			}
		}
	}

	if len(orphanedboxes) == 0 {
		ui.Info("no orphaned boxes found.")
		return nil
	}

	ui.Info("found %d orphaned box(s):", len(orphanedboxes))
	for _, boxName := range orphanedboxes {
		ui.Item("%s", boxName)
	}

	if dryRunFlag {
		ui.Blank()
		ui.Info("dry run: would remove %d orphaned boxes", len(orphanedboxes))
		return nil
	}

	if !forceFlag {
		ui.Prompt("\nRemove these orphaned boxes? (y/N): ")
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
	for _, boxName := range orphanedboxes {
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
	ui.Summary("%d removed, %d failed", removed, failed)
	if failed > 0 {
		return fmt.Errorf("failed to remove %d box(s)", failed)
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
	ui.Info("Devbox Boxes:")
	boxes, err := dockerClient.ListBoxes()
	if err != nil {
		ui.Error("failed to list boxes: %v", err)
	} else {
		ui.Info("active boxes: %d", len(boxes))
		for _, box := range boxes {
			for _, name := range box.Names {
				ui.Item("%s (%s)", strings.TrimPrefix(name, "/"), box.Status)
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
			ui.Item("%s -> %s", name, project.BoxName)
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
	cleanupCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Clean up all unused resources (boxes, images, volumes, networks)")
	cleanupCmd.Flags().BoolVar(&orphanedFlag, "orphaned", false, "Clean up orphaned devbox boxes only")
	cleanupCmd.Flags().BoolVar(&imagesFlag, "images", false, "Clean up unused Docker images only")
	cleanupCmd.Flags().BoolVar(&volumesFlag, "volumes", false, "Clean up unused Docker volumes only")
	cleanupCmd.Flags().BoolVar(&networksFlag, "networks", false, "Clean up unused Docker networks only")
	cleanupCmd.Flags().BoolVar(&systemPruneFlag, "system-prune", false, "Run Docker system prune for comprehensive cleanup")
	cleanupCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force cleanup without confirmation prompts")
}
