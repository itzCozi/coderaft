package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"devbox/internal/config"
	"devbox/internal/ui"
)

var (
	templateFlag   string
	generateConfig bool
	configOnlyFlag bool
)

var initCmd = &cobra.Command{
	Use:   "init <project>",
	Short: "Initialize a new devbox project",
	Long: `Create a new devbox project with its own Docker box.
This will create a project directory and a corresponding Docker box.

Examples:
  devbox init myproject                    # Basic project
  devbox init myproject --template python # Python development project
  devbox init myproject --config-only     # Generate devbox.json only
  devbox init myproject --generate-config # Create box and generate devbox.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		if err := validateProjectName(projectName); err != nil {
			return err
		}

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if _, exists := cfg.GetProject(projectName); exists && !forceFlag {
			return fmt.Errorf("project '%s' already exists. Use --force to overwrite", projectName)
		}

		workspacePath, err := getWorkspacePath(projectName)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(workspacePath, 0755); err != nil {
			return fmt.Errorf("failed to create workspace directory: %w", err)
		}

		ui.Status("created workspace directory: %s", workspacePath)

		var projectConfig *config.ProjectConfig

		if existingConfig, err := configManager.LoadProjectConfig(workspacePath); err == nil && existingConfig != nil {
			ui.Info("found existing devbox.json configuration")
			projectConfig = existingConfig
		} else if templateFlag != "" {

			ui.Status("creating project from template: %s", templateFlag)
			projectConfig, err = configManager.CreateProjectConfigFromTemplate(templateFlag, projectName)
			if err != nil {
				return fmt.Errorf("failed to create project from template: %w", err)
			}
		} else if generateConfig {

			projectConfig = configManager.GetDefaultProjectConfig(projectName)
		}

		if projectConfig != nil && (generateConfig || templateFlag != "") {
			if err := configManager.SaveProjectConfig(workspacePath, projectConfig); err != nil {
				return fmt.Errorf("failed to save project configuration: %w", err)
			}
			ui.Info("generated devbox.json configuration file")
		}

		if configOnlyFlag {
			ui.Success("configuration generated for '%s'", projectName)
			ui.Detail("workspace", workspacePath)
			ui.Detail("config", workspacePath+"/devbox.json")
			return nil
		}

		if projectConfig != nil {
			if err := configManager.ValidateProjectConfig(projectConfig); err != nil {
				return fmt.Errorf("invalid project configuration: %w", err)
			}
		}

		boxName := fmt.Sprintf("devbox_%s", projectName)

		baseImage := cfg.GetEffectiveBaseImage(&config.Project{
			Name:      projectName,
			BaseImage: "ubuntu:22.04",
		}, projectConfig)

		workspaceBox := "/workspace"
		if projectConfig != nil && projectConfig.WorkingDir != "" {
			workspaceBox = projectConfig.WorkingDir
		}

		ui.Status("setting up box '%s' with image '%s'...", boxName, baseImage)
		if err := dockerClient.PullImage(baseImage); err != nil {
			return fmt.Errorf("failed to pull base image: %w", err)
		}

		if forceFlag {
			exists, err := dockerClient.BoxExists(boxName)
			if err != nil {
				return fmt.Errorf("failed to check box existence: %w", err)
			}
			if exists {
				ui.Status("removing existing box '%s'...", boxName)
				dockerClient.StopBox(boxName)
				if err := dockerClient.RemoveBox(boxName); err != nil {
					return fmt.Errorf("failed to remove existing box: %w", err)
				}
			}
		}

		var configMap map[string]interface{}
		if projectConfig != nil {
			configData, _ := json.Marshal(projectConfig)
			json.Unmarshal(configData, &configMap)
		}

		if cfg.Settings != nil && cfg.Settings.AutoStopOnExit {
			if configMap == nil {
				configMap = map[string]interface{}{}
			}
			if _, ok := configMap["restart"]; !ok {
				configMap["restart"] = "no"
			}
		}

		boxID, err := dockerClient.CreateBoxWithConfig(boxName, baseImage, workspacePath, workspaceBox, configMap)
		if err != nil {
			return fmt.Errorf("failed to create box: %w", err)
		}

		if err := dockerClient.StartBox(boxID); err != nil {
			return fmt.Errorf("failed to start box: %w", err)
		}

		ui.Status("starting box...")
		if err := dockerClient.WaitForBox(boxName, 30*time.Second); err != nil {
			return fmt.Errorf("box failed to start: %w", err)
		}

		// Skip the heavy apt update + full-upgrade when there are no setup commands.
		// apt full-upgrade alone takes 10-30s and invalidates Docker layer cache
		// on every security patch. The base image is already up to date enough.
		if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
			ui.Status("updating system packages...")
			systemUpdateCommands := []string{
				"apt update -y",
			}
			if err := dockerClient.ExecuteSetupCommandsWithOutput(boxName, systemUpdateCommands, false); err != nil {
				ui.Warning("apt update failed: %v", err)
			}

			ui.Status("installing template packages (%d commands)...", len(projectConfig.SetupCommands))
			if err := dockerClient.ExecuteSetupCommandsWithOutput(boxName, projectConfig.SetupCommands, false); err != nil {
				return fmt.Errorf("failed to execute setup commands: %w", err)
			}
		}

		ui.Status("setting up devbox commands in box...")
		if err := dockerClient.SetupDevboxInBoxWithUpdate(boxName, projectName); err != nil {
			return fmt.Errorf("failed to setup devbox in box: %w", err)
		}

		project := &config.Project{
			Name:          projectName,
			BoxName:       boxName,
			BaseImage:     baseImage,
			WorkspacePath: workspacePath,
			Status:        "running",
		}

		cfg.MergeProjectConfig(project, projectConfig)

		cfg.AddProject(project)
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		ui.Blank()
		ui.Success("project '%s' initialized", projectName)
		ui.Detail("workspace", workspacePath)
		ui.Detail("box", boxName)
		ui.Detail("image", baseImage)

		if projectConfig != nil {
			ui.Detail("config", "devbox.json")
			if len(projectConfig.SetupCommands) > 0 {
				ui.Detail("setup commands", fmt.Sprintf("%d executed", len(projectConfig.SetupCommands)))
			}
			if len(projectConfig.Ports) > 0 {
				ui.Detail("ports", fmt.Sprintf("%v", projectConfig.Ports))
			}
		}

		if cfg.Settings != nil && cfg.Settings.AutoStopOnExit {
			if idle, err := dockerClient.IsContainerIdle(boxName); err == nil && idle {
				ui.Status("stopping box '%s' (auto-stop: idle)...", boxName)
				if err := dockerClient.StopBox(boxName); err != nil {
					ui.Warning("failed to stop box: %v", err)
				}
			}
		}

		ui.Blank()
		ui.Info("Next steps:")
		ui.Info("  devbox shell %s       # open interactive shell", projectName)
		ui.Info("  devbox run %s <cmd>   # run a command", projectName)
		if projectConfig == nil && !generateConfig {
			ui.Info("  devbox config %s      # generate devbox.json config", projectName)
		}

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force initialization, overwriting existing project")
	initCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Initialize from template (python, nodejs, go, web)")
	initCmd.Flags().BoolVarP(&generateConfig, "generate-config", "g", false, "Generate devbox.json configuration file")
	initCmd.Flags().BoolVarP(&configOnlyFlag, "config-only", "c", false, "Generate configuration file only (don't create box)")
}
