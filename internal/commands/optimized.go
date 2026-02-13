package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"coderaft/internal/config"
	"coderaft/internal/docker"
	"coderaft/internal/parallel"
	"coderaft/internal/ui"
)

type OptimizedSetup struct {
	dockerClient  DockerClientInterface
	configManager *config.ConfigManager
	imageCache    *docker.ImageCache
}

type DockerClientInterface interface {
	PullImage(image string) error
	CreateIslandWithConfig(name, image, workspaceHost, workspaceIsland string, projectConfig interface{}) (string, error)
	StartIsland(islandID string) error
	WaitForIsland(IslandName string, timeout time.Duration) error
	SetupCoderaftOnIslandWithUpdate(IslandName, projectName string) error
	ExecuteSetupCommandsWithOutput(IslandName string, commands []string, showOutput bool) error
	QueryPackagesParallel(IslandName string) (aptList, pipList, npmList, yarnList, pnpmList []string)
	ImageExists(ref string) bool
	SDKExecFunc() func(ctx context.Context, containerID string, cmd []string, showOutput bool) (string, string, int, error)
}

func NewOptimizedSetup(dockerClient DockerClientInterface, configManager *config.ConfigManager) *OptimizedSetup {
	return &OptimizedSetup{
		dockerClient:  dockerClient,
		configManager: configManager,
		imageCache:    docker.NewImageCacheWithSDK(dockerClient.ImageExists),
	}
}

func (optSetup *OptimizedSetup) OptimizedSystemUpdate(IslandName string) error {
	ui.Status("performing optimized system update...")

	executor := parallel.NewSetupCommandExecutorWithSDK(IslandName, false, 2, optSetup.dockerClient.SDKExecFunc())

	groups := []parallel.CommandGroup{
		{
			Name: "System Update",
			Commands: []string{
				"apt update -y",
				"DEBIAN_FRONTEND=noninteractive apt full-upgrade -y",
			},
			Parallel: false,
		},
		{
			Name: "System Optimization",
			Commands: []string{
				"apt autoremove -y",
				"apt autoclean",
				"rm -rf /var/lib/apt/lists/*",
			},
			Parallel: true,
		},
	}

	return executor.ExecuteCommandGroups(groups)
}

func (optSetup *OptimizedSetup) FastInit(projectName string, projectConfig *config.ProjectConfig, cfg *config.Config, workspacePath string, forceFlag bool, configMap map[string]interface{}) error {
	IslandName := fmt.Sprintf("coderaft_%s", projectName)
	baseImage := cfg.GetEffectiveBaseImage(&config.Project{
		Name:      projectName,
		BaseImage: "ubuntu:latest",
	}, projectConfig)

	workspaceIsland := "/island"
	if projectConfig != nil && projectConfig.WorkingDir != "" {
		workspaceIsland = projectConfig.WorkingDir
	}

	ui.Status("fast initialization of '%s'...", IslandName)

	effectiveImage := baseImage
	if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		buildCfg := &docker.BuildImageConfig{
			BaseImage:     baseImage,
			SetupCommands: projectConfig.SetupCommands,
			Environment:   projectConfig.Environment,
			Labels:        projectConfig.Labels,
			WorkingDir:    workspaceIsland,
			Shell:         projectConfig.Shell,
			User:          projectConfig.User,
			ProjectName:   projectName,
		}

		cachedImage, err := optSetup.imageCache.BuildCachedImage(buildCfg)
		if err != nil {
			ui.Warning("cached build failed, falling back to base image: %v", err)

			if pullErr := optSetup.dockerClient.PullImage(baseImage); pullErr != nil {
				return fmt.Errorf("failed to pull base image: %w", pullErr)
			}
		} else {
			effectiveImage = cachedImage
		}
	} else {
		ui.Status("pulling image '%s'...", baseImage)
		if err := optSetup.dockerClient.PullImage(baseImage); err != nil {
			return fmt.Errorf("failed to pull base image: %w", err)
		}
	}

	if forceFlag {
		ui.Status("force flag detected, recreating island...")
	}

	ui.Status("creating island...")
	if configMap == nil {
		configMap = make(map[string]interface{})
	}

	islandID, err := optSetup.dockerClient.CreateIslandWithConfig(IslandName, effectiveImage, workspacePath, workspaceIsland, configMap)
	if err != nil {
		return fmt.Errorf("failed to create island: %w", err)
	}

	ui.Status("starting island...")
	if err := optSetup.dockerClient.StartIsland(islandID); err != nil {
		return fmt.Errorf("failed to start island: %w", err)
	}

	ui.Status("waiting for island to be ready...")
	if err := optSetup.dockerClient.WaitForIsland(IslandName, 30*time.Second); err != nil {
		return fmt.Errorf("island failed to start: %w", err)
	}

	ui.Status("setting up coderaft commands...")
	if err := optSetup.dockerClient.SetupCoderaftOnIslandWithUpdate(IslandName, projectName); err != nil {
		return fmt.Errorf("failed to setup coderaft in island: %w", err)
	}

	if effectiveImage == baseImage && projectConfig != nil && len(projectConfig.SetupCommands) > 0 {

		if err := optSetup.OptimizedSystemUpdate(IslandName); err != nil {
			ui.Warning("system update failed: %v", err)
		}

		ui.Status("installing packages (%d commands)...", len(projectConfig.SetupCommands))
		if err := optSetup.dockerClient.ExecuteSetupCommandsWithOutput(IslandName, projectConfig.SetupCommands, false); err != nil {
			return fmt.Errorf("failed to execute setup commands: %w", err)
		}

		_ = WriteLockFileForIsland(IslandName, projectName, workspacePath, baseImage, "")
	}

	return nil
}

