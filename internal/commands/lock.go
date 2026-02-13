package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

type lockFile struct {
	Version     int               `json:"version"`
	Project     string            `json:"project"`
	IslandName  string            `json:"ISLAND_NAME"`
	CreatedAt   string            `json:"created_at"`
	Checksum    string            `json:"checksum"`
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
	Gpus         string            `json:"gpus,omitempty"`
}

type lockPackages struct {
	Apt  []string `json:"apt,omitempty"`
	Pip  []string `json:"pip,omitempty"`
	Npm  []string `json:"npm,omitempty"`
	Yarn []string `json:"yarn,omitempty"`
	Pnpm []string `json:"pnpm,omitempty"`
}

type lockRegistries struct {
	PipIndexURL   string   `json:"pip_index_url,omitempty"`
	PipExtraIndex []string `json:"pip_extra_index_urls,omitempty"`
	NpmRegistry   string   `json:"npm_registry,omitempty"`
	YarnRegistry  string   `json:"yarn_registry,omitempty"`
	PnpmRegistry  string   `json:"pnpm_registry,omitempty"`
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
	Long: `Generate a deterministic, checksummed environment snapshot as coderaft.lock.json.

The lock file captures the full island state: base image digest, container
configuration, every installed package (apt, pip, npm, yarn, pnpm) with
pinned versions, registry URLs, and apt sources. Package lists are sorted
alphabetically for deterministic output and a SHA-256 checksum is computed
over the reproducibility-critical fields so teammates can quickly verify
whether two lock files describe the same environment.

Commit coderaft.lock.json to your repository. Teammates can then run
'coderaft apply <project>' to reconcile their island to match, or
'coderaft verify <project>' to check for drift.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WriteLockFileForProject(args[0], lockOutput)
	},
}

func init() {
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

	mounts, err := dockerClient.GetMounts(IslandName)
	if err != nil {
		return fmt.Errorf("failed to get mounts for island '%s': %w", IslandName, err)
	}
	ports, err := dockerClient.GetPortMappings(IslandName)
	if err != nil {
		return fmt.Errorf("failed to get port mappings for island '%s': %w", IslandName, err)
	}

	envMap, workdir, user, restart, labels, capabilities, resources, network := dockerClient.GetContainerMeta(IslandName)

	ui.Status("gathering package information...")
	aptList, pipList, npmList, yarnList, pnpmList := dockerClient.QueryPackagesParallel(IslandName)

	sort.Strings(aptList)
	sort.Strings(pipList)
	sort.Strings(npmList)
	sort.Strings(yarnList)
	sort.Strings(pnpmList)

	aptSnapshot, aptSources, aptRelease := dockerClient.GetAptSources(IslandName)
	pipIndex, pipExtras := dockerClient.GetPipRegistries(IslandName)
	npmReg, yarnReg, pnpmReg := dockerClient.GetNodeRegistries(IslandName)

	var gpuConfig string
	if pcfg, pcfgErr := configManager.LoadProjectConfig(workspacePath); pcfgErr == nil && pcfg != nil {
		gpuConfig = pcfg.Gpus
	}

	lf := lockFile{
		Version:    2,
		Project:    projectName,
		IslandName: IslandName,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		BaseImage:  lockImage{Name: imgName, Digest: digest, ID: imgID},
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
			Gpus:         gpuConfig,
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
		},
		AptSources: lockAptSources{
			SnapshotURL:   aptSnapshot,
			SourcesLists:  aptSources,
			PinnedRelease: aptRelease,
		},
	}

	if pcfg2, pcfg2Err := configManager.LoadProjectConfig(workspacePath); pcfg2Err == nil && pcfg2 != nil {
		if len(pcfg2.SetupCommands) > 0 {
			lf.SetupScript = pcfg2.SetupCommands
		}
	}

	lf.Checksum = computeLockChecksum(&lf)

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

	ui.Success("wrote lock file: %s", finalOut)
	return nil
}

func computeLockChecksum(lf *lockFile) string {
	h := sha256.New()

	h.Write([]byte(lf.BaseImage.Name))
	h.Write([]byte(lf.BaseImage.Digest))

	h.Write([]byte("container:"))
	h.Write([]byte(lf.Container.WorkingDir))
	h.Write([]byte(lf.Container.User))
	h.Write([]byte(lf.Container.Restart))
	h.Write([]byte(lf.Container.Network))
	h.Write([]byte(lf.Container.Gpus))
	writeList := func(prefix string, items []string) {
		h.Write([]byte(prefix))
		for _, item := range items {
			h.Write([]byte(item))
			h.Write([]byte{0})
		}
	}
	writeList("ports:", lf.Container.Ports)
	writeList("volumes:", lf.Container.Volumes)
	writeList("capabilities:", lf.Container.Capabilities)
	writeSortedMap := func(prefix string, m map[string]string) {
		h.Write([]byte(prefix))
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte{0})
			h.Write([]byte(m[k]))
			h.Write([]byte{0})
		}
	}
	writeSortedMap("env:", lf.Container.Environment)
	writeSortedMap("labels:", lf.Container.Labels)
	writeSortedMap("resources:", lf.Container.Resources)

	writeList("setup:", lf.SetupScript)

	writeList("apt:", lf.Packages.Apt)
	writeList("pip:", lf.Packages.Pip)
	writeList("npm:", lf.Packages.Npm)
	writeList("yarn:", lf.Packages.Yarn)
	writeList("pnpm:", lf.Packages.Pnpm)

	h.Write([]byte(lf.Registries.PipIndexURL))
	for _, u := range lf.Registries.PipExtraIndex {
		h.Write([]byte(u))
	}
	h.Write([]byte(lf.Registries.NpmRegistry))
	h.Write([]byte(lf.Registries.YarnRegistry))
	h.Write([]byte(lf.Registries.PnpmRegistry))

	h.Write([]byte(lf.AptSources.SnapshotURL))
	for _, s := range lf.AptSources.SourcesLists {
		h.Write([]byte(s))
	}
	h.Write([]byte(lf.AptSources.PinnedRelease))

	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
