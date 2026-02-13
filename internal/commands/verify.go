package commands

import (
	"context"
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

var verifyTimeout int

var verifyCmd = &cobra.Command{
	Use:   "verify <project>",
	Short: "Verify current island matches coderaft.lock.json exactly",
	Long: `Compare the running island against coderaft.lock.json and report all drift.

Checks base image digest, container configuration (ports, volumes, environment,
capabilities, resources, working directory, user, restart policy, network),
every package set (apt, pip, npm, yarn, pnpm), registry URLs, and apt sources.
For each drifted package manager the output shows exactly which packages were
added, removed, or changed version.

If the lock file contains a checksum (v2+), it is recomputed from the live
island state and compared first for a fast-path pass/fail.

Exit code 0 means the island matches. Non-zero means drift was detected.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		timeout := time.Duration(verifyTimeout) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		type verifyResult struct {
			err error
		}
		resultCh := make(chan verifyResult, 1)
		go func() {
			resultCh <- verifyResult{err: runVerify(projectName)}
		}()

		select {
		case res := <-resultCh:
			return res.err
		case <-ctx.Done():
			return fmt.Errorf("verify timed out after %d seconds", verifyTimeout)
		}
	},
}

func runVerify(projectName string) error {
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
	var lf lockFile
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

	aptSnapshot, aptSources, aptRelease := dockerClient.GetAptSources(proj.IslandName)
	npmReg, yarnReg, pnpmReg := dockerClient.GetNodeRegistries(proj.IslandName)
	pipIndex, pipExtras := dockerClient.GetPipRegistries(proj.IslandName)
	aptList, pipList, npmList, yarnList, pnpmList := dockerClient.QueryPackagesParallel(proj.IslandName)
	envMap, workdir, user, restart, labels, capabilities, resources, network := dockerClient.GetContainerMeta(proj.IslandName)
	livePorts, _ := dockerClient.GetPortMappings(proj.IslandName)
	liveMounts, _ := dockerClient.GetMounts(proj.IslandName)

	sort.Strings(aptList)
	sort.Strings(pipList)
	sort.Strings(npmList)
	sort.Strings(yarnList)
	sort.Strings(pnpmList)

	if lf.Checksum != "" {
		ui.Status("verifying lock file checksum...")
		liveLf := lockFile{
			BaseImage: lf.BaseImage,
			Container: lockContainer{
				WorkingDir:   workdir,
				User:         user,
				Restart:      restart,
				Network:      network,
				Ports:        livePorts,
				Volumes:      liveMounts,
				Labels:       labels,
				Environment:  envMap,
				Capabilities: capabilities,
				Resources:    resources,
			},
			SetupScript: lf.SetupScript,
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

		if lf.BaseImage.Digest != "" {
			if liveDigest, _, _ := dockerClient.GetImageDigestInfo(lf.BaseImage.Name); liveDigest != "" {
				liveLf.BaseImage.Digest = liveDigest
			}
		}

		liveChecksum := computeLockChecksum(&liveLf)
		if liveChecksum == lf.Checksum {
			ui.Success("island matches coderaft.lock.json (checksum fast-path)")
			ui.Detail("checksum", lf.Checksum)
			return nil
		}
		ui.Status("checksum mismatch (lock=%s live=%s), performing detailed diff...", lf.Checksum[:24]+"...", liveChecksum[:24]+"...")
	}

	var drifts []string

	if lf.BaseImage.Digest != "" {
		liveDigest, _, _ := dockerClient.GetImageDigestInfo(lf.BaseImage.Name)
		if liveDigest != "" && liveDigest != lf.BaseImage.Digest {
			drifts = append(drifts, fmt.Sprintf("base image digest mismatch: lock=%s current=%s", lf.BaseImage.Digest, liveDigest))
		}
	}

	if lf.Container.WorkingDir != "" && lf.Container.WorkingDir != workdir {
		drifts = append(drifts, fmt.Sprintf("working_dir mismatch: lock=%s current=%s", lf.Container.WorkingDir, workdir))
	}
	if lf.Container.User != "" && lf.Container.User != user {
		drifts = append(drifts, fmt.Sprintf("user mismatch: lock=%s current=%s", lf.Container.User, user))
	}
	if lf.Container.Restart != "" && lf.Container.Restart != restart {
		drifts = append(drifts, fmt.Sprintf("restart policy mismatch: lock=%s current=%s", lf.Container.Restart, restart))
	}
	if lf.Container.Network != "" && lf.Container.Network != network {
		drifts = append(drifts, fmt.Sprintf("network mismatch: lock=%s current=%s", lf.Container.Network, network))
	}
	if len(lf.Container.Ports) > 0 && !stringSetEqual(lf.Container.Ports, livePorts) {
		drifts = append(drifts, fmt.Sprintf("ports mismatch: lock=%v current=%v", lf.Container.Ports, livePorts))
	}
	if len(lf.Container.Volumes) > 0 && !stringSetEqual(lf.Container.Volumes, liveMounts) {
		drifts = append(drifts, fmt.Sprintf("volumes mismatch: lock=%d entries current=%d entries", len(lf.Container.Volumes), len(liveMounts)))
	}
	if len(lf.Container.Capabilities) > 0 && !stringSetEqual(lf.Container.Capabilities, capabilities) {
		drifts = append(drifts, fmt.Sprintf("capabilities mismatch: lock=%v current=%v", lf.Container.Capabilities, capabilities))
	}
	if len(lf.Container.Environment) > 0 {
		for k, lockVal := range lf.Container.Environment {
			if liveVal, ok := envMap[k]; !ok {
				drifts = append(drifts, fmt.Sprintf("env var '%s' missing in live island (lock=%s)", k, lockVal))
			} else if liveVal != lockVal {
				drifts = append(drifts, fmt.Sprintf("env var '%s' mismatch: lock=%s current=%s", k, lockVal, liveVal))
			}
		}
	}
	if len(lf.Container.Resources) > 0 {
		for k, lockVal := range lf.Container.Resources {
			if liveVal, ok := resources[k]; !ok {
				drifts = append(drifts, fmt.Sprintf("resource '%s' missing in live island (lock=%s)", k, lockVal))
			} else if liveVal != lockVal {
				drifts = append(drifts, fmt.Sprintf("resource '%s' mismatch: lock=%s current=%s", k, lockVal, liveVal))
			}
		}
	}

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
}

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
	verifyCmd.Flags().IntVar(&verifyTimeout, "timeout", 300, "Timeout in seconds for the verify operation")
}
