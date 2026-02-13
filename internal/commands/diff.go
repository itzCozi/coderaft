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

var diffCmd = &cobra.Command{
	Use:   "diff <project>",
	Short: "Show differences between the lock file and the live island state",
	Long: `Compare the coderaft.lock.json with the current state of the running island.

Displays a human-readable, colorized diff showing:
  - Base image and digest drift
  - Container configuration changes (working_dir, user, network, etc.)
  - Package additions, removals, and version changes for all managers
  - Registry URL differences
  - Apt source differences

This is a read-only operation – it never modifies the island.
Use 'coderaft verify' for a pass/fail check, or 'coderaft apply' to reconcile.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiff(args[0])
	},
}

func runDiff(projectName string) error {
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
		return fmt.Errorf("no lock file found at %s — run 'coderaft lock %s' first: %w", lockPath, projectName, err)
	}
	var lf lockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return fmt.Errorf("invalid lock file: %w", err)
	}

	exists, err := dockerClient.IslandExists(proj.IslandName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("island '%s' not found — run 'coderaft up %s' first", proj.IslandName, projectName)
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

	
	envMap, workdir, user, restart, labels, capabilities, resources, network := dockerClient.GetContainerMeta(proj.IslandName)
	livePorts, _ := dockerClient.GetPortMappings(proj.IslandName)
	liveMounts, _ := dockerClient.GetMounts(proj.IslandName)
	liveDigest, _, _ := dockerClient.GetImageDigestInfo(lf.BaseImage.Name)
	aptSnapshot, aptSources, aptRelease := dockerClient.GetAptSources(proj.IslandName)
	pipIndex, pipExtras := dockerClient.GetPipRegistries(proj.IslandName)
	npmReg, yarnReg, pnpmReg := dockerClient.GetNodeRegistries(proj.IslandName)
	aptList, pipList, npmList, yarnList, pnpmList := dockerClient.QueryPackagesParallel(proj.IslandName)

	var sections []string

	
	sec := diffSection("Base Image", []diffLine{
		diffField("name", lf.BaseImage.Name, lf.BaseImage.Name),
		diffField("digest", lf.BaseImage.Digest, liveDigest),
	})
	if sec != "" {
		sections = append(sections, sec)
	}

	
	var containerLines []diffLine
	containerLines = append(containerLines, diffField("working_dir", lf.Container.WorkingDir, workdir))
	containerLines = append(containerLines, diffField("user", lf.Container.User, user))
	containerLines = append(containerLines, diffField("restart", lf.Container.Restart, restart))
	containerLines = append(containerLines, diffField("network", lf.Container.Network, network))
	containerLines = append(containerLines, diffSlice("ports", lf.Container.Ports, livePorts))
	containerLines = append(containerLines, diffSlice("volumes", lf.Container.Volumes, liveMounts))
	containerLines = append(containerLines, diffSlice("capabilities", lf.Container.Capabilities, capabilities))
	containerLines = append(containerLines, diffMap("environment", lf.Container.Environment, envMap))
	containerLines = append(containerLines, diffMap("labels", lf.Container.Labels, labels))
	containerLines = append(containerLines, diffMap("resources", lf.Container.Resources, resources))
	sec = diffSection("Container Config", containerLines)
	if sec != "" {
		sections = append(sections, sec)
	}

	
	for _, pm := range []struct {
		name, sep string
		locked    []string
		live      []string
	}{
		{"apt", "=", lf.Packages.Apt, aptList},
		{"pip", "==", lf.Packages.Pip, pipList},
		{"npm", "@", lf.Packages.Npm, npmList},
		{"yarn", "@", lf.Packages.Yarn, yarnList},
		{"pnpm", "@", lf.Packages.Pnpm, pnpmList},
	} {
		drifts := packageDiff(pm.name, pm.sep, pm.locked, pm.live)
		if len(drifts) > 0 {
			sections = append(sections, strings.Join(drifts, "\n"))
		}
	}

	
	var regLines []diffLine
	regLines = append(regLines, diffField("pip_index_url", lf.Registries.PipIndexURL, pipIndex))
	regLines = append(regLines, diffSlice("pip_extra_index_urls", lf.Registries.PipExtraIndex, pipExtras))
	regLines = append(regLines, diffField("npm_registry", lf.Registries.NpmRegistry, npmReg))
	regLines = append(regLines, diffField("yarn_registry", lf.Registries.YarnRegistry, yarnReg))
	regLines = append(regLines, diffField("pnpm_registry", lf.Registries.PnpmRegistry, pnpmReg))
	sec = diffSection("Registries", regLines)
	if sec != "" {
		sections = append(sections, sec)
	}

	
	var aptLines []diffLine
	aptLines = append(aptLines, diffField("snapshot_url", lf.AptSources.SnapshotURL, aptSnapshot))
	aptLines = append(aptLines, diffSlice("sources_lists", lf.AptSources.SourcesLists, aptSources))
	aptLines = append(aptLines, diffField("pinned_release", lf.AptSources.PinnedRelease, aptRelease))
	sec = diffSection("Apt Sources", aptLines)
	if sec != "" {
		sections = append(sections, sec)
	}

	if len(sections) == 0 {
		ui.Success("no differences — island matches lock file")
		return nil
	}

	fmt.Printf("=== diff: lock vs live for %s ===\n\n", projectName)
	for _, s := range sections {
		fmt.Println(s)
		fmt.Println()
	}
	return nil
}


type diffLine string

func diffField(name, locked, live string) diffLine {
	if locked == "" && live == "" {
		return ""
	}
	if normalizeURL(locked) == normalizeURL(live) {
		return ""
	}
	return diffLine(fmt.Sprintf("  %s:\n    lock: %s\n    live: %s", name, valueOrNone(locked), valueOrNone(live)))
}

func diffSlice(name string, locked, live []string) diffLine {
	normSort := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, s := range in {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		sort.Strings(out)
		return out
	}
	l := normSort(locked)
	r := normSort(live)
	if sliceEqual(l, r) {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %s:\n", name)
	lockSet := make(map[string]bool, len(l))
	for _, s := range l {
		lockSet[s] = true
	}
	liveSet := make(map[string]bool, len(r))
	for _, s := range r {
		liveSet[s] = true
	}
	for _, s := range l {
		if !liveSet[s] {
			fmt.Fprintf(&b, "    - %s (in lock, not live)\n", s)
		}
	}
	for _, s := range r {
		if !lockSet[s] {
			fmt.Fprintf(&b, "    + %s (in live, not lock)\n", s)
		}
	}
	return diffLine(strings.TrimRight(b.String(), "\n"))
}

func diffMap(name string, locked, live map[string]string) diffLine {
	if len(locked) == 0 && len(live) == 0 {
		return ""
	}
	var diffs []string
	keys := make(map[string]bool)
	for k := range locked {
		keys[k] = true
	}
	for k := range live {
		keys[k] = true
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		lockVal, inLock := locked[k]
		liveVal, inLive := live[k]
		if inLock && inLive && lockVal == liveVal {
			continue
		}
		if inLock && !inLive {
			diffs = append(diffs, fmt.Sprintf("    - %s=%s (in lock, not live)", k, lockVal))
		} else if !inLock && inLive {
			diffs = append(diffs, fmt.Sprintf("    + %s=%s (in live, not lock)", k, liveVal))
		} else {
			diffs = append(diffs, fmt.Sprintf("    ~ %s: %s → %s", k, lockVal, liveVal))
		}
	}
	if len(diffs) == 0 {
		return ""
	}
	return diffLine(fmt.Sprintf("  %s:\n%s", name, strings.Join(diffs, "\n")))
}

func diffSection(title string, lines []diffLine) string {
	var nonEmpty []string
	for _, l := range lines {
		if l != "" {
			nonEmpty = append(nonEmpty, string(l))
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	return fmt.Sprintf("[%s]\n%s", title, strings.Join(nonEmpty, "\n"))
}

func valueOrNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