func (optSetup *OptimizedSetup) FastUp(projectConfig *config.ProjectConfig, projectName, IslandName, baseImage, cwd, workspaceIsland string, configMap map[string]interface{}) error {
	ui.Status("fast startup of island...")

	effectiveImage := baseImage
	if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		buildCfg := &docker.BuildImageConfig{
			BaseImage:     baseImage,
			SetupCommands: projectConfig.SetupCommands,
			Environment:   projectConfig.Environment,
			Labels:        projectConfig.Labels,
			WorkingDir:    workspaceIsland,
			Shell:         projectConfig.Shell,
			User:          projectConfig.User,
			ProjectName:   projectName,
		}

		cachedImage, err := optSetup.imageCache.BuildCachedImage(buildCfg)
		if err != nil {
			ui.Warning("cached build failed, using base image: %v", err)
		} else {
			effectiveImage = cachedImage
		}
	}

	if configMap == nil {
		configMap = make(map[string]interface{})
	}

	ui.Status("creating optimized island...")
	islandID, err := optSetup.dockerClient.CreateIslandWithConfig(IslandName, effectiveImage, cwd, workspaceIsland, configMap)
	if err != nil {
		return fmt.Errorf("failed to create island: %w", err)
	}

	if err := optSetup.dockerClient.StartIsland(islandID); err != nil {
		return fmt.Errorf("failed to start island: %w", err)
	}

	ui.Status("waiting for island startup...")
	if err := optSetup.dockerClient.WaitForIsland(IslandName, 30*time.Second); err != nil {
		return fmt.Errorf("island failed to start: %w", err)
	}

	if err := optSetup.dockerClient.SetupCoderaftOnIslandWithUpdate(IslandName, projectName); err != nil {
		return fmt.Errorf("failed to setup coderaft in island: %w", err)
	}

	lockfilePath := filepath.Join(cwd, "coderaft.history")
	if _, err := os.Stat(lockfilePath); err == nil {
		ui.Status("processing history file...")
		if err := optSetup.processLockFile(IslandName, lockfilePath); err != nil {
			return fmt.Errorf("failed to process lock file: %w", err)
		}
	}

	if effectiveImage == baseImage && projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		if err := optSetup.OptimizedSystemUpdate(IslandName); err != nil {
			ui.Warning("system update failed: %v", err)
		}

		ui.Status("installing packages (%d commands)...", len(projectConfig.SetupCommands))
		if err := optSetup.dockerClient.ExecuteSetupCommandsWithOutput(IslandName, projectConfig.SetupCommands, false); err != nil {
			return fmt.Errorf("failed to execute setup commands: %w", err)
		}

		_ = WriteLockFileForIsland(IslandName, projectName, cwd, baseImage, "")
	}

	return nil
}




var allowedHistoryPrefixes = []string{
	"apt ", "apt-get ", "pip ", "pip3 ", "npm ", "yarn ", "pnpm ", "corepack ",
}

func isAllowedHistoryCommand(cmd string) bool {
	for _, prefix := range allowedHistoryPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

func (optSetup *OptimizedSetup) processLockFile(IslandName, lockfilePath string) error {
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var cmds []string
	var skipped int
	for _, line := range lines {
		cmd := strings.TrimSpace(line)
		if cmd == "" || strings.HasPrefix(cmd, "#") {
			continue
		}
		if !isAllowedHistoryCommand(cmd) {
			skipped++
			ui.Warning("skipping disallowed history command: %s", cmd)
			continue
		}
		cmds = append(cmds, cmd)
	}

	if skipped > 0 {
		ui.Warning("%d commands in coderaft.history were skipped (only package manager commands are allowed)", skipped)
	}

	if len(cmds) > 0 {
		ui.Status("replaying %d commands from history file...", len(cmds))
		return optSetup.dockerClient.ExecuteSetupCommandsWithOutput(IslandName, cmds, false)
	}

	return nil
}

func (optSetup *OptimizedSetup) PrewarmImage(image string) error {
	ui.Status("prewarming image %s...", image)
	return optSetup.dockerClient.PullImage(image)
}

func (optSetup *OptimizedSetup) OptimizeEnvironment(IslandName string) error {
	ui.Status("optimizing island...")

	executor := parallel.NewSetupCommandExecutorWithSDK(IslandName, false, 3, optSetup.dockerClient.SDKExecFunc())

	optimizationGroups := []parallel.CommandGroup{
		{
			Name: "Package Manager Optimization",
			Commands: []string{
				"apt-get clean",
				"pip cache purge || true",
				"npm cache clean --force || true",
			},
			Parallel: true,
		},
		{
			Name: "System Optimization",
			Commands: []string{
				"updatedb || true",
				"ldconfig",
			},
			Parallel: true,
		},
	}

	return executor.ExecuteCommandGroups(optimizationGroups)
}
