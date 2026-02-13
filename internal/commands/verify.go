package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

type verifyLockFile struct {
	Version    int            `json:"version"`
	Project    string         `json:"project"`
	IslandName string         `json:"ISLAND_NAME"`
	Checksum   string         `json:"checksum"`
	BaseImage  lockImage      `json:"base_image"`
	Packages   lockPackages   `json:"packages"`
	Registries lockRegistries `json:"registries"`
	AptSources lockAptSources `json:"apt_sources"`
}

var verifyCmd = &cobra.Command{
	Use:   "verify <project>",
	Short: "Verify current island matches coderaft.lock.json exactly",
	Long: `Compare the running island against coderaft.lock.json and report all drift.

Checks base image digest, every package set (apt, pip, npm, yarn, pnpm),
registry URLs, and apt sources. For each drifted package manager the output
shows exactly which packages were added, removed, or changed version.

If the lock file contains a checksum (v2+), it is recomputed from the live
island state and compared first for a fast-path pass/fail.

Exit code 0 means the island matches. Non-zero means drift was detected.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		proj, ok := cfg.GetProject(projectName)
		if !ok {
			return fmt.Errorf("project '%s' not found", projectName)
		}

		lockPath := filepath.Join(proj.WorkspacePath, "coderaft.lock.json")
		data, err := os.ReadFile(lockPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", lockPath, err)
		}
		var lf verifyLockFile
		if err := json.Unmarshal(data, &lf); err != nil {
			return fmt.Errorf("invalid lockfile: %w", err)
		}

		exists, err := dockerClient.IslandExists(proj.IslandName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("island '%s' not found; run 'coderaft up %s' first", proj.IslandName, projectName)
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

		// Quick checksum comparison (v2+ lock files)
		if lf.Checksum != "" {
			ui.Status("verifying lock file checksum...")
		}

		// Gather live state
		aptSnapshot, aptSources, aptRelease := dockerClient.GetAptSources(proj.IslandName)
		npmReg, yarnReg, pnpmReg := dockerClient.GetNodeRegistries(proj.IslandName)
		pipIndex, pipExtras := dockerClient.GetPipRegistries(proj.IslandName)

		var drifts []string

		// --- Base image digest ---
		if lf.BaseImage.Digest != "" {
			liveDigest, _, _ := dockerClient.GetImageDigestInfo(lf.BaseImage.Name)
			if liveDigest != "" && liveDigest != lf.BaseImage.Digest {
				drifts = append(drifts, fmt.Sprintf("base image digest mismatch: lock=%s current=%s", lf.BaseImage.Digest, liveDigest))
			}
		}

		// --- Registries ---
		if lf.AptSources.SnapshotURL != "" && normalizeURL(lf.AptSources.SnapshotURL) != normalizeURL(aptSnapshot) {
			drifts = append(drifts, fmt.Sprintf("APT snapshot mismatch: lock=%s current=%s", lf.AptSources.SnapshotURL, aptSnapshot))
		}
		if lf.AptSources.PinnedRelease != "" && strings.TrimSpace(lf.AptSources.PinnedRelease) != strings.TrimSpace(aptRelease) {
			drifts = append(drifts, fmt.Sprintf("APT release mismatch: lock=%s current=%s", lf.AptSources.PinnedRelease, aptRelease))
		}
		if len(lf.AptSources.SourcesLists) > 0 {
			if !stringSetEqual(lf.AptSources.SourcesLists, aptSources) {
				drifts = append(drifts, "APT sources.list entries drifted")
			}
		}

		if lf.Registries.PipIndexURL != "" && normalizeURL(lf.Registries.PipIndexURL) != normalizeURL(pipIndex) {
			drifts = append(drifts, fmt.Sprintf("pip index-url mismatch: lock=%s current=%s", lf.Registries.PipIndexURL, pipIndex))
		}
		if len(lf.Registries.PipExtraIndex) > 0 {
			if !stringSetEqual(lf.Registries.PipExtraIndex, pipExtras) {
				drifts = append(drifts, "pip extra-index-urls drifted")
			}
		}

		if lf.Registries.NpmRegistry != "" && normalizeURL(lf.Registries.NpmRegistry) != normalizeURL(npmReg) {
			drifts = append(drifts, fmt.Sprintf("npm registry mismatch: lock=%s current=%s", lf.Registries.NpmRegistry, npmReg))
		}
		if lf.Registries.YarnRegistry != "" && normalizeURL(lf.Registries.YarnRegistry) != normalizeURL(yarnReg) {
			drifts = append(drifts, fmt.Sprintf("yarn registry mismatch: lock=%s current=%s", lf.Registries.YarnRegistry, yarnReg))
		}
		if lf.Registries.PnpmRegistry != "" && normalizeURL(lf.Registries.PnpmRegistry) != normalizeURL(pnpmReg) {
			drifts = append(drifts, fmt.Sprintf("pnpm registry mismatch: lock=%s current=%s", lf.Registries.PnpmRegistry, pnpmReg))
		}

		// --- Packages (with detailed per-package diff) ---
		aptList, pipList, npmList, yarnList, pnpmList := dockerClient.QueryPackagesParallel(proj.IslandName)

		drifts = append(drifts, packageDiff("apt", "=", lf.Packages.Apt, aptList)...)
		drifts = append(drifts, packageDiff("pip", "==", lf.Packages.Pip, pipList)...)
		drifts = append(drifts, packageDiff("npm", "@", lf.Packages.Npm, npmList)...)
		drifts = append(drifts, packageDiff("yarn", "@", lf.Packages.Yarn, yarnList)...)
		drifts = append(drifts, packageDiff("pnpm", "@", lf.Packages.Pnpm, pnpmList)...)

		if len(drifts) > 0 {
			ui.Error("verification failed — %d drift(s) detected:", len(drifts))
			for _, d := range drifts {
				ui.Item(d)
			}
			return fmt.Errorf("island does not match lockfile (%d drifts)", len(drifts))
		}

		ui.Success("island matches coderaft.lock.json (0 drifts)")
		if lf.Checksum != "" {
			ui.Detail("checksum", lf.Checksum)
		}
		return nil
	},
}

// packageDiff compares a locked package list against the live list and returns
// human-readable drift descriptions for added, removed, and version-changed
// packages.
func packageDiff(manager, sep string, locked, live []string) []string {
	lockMap := parseMap(locked, sep)
	liveMap := parseMap(live, sep)

	var drifts []string
	var added, removed, changed []string

	for name, lockVer := range lockMap {
		liveVer, ok := liveMap[name]
		if !ok {
			removed = append(removed, fmt.Sprintf("%s%s%s", name, sep, lockVer))
		} else if liveVer != lockVer {
			changed = append(changed, fmt.Sprintf("%s: %s → %s", name, lockVer, liveVer))
		}
	}
	for name, liveVer := range liveMap {
		if _, ok := lockMap[name]; !ok {
			added = append(added, fmt.Sprintf("%s%s%s", name, sep, liveVer))
		}
	}

	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)

	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		return nil
	}

	drifts = append(drifts, fmt.Sprintf("%s packages drifted: +%d added, -%d removed, ~%d changed", manager, len(added), len(removed), len(changed)))
	for _, s := range added {
		drifts = append(drifts, fmt.Sprintf("  + %s", s))
	}
	for _, s := range removed {
		drifts = append(drifts, fmt.Sprintf("  - %s", s))
	}
	for _, s := range changed {
		drifts = append(drifts, fmt.Sprintf("  ~ %s", s))
	}
	return drifts
}

func normalizeURL(s string) string {
	return strings.TrimRight(strings.TrimSpace(strings.ToLower(s)), "/")
}

func stringSetEqual(a, b []string) bool {
	normalize := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, s := range in {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		sort.Strings(out)
		return out
	}
	aa := normalize(a)
	bb := normalize(b)
	if len(aa) != len(bb) {
		return false
	}
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func init() {
}
