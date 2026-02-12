package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"devbox/internal/ui"
)

var updateCmd = &cobra.Command{
	Use:   "update [project]",
	Short: "Pull latest base image(s) and rebuild box(es)",
	Long:  "Update environments by pulling the latest base images and rebuilding the project boxes using current configuration.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			projectName := args[0]
			if err := validateProjectName(projectName); err != nil {
				return err
			}
			return updateSingleProject(projectName)
		}

		return updateAllProjects()
	},
}

func updateSingleProject(projectName string) error {
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	project, exists := cfg.GetProject(projectName)
	if !exists {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	projectConfig, _ := configManager.LoadProjectConfig(project.WorkspacePath)
	baseImage := cfg.GetEffectiveBaseImage(project, projectConfig)

	ui.Status("pulling latest image for '%s': %s", projectName, baseImage)
	if err := dockerClient.RunDockerCommand([]string{"pull", baseImage}); err != nil {
		return fmt.Errorf("failed to pull base image %s: %w", baseImage, err)
	}

	existsBox, err := dockerClient.BoxExists(project.BoxName)
	if err != nil {
		return fmt.Errorf("failed to check box existence: %w", err)
	}
	if existsBox {
		ui.Status("stopping and removing existing box '%s'...", project.BoxName)

		_ = dockerClient.StopBox(project.BoxName)
		if err := dockerClient.RemoveBox(project.BoxName); err != nil {
			return fmt.Errorf("failed to remove existing box: %w", err)
		}
	}

	workspaceBox := "/workspace"
	if projectConfig != nil && projectConfig.WorkingDir != "" {
		workspaceBox = projectConfig.WorkingDir
	}

	var configMap map[string]interface{}
	if projectConfig != nil {
		if data, err := json.Marshal(projectConfig); err == nil {
			_ = json.Unmarshal(data, &configMap)
		}
	}

	ui.Status("recreating box '%s' with image '%s'...", project.BoxName, baseImage)
	boxID, err := dockerClient.CreateBoxWithConfig(project.BoxName, baseImage, project.WorkspacePath, workspaceBox, configMap)
	if err != nil {
		return fmt.Errorf("failed to create box: %w", err)
	}

	if err := dockerClient.StartBox(boxID); err != nil {
		return fmt.Errorf("failed to start box: %w", err)
	}

	if err := dockerClient.WaitForBox(project.BoxName, 30*time.Second); err != nil {
		return fmt.Errorf("box failed to become ready: %w", err)
	}

	updateCommands := []string{
		"apt update -y",
		"apt full-upgrade -y",
	}
	if err := dockerClient.ExecuteSetupCommandsWithOutput(project.BoxName, updateCommands, false); err != nil {
		ui.Warning("failed to update system packages: %v", err)
	}

	if project.WorkspacePath != "" {
		lockfilePath := filepath.Join(project.WorkspacePath, "devbox.lock")
		if _, err := os.Stat(lockfilePath); err == nil {
			ui.Info("replaying recorded package installs from devbox.lock...")
			if data, readErr := os.ReadFile(lockfilePath); readErr == nil {
				lines := strings.Split(string(data), "\n")
				var cmds []string
				for _, line := range lines {
					cmd := strings.TrimSpace(line)
					if cmd == "" || strings.HasPrefix(cmd, "#") {
						continue
					}
					cmds = append(cmds, cmd)
				}
				if len(cmds) > 0 {
					if err := dockerClient.ExecuteSetupCommandsWithOutput(project.BoxName, cmds, false); err != nil {
						ui.Warning("failed to replay devbox.lock commands: %v", err)
					}
				}
			}
		}
	}

	if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		if err := dockerClient.ExecuteSetupCommandsWithOutput(project.BoxName, projectConfig.SetupCommands, false); err != nil {
			ui.Warning("failed to execute setup commands: %v", err)
		}
	}

	if err := dockerClient.SetupDevboxInBoxWithUpdate(project.BoxName, projectName); err != nil {
		ui.Warning("failed to setup devbox environment: %v", err)
	}

	if project.BaseImage != baseImage {
		project.BaseImage = baseImage
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save updated config: %w", err)
		}
	}

	ui.Success("'%s' updated", projectName)
	return nil
}

func updateAllProjects() error {
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	projects := cfg.GetProjects()
	if len(projects) == 0 {
		ui.Info("no projects to update.")
		return nil
	}

	var updated, failed int
	for projectName := range projects {
		if err := updateSingleProject(projectName); err != nil {
			ui.Error("failed to update %s: %v", projectName, err)
			failed++
		} else {
			updated++
		}
	}

	ui.Blank()
	ui.Summary("%d updated, %d failed", updated, failed)
	if failed > 0 {
		return fmt.Errorf("failed to update %d project(s)", failed)
	}
	return nil
}

func init() {

}
