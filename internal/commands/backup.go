package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/config"
	"coderaft/internal/security"
	"coderaft/internal/ui"
)

type backupManifest struct {
	Version        int                   `json:"version"`
	Project        string                `json:"project"`
	IslandName     string                `json:"ISLAND_NAME"`
	CreatedAt      string                `json:"created_at"`
	ImageTag       string                `json:"image_tag"`
	CoderaftConfig *config.ProjectConfig `json:"coderaft_config,omitempty"`
	LockFileJSON   json.RawMessage       `json:"lock_file_json,omitempty"`
}

var (
	backupOutput string
)

var backupCmd = &cobra.Command{
	Use:   "backup <project>",
	Short: "Backup the project's coderaft island (state + config)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		proj, ok := cfg.GetProject(projectName)
		if !ok {
			return fmt.Errorf("project '%s' not found", projectName)
		}

		exists, err := dockerClient.IslandExists(proj.IslandName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("island '%s' does not exist", proj.IslandName)
		}

		ts := time.Now().UTC().Format("20060102-150405")
		defaultDir := filepath.Join(proj.WorkspacePath, ".coderaft_backups", ts)
		outDir := backupOutput
		if strings.TrimSpace(outDir) == "" {
			outDir = defaultDir
		}

		sanitizedOutDir, err := security.SanitizePath(outDir, "")
		if err != nil {
			return fmt.Errorf("invalid backup directory: %w", err)
		}
		outDir = sanitizedOutDir

		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}

		imageTag := fmt.Sprintf("coderaft/%s:backup-%s", projectName, ts)
		ui.Status("creating image from island '%s'...", proj.IslandName)
		if _, err := dockerClient.CommitContainer(proj.IslandName, imageTag); err != nil {
			return fmt.Errorf("failed to commit island: %w", err)
		}

		imageTar := filepath.Join(outDir, "image.tar")
		ui.Status("saving image '%s' to %s...", imageTag, security.SanitizePathForError(imageTar))
		if err := dockerClient.SaveImage(imageTag, imageTar); err != nil {
			return fmt.Errorf("failed to save image: %w", err)
		}

		var pcfg *config.ProjectConfig
		if c, err := configManager.LoadProjectConfig(proj.WorkspacePath); err == nil {
			pcfg = c
		}
		var lockRaw json.RawMessage
		if b, err := os.ReadFile(filepath.Join(proj.WorkspacePath, "coderaft.lock.json")); err == nil {
			lockRaw = json.RawMessage(b)
		}

		manifest := backupManifest{
			Version:        1,
			Project:        proj.Name,
			IslandName:     proj.IslandName,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			ImageTag:       imageTag,
			CoderaftConfig: pcfg,
			LockFileJSON:   lockRaw,
		}
		manPath := filepath.Join(outDir, "metadata.json")
		b, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if err := os.WriteFile(manPath, b, 0644); err != nil {
			return fmt.Errorf("failed to write metadata: %w", err)
		}

		ui.Success("backup complete")
		ui.Detail("directory", outDir)
		ui.Detail("image tag", imageTag)
		ui.Detail("files", "image.tar, metadata.json")
		return nil
	},
}

func init() {
	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "Output directory for backup (default: <workspace>/.coderaft_backups/<timestamp>)")
}
