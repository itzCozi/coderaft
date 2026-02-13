package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/config"
	"coderaft/internal/ui"
)

func engineCmd() string {
	if v := strings.TrimSpace(os.Getenv("CODERAFT_ENGINE")); v != "" {
		return v
	}
	return "docker"
}

var (
	upDotfilesPath string
)

var keepRunningUpFlag bool

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start a coderaft island from the current folder's coderaft.json",
	Long:  "Reads coderaft.json in the current directory and boots the island so new teammates can simply run 'coderaft up'.",
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

		IslandName := fmt.Sprintf("coderaft_%s", projectName)
		baseImage := cfg.GetEffectiveBaseImage(&config.Project{Name: projectName, BaseImage: projectConfig.BaseImage}, projectConfig)

		workspaceIsland := "/island"
		if projectConfig.WorkingDir != "" {
			workspaceIsland = projectConfig.WorkingDir
		}

		exists, err := dockerClient.IslandExists(IslandName)
		if err != nil {
			return fmt.Errorf("failed to check island existence: %w", err)
		}

		if exists {
			status, err := dockerClient.GetIslandStatus(IslandName)
			if err != nil {
				return fmt.Errorf("failed to get island status: %w", err)
			}
			if status != "running" {
				if err := dockerClient.StartIsland(IslandName); err != nil {
					return fmt.Errorf("failed to start existing island: %w", err)
				}
			}

			if !dockerClient.IsIslandInitialized(IslandName) {
				if err := dockerClient.SetupCoderaftOnIsland(IslandName, projectName); err != nil {
					return fmt.Errorf("failed to setup coderaft in existing island: %w", err)
				}
			}
			ui.Success("island is up")
			ui.Detail("workspace", cwd)
			ui.Detail("island", IslandName)
			ui.Detail("image", baseImage)
			ui.Info("hint: run 'coderaft shell %s' to enter the island.", projectName)

			if cfg.Settings != nil && cfg.Settings.AutoStopOnExit && !keepRunningUpFlag {
				if idle, err := dockerClient.IsContainerIdle(IslandName); err == nil && idle {
					ui.Status("stopping island '%s' (auto-stop: idle)...", IslandName)
					if err := dockerClient.StopIsland(IslandName); err != nil {
						ui.Warning("failed to stop island: %v", err)
					}
				}
			}
			return nil
		}

		ui.Status("setting up island '%s' with image '%s'...", IslandName, baseImage)
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
		if err := optimizedSetup.FastUp(projectConfig, projectName, IslandName, baseImage, cwd, workspaceIsland); err != nil {
			return fmt.Errorf("failed to start island: %w", err)
		}

		ui.Success("island is up")
		ui.Detail("workspace", cwd)
		ui.Detail("island", IslandName)
		ui.Detail("image", baseImage)
		ui.Info("hint: run 'coderaft shell %s' to enter the island.", projectName)

		_ = WriteLockFileForIsland(IslandName, projectName, cwd, baseImage, "")

		if cfg.Settings != nil && cfg.Settings.AutoApplyLock {
			lockPath := filepath.Join(cwd, "coderaft.lock.json")
			if _, err := os.Stat(lockPath); err == nil {
				if err := applyLockInline(projectName, lockPath); err != nil {
					fmt.Printf("Warning: failed to auto-apply lockfile: %v\n", err)
				}
			}
		}

		if cfg.Settings != nil && cfg.Settings.AutoStopOnExit && !keepRunningUpFlag {
			if idle, err := dockerClient.IsContainerIdle(IslandName); err == nil && idle {
				ui.Status("stopping island '%s' (auto-stop: idle)...", IslandName)
				if err := dockerClient.StopIsland(IslandName); err != nil {
					ui.Warning("failed to stop island: %v", err)
				}
			}
		}
		return nil
	},
}

func init() {
	upCmd.Flags().StringVar(&upDotfilesPath, "dotfiles", "", "Path to local dotfiles directory to mount into the island")
	upCmd.Flags().BoolVar(&keepRunningUpFlag, "keep-running", false, "Keep the island running after 'up' finishes")
}

