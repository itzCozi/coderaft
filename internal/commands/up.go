package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"coderaft/internal/config"
	"coderaft/internal/ui"
)

var (
	upDotfilesPath string
)

var keepRunningUpFlag bool

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start a coderaft environment from the current folder's coderaft.json",
	Long:  "Reads coderaft.json in the current directory and boots the environment so new teammates can simply run 'coderaft up'.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		projectConfig, err := configManager.LoadProjectConfig(cwd)
		if err != nil {
			return fmt.Errorf("failed to load coderaft.json: %w", err)
		}
		if projectConfig == nil {
			return fmt.Errorf("no coderaft.json found in %s", cwd)
		}

		if err := configManager.ValidateProjectConfig(projectConfig); err != nil {
			return fmt.Errorf("invalid coderaft.json: %w", err)
		}

		projectName := projectConfig.Name
		if projectName == "" {

			projectName = filepath.Base(cwd)
		}

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load global config: %w", err)
		}

		boxName := fmt.Sprintf("coderaft_%s", projectName)
		baseImage := cfg.GetEffectiveBaseImage(&config.Project{Name: projectName, BaseImage: projectConfig.BaseImage}, projectConfig)

		workspaceBox := "/workspace"
		if projectConfig.WorkingDir != "" {
			workspaceBox = projectConfig.WorkingDir
		}

		exists, err := dockerClient.BoxExists(boxName)
		if err != nil {
			return fmt.Errorf("failed to check box existence: %w", err)
		}

		if exists {
			status, err := dockerClient.GetBoxStatus(boxName)
			if err != nil {
				return fmt.Errorf("failed to get box status: %w", err)
			}
			if status != "running" {
				if err := dockerClient.StartBox(boxName); err != nil {
					return fmt.Errorf("failed to start existing box: %w", err)
				}
			}

			if !dockerClient.IsBoxInitialized(boxName) {
				if err := dockerClient.SetupCoderaftInBox(boxName, projectName); err != nil {
					return fmt.Errorf("failed to setup coderaft in existing box: %w", err)
				}
			}
			ui.Success("environment is up")
			ui.Detail("workspace", cwd)
			ui.Detail("box", boxName)
			ui.Detail("image", baseImage)
			ui.Info("hint: run 'coderaft shell %s' to enter the environment.", projectName)

			if cfg.Settings != nil && cfg.Settings.AutoStopOnExit && !keepRunningUpFlag {
				if idle, err := dockerClient.IsContainerIdle(boxName); err == nil && idle {
					ui.Status("stopping box '%s' (auto-stop: idle)...", boxName)
					if err := dockerClient.StopBox(boxName); err != nil {
						ui.Warning("failed to stop box: %v", err)
					}
				}
			}
			return nil
		}

		ui.Status("setting up box '%s' with image '%s'...", boxName, baseImage)
		if err := dockerClient.PullImage(baseImage); err != nil {
			return fmt.Errorf("failed to pull base image: %w", err)
		}

		var configMap map[string]interface{}
		if projectConfig != nil {
			data, _ := json.Marshal(projectConfig)
			_ = json.Unmarshal(data, &configMap)
		}

		if cfg.Settings != nil && cfg.Settings.AutoStopOnExit {
			if configMap == nil {
				configMap = map[string]interface{}{}
			}
			if _, ok := configMap["restart"]; !ok {
				configMap["restart"] = "no"
			}
		}

		var dotfiles []string
		if len(projectConfig.Dotfiles) > 0 {
			dotfiles = append(dotfiles, projectConfig.Dotfiles...)
		}
		if upDotfilesPath != "" {
			dotfiles = append(dotfiles, upDotfilesPath)
		}
		if len(dotfiles) > 0 {
			arr := make([]interface{}, 0, len(dotfiles))
			for _, s := range dotfiles {
				arr = append(arr, s)
			}
			if configMap == nil {
				configMap = map[string]interface{}{}
			}
			configMap["dotfiles"] = arr
		}

		optimizedSetup := NewOptimizedSetup(dockerClient, configManager)
		if err := optimizedSetup.FastUp(projectConfig, projectName, boxName, baseImage, cwd, workspaceBox); err != nil {
			return fmt.Errorf("failed to start environment: %w", err)
		}

		ui.Success("environment is up")
		ui.Detail("workspace", cwd)
		ui.Detail("box", boxName)
		ui.Detail("image", baseImage)
		ui.Info("hint: run 'coderaft shell %s' to enter the environment.", projectName)

		if cfg.Settings != nil && cfg.Settings.AutoStopOnExit && !keepRunningUpFlag {
			if idle, err := dockerClient.IsContainerIdle(boxName); err == nil && idle {
				ui.Status("stopping box '%s' (auto-stop: idle)...", boxName)
				if err := dockerClient.StopBox(boxName); err != nil {
					ui.Warning("failed to stop box: %v", err)
				}
			}
		}
		return nil
	},
}

func init() {
	upCmd.Flags().StringVar(&upDotfilesPath, "dotfiles", "", "Path to local dotfiles directory to mount into the box")
	upCmd.Flags().BoolVar(&keepRunningUpFlag, "keep-running", false, "Keep the box running after 'up' finishes")
}
