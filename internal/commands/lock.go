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

type lockFile struct {
	Version     int               `json:"version"`
	Project     string            `json:"project"`
	IslandName     string            `json:"ISLAND_NAME"`
	CreatedAt   string            `json:"created_at"`
	BaseImage   lockImage         `json:"base_image"`
	Container   lockContainer     `json:"container"`
	Packages    lockPackages      `json:"packages"`
	Registries  lockRegistries    `json:"registries,omitempty"`
	AptSources  lockAptSources    `json:"apt_sources,omitempty"`
	SetupScript []string          `json:"setup_commands,omitempty"`
	Notes       map[string]string `json:"notes,omitempty"`
}

type lockImage struct {
	Name   string `json:"name"`
	Digest string `json:"digest,omitempty"`
	ID     string `json:"id,omitempty"`
}

type lockContainer struct {
	WorkingDir   string            `json:"working_dir,omitempty"`
	User         string            `json:"user,omitempty"`
	Restart      string            `json:"restart,omitempty"`
	Network      string            `json:"network,omitempty"`
	Ports        []string          `json:"ports,omitempty"`
	Volumes      []string          `json:"volumes,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Environment  map[string]string `json:"environment,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Resources    map[string]string `json:"resources,omitempty"`
}

type lockPackages struct {
	Apt  []string `json:"apt,omitempty"`
	Pip  []string `json:"pip,omitempty"`
	Npm  []string `json:"npm,omitempty"`
	Yarn []string `json:"yarn,omitempty"`
	Pnpm []string `json:"pnpm,omitempty"`
}

