package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var restoreForce bool

var restoreCmd = &cobra.Command{
	Use:   "restore <project> <backup-dir>",
	Short: "Restore a project's coderaft island from a backup directory",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		backupDir := args[1]

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		proj, ok := cfg.GetProject(projectName)
		if !ok {
			return fmt.Errorf("project '%s' not found", projectName)
		}

		imageTar := filepath.Join(backupDir, "image.tar")
		metaPath := filepath.Join(backupDir, "metadata.json")
		if _, err := os.Stat(imageTar); err != nil {
			return fmt.Errorf("missing image tar at %s", imageTar)
		}
		metaBytes, err := os.ReadFile(metaPath)
		if err != nil {
			return fmt.Errorf("failed to read metadata: %w", err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(metaBytes, &manifest); err != nil {
			return fmt.Errorf("failed to parse metadata: %w", err)
		}

		ui.Status("loading image from %s...", imageTar)
		imgID, err := dockerClient.LoadImage(imageTar)
		if err != nil {
			return fmt.Errorf("failed to load image: %w", err)
		}

		imageRef := ""
		if v, ok := manifest["image_tag"].(string); ok && v != "" {
			imageRef = v
		}
		if imageRef == "" {
			imageRef = imgID
		}

		exists, err := dockerClient.IslandExists(proj.IslandName)
		if err == nil && exists {
			if !restoreForce {
				return fmt.Errorf("island '%s' already exists. Use --force to overwrite", proj.IslandName)
			}
			_ = dockerClient.StopIsland(proj.IslandName)
			if err := dockerClient.RemoveIsland(proj.IslandName); err != nil {
				return fmt.Errorf("failed to remove existing island: %w", err)
			}
		}

		workspaceIsland := "/island"
		if pcfg, err := configManager.LoadProjectConfig(proj.WorkspacePath); err == nil && pcfg != nil && strings.TrimSpace(pcfg.WorkingDir) != "" {
			workspaceIsland = pcfg.WorkingDir
		}

		islandID, err := dockerClient.CreateIslandWithConfig(proj.IslandName, imageRef, proj.WorkspacePath, workspaceIsland, nil)
		if err != nil {
			return fmt.Errorf("failed to create island from image: %w", err)
		}
		if err := dockerClient.StartIsland(islandID); err != nil {
			return fmt.Errorf("failed to start restored island: %w", err)
		}

		ui.Success("restore complete, island '%s' recreated from backup.", proj.IslandName)
		return nil
	},
}

func init() {
	restoreCmd.Flags().BoolVarP(&restoreForce, "force", "f", false, "Overwrite existing island if present")
}
