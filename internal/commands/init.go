package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/config"
	"coderaft/internal/ui"
)

var (
	templateFlag   string
	generateConfig bool
	configOnlyFlag bool
)

var initCmd = &cobra.Command{
	Use:   "init <project>",
	Short: "Initialize a new coderaft project",
	Long: `Create a new coderaft project with its own Docker box.
This will create a project directory and a corresponding Docker box.

Examples:
  coderaft init myproject                    # Basic project
  coderaft init myproject --template python # Python development project
  coderaft init myproject --config-only     # Generate coderaft.json only
  coderaft init myproject --generate-config # Create box and generate coderaft.json`,
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
			ui.Info("found existing coderaft.json configuration")
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
			ui.Info("generated coderaft.json configuration file")
		}

		if configOnlyFlag {
			ui.Success("configuration generated for '%s'", projectName)
			ui.Detail("workspace", workspacePath)
			ui.Detail("config", workspacePath+"/coderaft.json")
			return nil
		}

		if projectConfig != nil {
			if err := configManager.ValidateProjectConfig(projectConfig); err != nil {
				return fmt.Errorf("invalid project configuration: %w", err)
			}
		}

		boxName := fmt.Sprintf("coderaft_%s", projectName)

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

		ui.Status("setting up coderaft commands in box...")
		if err := dockerClient.SetupCoderaftInBoxWithUpdate(boxName, projectName); err != nil {
			return fmt.Errorf("failed to setup coderaft in box: %w", err)
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
			ui.Detail("config", "coderaft.json")
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
		ui.Info("  coderaft shell %s       # open interactive shell", projectName)
		ui.Info("  coderaft run %s <cmd>   # run a command", projectName)
		if projectConfig == nil && !generateConfig {
			ui.Info("  coderaft config %s      # generate coderaft.json config", projectName)
		}

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force initialization, overwriting existing project")
	initCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Initialize from template (python, nodejs, go, web)")
	initCmd.Flags().BoolVarP(&generateConfig, "generate-config", "g", false, "Generate coderaft.json configuration file")
	initCmd.Flags().BoolVarP(&configOnlyFlag, "config-only", "c", false, "Generate configuration file only (don't create box)")
}
