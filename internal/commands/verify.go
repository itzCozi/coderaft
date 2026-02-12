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
	BoxName    string         `json:"box_name"`
	Packages   lockPackages   `json:"packages"`
	Registries lockRegistries `json:"registries"`
	AptSources lockAptSources `json:"apt_sources"`
}

var verifyCmd = &cobra.Command{
	Use:   "verify <project>",
	Short: "Verify current box matches coderaft.lock.json exactly",
	Args:  cobra.ExactArgs(1),
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

		exists, err := dockerClient.BoxExists(proj.BoxName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("box '%s' not found; run 'coderaft up %s' first", proj.BoxName, projectName)
		}
		status, err := dockerClient.GetBoxStatus(proj.BoxName)
		if err != nil {
			return err
		}
		if status != "running" {
			if err := dockerClient.StartBox(proj.BoxName); err != nil {
				return fmt.Errorf("failed to start box: %w", err)
			}
		}

		aptSnapshot, aptSources, aptRelease := dockerClient.GetAptSources(proj.BoxName)
		npmReg, yarnReg, pnpmReg := dockerClient.GetNodeRegistries(proj.BoxName)
		pipIndex, pipExtras := dockerClient.GetPipRegistries(proj.BoxName)

		var drifts []string

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

		aptList, pipList, npmList, yarnList, pnpmList := dockerClient.QueryPackagesParallel(proj.BoxName)
		if !stringSetEqual(lf.Packages.Apt, aptList) {
			drifts = append(drifts, "APT packages drifted")
		}
		if !stringSetEqual(lf.Packages.Pip, pipList) {
			drifts = append(drifts, "pip packages drifted")
		}
		if !stringSetEqual(lf.Packages.Npm, npmList) {
			drifts = append(drifts, "npm packages drifted")
		}
		if !stringSetEqual(lf.Packages.Yarn, yarnList) {
			drifts = append(drifts, "yarn packages drifted")
		}
		if !stringSetEqual(lf.Packages.Pnpm, pnpmList) {
			drifts = append(drifts, "pnpm packages drifted")
		}

		if len(drifts) > 0 {
			ui.Error("verification failed, drift detected:")
			for _, d := range drifts {
				ui.Item(d)
			}
			return fmt.Errorf("environment does not match lockfile")
		}

		ui.Success("environment matches coderaft.lock.json")
		return nil
	},
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
	rootCmd.AddCommand(verifyCmd)
}
