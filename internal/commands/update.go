package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var updateLock bool

var updateCmd = &cobra.Command{
	Use:   "update [project]",
	Short: "Pull latest base image(s) and rebuild island(s)",
	Long: `Update islands by pulling the latest base images and rebuilding project islands using current configuration.

Use --lock to regenerate the lock file after the update. This is equivalent
to running 'coderaft update <project>' followed by 'coderaft lock <project>'.`,
	Args: cobra.MaximumNArgs(1),
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

	existsIsland, err := dockerClient.IslandExists(project.IslandName)
	if err != nil {
		return fmt.Errorf("failed to check island existence: %w", err)
	}
	if existsIsland {
		ui.Status("stopping and removing existing island '%s'...", project.IslandName)

		_ = dockerClient.StopIsland(project.IslandName)
		if err := dockerClient.RemoveIsland(project.IslandName); err != nil {
			return fmt.Errorf("failed to remove existing island: %w", err)
		}
	}

	workspaceIsland := "/island"
	if projectConfig != nil && projectConfig.WorkingDir != "" {
		workspaceIsland = projectConfig.WorkingDir
	}

	var configMap map[string]interface{}
	if projectConfig != nil {
		if data, err := json.Marshal(projectConfig); err == nil {
			_ = json.Unmarshal(data, &configMap)
		}
	}

	ui.Status("recreating island '%s' with image '%s'...", project.IslandName, baseImage)
	islandID, err := dockerClient.CreateIslandWithConfig(project.IslandName, baseImage, project.WorkspacePath, workspaceIsland, configMap)
	if err != nil {
		return fmt.Errorf("failed to create island: %w", err)
	}

	if err := dockerClient.StartIsland(islandID); err != nil {
		return fmt.Errorf("failed to start island: %w", err)
	}

	if err := dockerClient.WaitForIsland(project.IslandName, 30*time.Second); err != nil {
		return fmt.Errorf("island failed to become ready: %w", err)
	}

	updateCommands := []string{
		"apt update -y",
		"DEBIAN_FRONTEND=noninteractive apt full-upgrade -y",
	}
	if err := dockerClient.ExecuteSetupCommandsWithOutput(project.IslandName, updateCommands, false); err != nil {
		ui.Warning("failed to update system packages: %v", err)
	}

	if project.WorkspacePath != "" {
		lockfilePath := filepath.Join(project.WorkspacePath, "coderaft.history")
		if _, err := os.Stat(lockfilePath); err == nil {
			ui.Info("replaying recorded package installs from coderaft.history...")
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
					if err := dockerClient.ExecuteSetupCommandsWithOutput(project.IslandName, cmds, false); err != nil {
						ui.Warning("failed to replay coderaft.history commands: %v", err)
					}
				}
			}
		}
	}

	if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		if err := dockerClient.ExecuteSetupCommandsWithOutput(project.IslandName, projectConfig.SetupCommands, false); err != nil {
			ui.Warning("failed to execute setup commands: %v", err)
		}
	}

	if err := dockerClient.SetupCoderaftOnIslandWithUpdate(project.IslandName, projectName); err != nil {
		ui.Warning("failed to setup coderaft on island: %v", err)
	}

	if project.BaseImage != baseImage {
		project.BaseImage = baseImage
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save updated config: %w", err)
		}
	}

	ui.Success("'%s' updated", projectName)

	if updateLock {
		lockOut := filepath.Join(project.WorkspacePath, "coderaft.lock.json")
		ui.Status("regenerating lock file...")
		if err := WriteLockFileForProject(projectName, lockOut); err != nil {
			return fmt.Errorf("update succeeded but lock file generation failed: %w", err)
		}
		ui.Success("lock file updated at %s", lockOut)
	}

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
	updateCmd.Flags().BoolVar(&updateLock, "lock", false, "Regenerate coderaft.lock.json after the update")
}
