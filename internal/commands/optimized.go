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
	CreateBoxWithConfig(name, image, workspaceHost, workspaceBox string, projectConfig interface{}) (string, error)
	StartBox(boxID string) error
	WaitForBox(boxName string, timeout time.Duration) error
	SetupCoderaftInBoxWithUpdate(boxName, projectName string) error
	ExecuteSetupCommandsWithOutput(boxName string, commands []string, showOutput bool) error
	QueryPackagesParallel(boxName string) (aptList, pipList, npmList, yarnList, pnpmList []string)
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

func (optSetup *OptimizedSetup) OptimizedSystemUpdate(boxName string) error {
	ui.Status("performing optimized system update...")

	executor := parallel.NewSetupCommandExecutorWithSDK(boxName, false, 2, optSetup.dockerClient.SDKExecFunc())

	groups := []parallel.CommandGroup{
		{
			Name: "System Update",
			Commands: []string{
				"apt update -y",
				"apt full-upgrade -y",
			},
			Parallel: false,
		},
		{
			Name: "System Optimization",
			Commands: []string{
				"apt autoremove -y",
				"apt autoclean",
			},
			Parallel: true,
		},
	}

	return executor.ExecuteCommandGroups(groups)
}

func (optSetup *OptimizedSetup) FastInit(projectName string, projectConfig *config.ProjectConfig, cfg *config.Config, workspacePath string, forceFlag bool) error {
	boxName := fmt.Sprintf("coderaft_%s", projectName)
	baseImage := cfg.GetEffectiveBaseImage(&config.Project{
		Name:      projectName,
		BaseImage: "ubuntu:22.04",
	}, projectConfig)

	workspaceBox := "/workspace"
	if projectConfig != nil && projectConfig.WorkingDir != "" {
		workspaceBox = projectConfig.WorkingDir
	}

	ui.Status("fast initialization of '%s'...", boxName)

	effectiveImage := baseImage
	if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		buildCfg := &docker.BuildImageConfig{
			BaseImage:     baseImage,
			SetupCommands: projectConfig.SetupCommands,
			Environment:   projectConfig.Environment,
			Labels:        projectConfig.Labels,
			WorkingDir:    workspaceBox,
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
		ui.Status("force flag detected, recreating box...")
	}

	ui.Status("creating box...")
	configMap := make(map[string]interface{})

	boxID, err := optSetup.dockerClient.CreateBoxWithConfig(boxName, effectiveImage, workspacePath, workspaceBox, configMap)
	if err != nil {
		return fmt.Errorf("failed to create box: %w", err)
	}

	ui.Status("starting box...")
	if err := optSetup.dockerClient.StartBox(boxID); err != nil {
		return fmt.Errorf("failed to start box: %w", err)
	}

	ui.Status("waiting for box to be ready...")
	if err := optSetup.dockerClient.WaitForBox(boxName, 30*time.Second); err != nil {
		return fmt.Errorf("box failed to start: %w", err)
	}

	ui.Status("setting up coderaft commands...")
	if err := optSetup.dockerClient.SetupCoderaftInBoxWithUpdate(boxName, projectName); err != nil {
		return fmt.Errorf("failed to setup coderaft in box: %w", err)
	}

	if effectiveImage == baseImage && projectConfig != nil && len(projectConfig.SetupCommands) > 0 {

		if err := optSetup.OptimizedSystemUpdate(boxName); err != nil {
			ui.Warning("system update failed: %v", err)
		}

		ui.Status("installing packages (%d commands)...", len(projectConfig.SetupCommands))
		if err := optSetup.dockerClient.ExecuteSetupCommandsWithOutput(boxName, projectConfig.SetupCommands, false); err != nil {
			return fmt.Errorf("failed to execute setup commands: %w", err)
		}

		_ = WriteLockFileForBox(boxName, projectName, workspacePath, baseImage, "")
	}

	return nil
}

func (optSetup *OptimizedSetup) FastUp(projectConfig *config.ProjectConfig, projectName, boxName, baseImage, cwd, workspaceBox string) error {
	ui.Status("fast startup of environment...")

	effectiveImage := baseImage
	if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		buildCfg := &docker.BuildImageConfig{
			BaseImage:     baseImage,
			SetupCommands: projectConfig.SetupCommands,
			Environment:   projectConfig.Environment,
			Labels:        projectConfig.Labels,
			WorkingDir:    workspaceBox,
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

	configMap := make(map[string]interface{})

	ui.Status("creating optimized box...")
	boxID, err := optSetup.dockerClient.CreateBoxWithConfig(boxName, effectiveImage, cwd, workspaceBox, configMap)
	if err != nil {
		return fmt.Errorf("failed to create box: %w", err)
	}

	if err := optSetup.dockerClient.StartBox(boxID); err != nil {
		return fmt.Errorf("failed to start box: %w", err)
	}

	ui.Status("waiting for box startup...")
	if err := optSetup.dockerClient.WaitForBox(boxName, 30*time.Second); err != nil {
		return fmt.Errorf("box failed to start: %w", err)
	}

	if err := optSetup.dockerClient.SetupCoderaftInBoxWithUpdate(boxName, projectName); err != nil {
		return fmt.Errorf("failed to setup coderaft in box: %w", err)
	}

	lockfilePath := filepath.Join(cwd, "coderaft.lock")
	if _, err := os.Stat(lockfilePath); err == nil {
		ui.Status("processing lock file...")
		if err := optSetup.processLockFile(boxName, lockfilePath); err != nil {
			return fmt.Errorf("failed to process lock file: %w", err)
		}
	}

	if effectiveImage == baseImage && projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
		if err := optSetup.OptimizedSystemUpdate(boxName); err != nil {
			ui.Warning("system update failed: %v", err)
		}

		ui.Status("installing packages (%d commands)...", len(projectConfig.SetupCommands))
		if err := optSetup.dockerClient.ExecuteSetupCommandsWithOutput(boxName, projectConfig.SetupCommands, false); err != nil {
			return fmt.Errorf("failed to execute setup commands: %w", err)
		}

		_ = WriteLockFileForBox(boxName, projectName, cwd, baseImage, "")
	}

	return nil
}

func (optSetup *OptimizedSetup) processLockFile(boxName, lockfilePath string) error {
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return err
	}

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
		ui.Status("replaying %d commands from lock file...", len(cmds))
		return optSetup.dockerClient.ExecuteSetupCommandsWithOutput(boxName, cmds, false)
	}

	return nil
}

func (optSetup *OptimizedSetup) PrewarmImage(image string) error {
	ui.Status("prewarming image %s...", image)
	return optSetup.dockerClient.PullImage(image)
}

func (optSetup *OptimizedSetup) OptimizeEnvironment(boxName string) error {
	ui.Status("optimizing environment...")

	executor := parallel.NewSetupCommandExecutorWithSDK(boxName, false, 3, optSetup.dockerClient.SDKExecFunc())

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
