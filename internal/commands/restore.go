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

var restoreCmd = &cobra.Command{
	Use:   "restore <project> <backup-dir>",
	Short: "Restore a project's coderaft environment from a backup directory",
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
		_ = json.Unmarshal(metaBytes, &manifest)

		ui.Status("loading image from %s...", imageTar)
		imgID, err := dockerClient.LoadImage(imageTar)
		if err != nil {
			return fmt.Errorf("failed to load image: %w", err)
		}

		imageRef := ""
		if v, ok := manifest["ImageTag"].(string); ok && v != "" {
			imageRef = v
		}
		if imageRef == "" {
			imageRef = imgID
		}

		exists, err := dockerClient.BoxExists(proj.BoxName)
		if err == nil && exists {
			if !forceFlag {
				return fmt.Errorf("box '%s' already exists. Use --force to overwrite", proj.BoxName)
			}
			_ = dockerClient.StopBox(proj.BoxName)
			if err := dockerClient.RemoveBox(proj.BoxName); err != nil {
				return fmt.Errorf("failed to remove existing box: %w", err)
			}
		}

		workspaceBox := "/workspace"
		if pcfg, err := configManager.LoadProjectConfig(proj.WorkspacePath); err == nil && pcfg != nil && strings.TrimSpace(pcfg.WorkingDir) != "" {
			workspaceBox = pcfg.WorkingDir
		}

		boxID, err := dockerClient.CreateBoxWithConfig(proj.BoxName, imageRef, proj.WorkspacePath, workspaceBox, nil)
		if err != nil {
			return fmt.Errorf("failed to create box from image: %w", err)
		}
		if err := dockerClient.StartBox(boxID); err != nil {
			return fmt.Errorf("failed to start restored box: %w", err)
		}

		ui.Success("restore complete, box '%s' recreated from backup.", proj.BoxName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Overwrite existing box if present")
}