func applyLockInline(projectName, lockPath string) error {
	cfg, err := configManager.Load()
	if err != nil {
		return err
	}
	proj, ok := cfg.GetProject(projectName)
	if !ok {
		return fmt.Errorf("project '%s' not registered", projectName)
	}
	exists, err := dockerClient.IslandExists(proj.IslandName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("island '%s' not found", proj.IslandName)
	}
	status, err := dockerClient.GetIslandStatus(proj.IslandName)
	if err != nil {
		return err
	}
	if status != "running" {
		if err := dockerClient.StartIsland(proj.IslandName); err != nil {
			return fmt.Errorf("failed to start island: %w", err)
		}
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return err
	}
	var lf struct {
		Packages   struct{ Apt, Pip, Npm, Yarn, Pnpm []string } `json:"packages"`
		Registries struct {
			PipIndexURL   string   `json:"pip_index_url"`
			PipExtraIndex []string `json:"pip_extra_index_urls"`
			NpmRegistry   string   `json:"npm_registry"`
			YarnRegistry  string   `json:"yarn_registry"`
			PnpmRegistry  string   `json:"pnpm_registry"`
		} `json:"registries"`
		AptSources struct {
			SourcesLists  []string `json:"sources_lists"`
			PinnedRelease string   `json:"pinned_release"`
		} `json:"apt_sources"`
	}
	if err := json.Unmarshal(data, &lf); err != nil {
		return err
	}

	var cmds []string
	if len(lf.AptSources.SourcesLists) > 0 {
		heredoc := "cat > /etc/apt/sources.list <<'EOF'\n" + strings.Join(lf.AptSources.SourcesLists, "\n") + "\nEOF"
		cmds = append(cmds,
			"cp /etc/apt/sources.list /etc/apt/sources.list.bak 2>/dev/null || true",
			"rm -f /etc/apt/sources.list.d/*.list 2>/dev/null || true",
			heredoc,
			"apt update -y",
		)
	}
	if lf.AptSources.PinnedRelease != "" {
		cmds = append(cmds, fmt.Sprintf("echo 'APT::Default-Release \"%s\";' > /etc/apt/apt.conf.d/99defaultrelease", lf.AptSources.PinnedRelease))
	}
	if lf.Registries.PipIndexURL != "" || len(lf.Registries.PipExtraIndex) > 0 {
		var b strings.Builder
		b.WriteString("cat > /etc/pip.conf <<'EOF'\n[global]\n")
		if lf.Registries.PipIndexURL != "" {
			b.WriteString("index-url = " + lf.Registries.PipIndexURL + "\n")
		}
		for _, u := range lf.Registries.PipExtraIndex {
			if s := strings.TrimSpace(u); s != "" {
				b.WriteString("extra-index-url = " + s + "\n")
			}
		}
		b.WriteString("EOF")
		cmds = append(cmds, b.String())
	}
	if lf.Registries.NpmRegistry != "" {
		cmds = append(cmds, fmt.Sprintf("npm config set registry %s -g", lf.Registries.NpmRegistry))
	}
	if lf.Registries.YarnRegistry != "" {
		cmds = append(cmds, fmt.Sprintf("yarn config set npmRegistryServer %s -g", lf.Registries.YarnRegistry))
	}
	if lf.Registries.PnpmRegistry != "" {
		cmds = append(cmds, fmt.Sprintf("pnpm config set registry %s -g", lf.Registries.PnpmRegistry))
	}

	if err := dockerClient.ExecuteSetupCommandsWithOutput(proj.IslandName, cmds, false); err != nil {
		return err
	}

	curApt, curPip, curNpm, curYarn, curPnpm := dockerClient.QueryPackagesParallel(proj.IslandName)
	actions := buildReconcileActions(lockPackages{Apt: lf.Packages.Apt, Pip: lf.Packages.Pip, Npm: lf.Packages.Npm, Yarn: lf.Packages.Yarn, Pnpm: lf.Packages.Pnpm}, curApt, curPip, curNpm, curYarn, curPnpm)
	if len(actions) > 0 {
		if err := dockerClient.ExecuteSetupCommandsWithOutput(proj.IslandName, actions, true); err != nil {
			return err
		}
	}
	fmt.Println("Applied coderaft.lock.json")
	return nil
}