type lockRegistries struct {
	PipIndexURL   string            `json:"pip_index_url,omitempty"`
	PipExtraIndex []string          `json:"pip_extra_index_urls,omitempty"`
	NpmRegistry   string            `json:"npm_registry,omitempty"`
	YarnRegistry  string            `json:"yarn_registry,omitempty"`
	PnpmRegistry  string            `json:"pnpm_registry,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
}

type lockAptSources struct {
	SnapshotURL   string   `json:"snapshot_url,omitempty"`
	SourcesLists  []string `json:"sources_lists,omitempty"`
	PinnedRelease string   `json:"pinned_release,omitempty"`
}

var (
	lockOutput string
)

var lockCmd = &cobra.Command{
	Use:   "lock <project>",
	Short: "Generate a comprehensive coderaft.lock.json for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		proj, ok := cfg.GetProject(projectName)
		if !ok {
			return fmt.Errorf("project '%s' not found. Run 'coderaft init %s' first", projectName, projectName)
		}

		exists, err := dockerClient.IslandExists(proj.IslandName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("island '%s' does not exist. Start it with 'coderaft up %s'", proj.IslandName, projectName)
		}
		status, err := dockerClient.GetIslandStatus(proj.IslandName)
		if err != nil {
			return err
		}
		if status != "running" {
			if err := dockerClient.StartIsland(proj.IslandName); err != nil {
				return fmt.Errorf("failed to start island '%s': %w", proj.IslandName, err)
			}
		}

		imgName := proj.BaseImage
		digest, imgID, imgErr := dockerClient.GetImageDigestInfo(imgName)
		if imgErr != nil || digest == "" {

			cid, err := dockerClient.GetContainerID(proj.IslandName)
			if err == nil && cid != "" {
				d2, id2, _ := dockerClient.GetImageDigestInfo(cid)
				if d2 != "" || id2 != "" {
					digest, imgID = d2, id2
				}
			}
		}

		mounts, _ := dockerClient.GetMounts(proj.IslandName)
		ports, _ := dockerClient.GetPortMappings(proj.IslandName)

		envMap, workdir, user, restart, labels, capabilities, resources, network := dockerClient.GetContainerMeta(proj.IslandName)

		ui.Status("gathering package information...")
		aptList, pipList, npmList, yarnList, pnpmList := dockerClient.QueryPackagesParallel(proj.IslandName)

		aptSnapshot, aptSources, aptRelease := dockerClient.GetAptSources(proj.IslandName)
		pipIndex, pipExtras := dockerClient.GetPipRegistries(proj.IslandName)
		npmReg, yarnReg, pnpmReg := dockerClient.GetNodeRegistries(proj.IslandName)

		lf := lockFile{
			Version:   1,
			Project:   proj.Name,
			IslandName:   proj.IslandName,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			BaseImage: lockImage{Name: imgName, Digest: digest, ID: imgID},
			Container: lockContainer{
				WorkingDir:   workdir,
				User:         user,
				Restart:      restart,
				Network:      network,
				Ports:        ports,
				Volumes:      mounts,
				Labels:       labels,
				Environment:  envMap,
				Capabilities: capabilities,
				Resources:    resources,
			},
			Packages: lockPackages{
				Apt:  aptList,
				Pip:  pipList,
				Npm:  npmList,
				Yarn: yarnList,
				Pnpm: pnpmList,
			},
			Registries: lockRegistries{
				PipIndexURL:   pipIndex,
				PipExtraIndex: pipExtras,
				NpmRegistry:   npmReg,
				YarnRegistry:  yarnReg,
				PnpmRegistry:  pnpmReg,
				Env:           envMap,
			},
			AptSources: lockAptSources{
				SnapshotURL:   aptSnapshot,
				SourcesLists:  aptSources,
				PinnedRelease: aptRelease,
			},
		}

		if pcfg, err := configManager.LoadProjectConfig(proj.WorkspacePath); err == nil && pcfg != nil {
			if len(pcfg.SetupCommands) > 0 {
				lf.SetupScript = pcfg.SetupCommands
			}
		}

		outPath := lockOutput
		if strings.TrimSpace(outPath) == "" {
			outPath = filepath.Join(proj.WorkspacePath, "coderaft.lock.json")
		}

		b, err := json.MarshalIndent(lf, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal lock file: %w", err)
		}
		if err := os.WriteFile(outPath, b, 0644); err != nil {
			return fmt.Errorf("failed to write lock file: %w", err)
		}

		ui.Success("wrote lock file: %s", outPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lockCmd)
	lockCmd.Flags().StringVarP(&lockOutput, "output", "o", "", "Output path for lock file (default: <workspace>/coderaft.lock.json)")
}

func WriteLockFileForProject(projectName string, outPath string) error {
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	proj, ok := cfg.GetProject(projectName)
	if !ok {
		return fmt.Errorf("project '%s' not found. Run 'coderaft init %s' first", projectName, projectName)
	}

	return WriteLockFileForIsland(proj.IslandName, proj.Name, proj.WorkspacePath, proj.BaseImage, outPath)
}

func WriteLockFileForIsland(IslandName, projectName, workspacePath, baseImage, outPath string) error {
	exists, err := dockerClient.IslandExists(IslandName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("island '%s' does not exist. Start it first", IslandName)
	}
	status, err := dockerClient.GetIslandStatus(IslandName)
	if err != nil {
		return err
	}
	if status != "running" {
		if err := dockerClient.StartIsland(IslandName); err != nil {
			return fmt.Errorf("failed to start island '%s': %w", IslandName, err)
		}
	}

	imgName := baseImage
	digest, imgID, imgErr := dockerClient.GetImageDigestInfo(imgName)
	if imgErr != nil || strings.TrimSpace(digest) == "" {

		cid, err := dockerClient.GetContainerID(IslandName)
		if err == nil && cid != "" {
			d2, id2, _ := dockerClient.GetImageDigestInfo(cid)
			if d2 != "" || id2 != "" {
				digest, imgID = d2, id2
			}
		}
	}

	mounts, _ := dockerClient.GetMounts(IslandName)
	ports, _ := dockerClient.GetPortMappings(IslandName)

	envMap, workdir, user, restart, labels, capabilities, resources, network := dockerClient.GetContainerMeta(IslandName)

	fmt.Printf("Gathering package information in parallel...\n")
	aptList, pipList, npmList, yarnList, pnpmList := dockerClient.QueryPackagesParallel(IslandName)

	aptSnapshot, aptSources, aptRelease := dockerClient.GetAptSources(IslandName)
	pipIndex, pipExtras := dockerClient.GetPipRegistries(IslandName)
	npmReg, yarnReg, pnpmReg := dockerClient.GetNodeRegistries(IslandName)

	lf := lockFile{
		Version:   1,
		Project:   projectName,
		IslandName:   IslandName,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		BaseImage: lockImage{Name: imgName, Digest: digest, ID: imgID},
		Container: lockContainer{
			WorkingDir:   workdir,
			User:         user,
			Restart:      restart,
			Network:      network,
			Ports:        ports,
			Volumes:      mounts,
			Labels:       labels,
			Environment:  envMap,
			Capabilities: capabilities,
			Resources:    resources,
		},
		Packages: lockPackages{
			Apt:  aptList,
			Pip:  pipList,
			Npm:  npmList,
			Yarn: yarnList,
			Pnpm: pnpmList,
		},
		Registries: lockRegistries{
			PipIndexURL:   pipIndex,
			PipExtraIndex: pipExtras,
			NpmRegistry:   npmReg,
			YarnRegistry:  yarnReg,
			PnpmRegistry:  pnpmReg,
			Env:           envMap,
		},
		AptSources: lockAptSources{
			SnapshotURL:   aptSnapshot,
			SourcesLists:  aptSources,
			PinnedRelease: aptRelease,
		},
	}

	if pcfg, err := configManager.LoadProjectConfig(workspacePath); err == nil && pcfg != nil {
		if len(pcfg.SetupCommands) > 0 {
			lf.SetupScript = pcfg.SetupCommands
		}
	}

	finalOut := strings.TrimSpace(outPath)
	if finalOut == "" {
		finalOut = filepath.Join(workspacePath, "coderaft.lock.json")
	}

	b, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}
	if err := os.WriteFile(finalOut, b, 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	fmt.Printf("Wrote lock file: %s\n", finalOut)
	return nil
}
