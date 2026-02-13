package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

type applyLockFile struct {
	Version    int            `json:"version"`
	Project    string         `json:"project"`
	IslandName string         `json:"ISLAND_NAME"`
	Packages   lockPackages   `json:"packages"`
	Registries lockRegistries `json:"registries"`
	AptSources lockAptSources `json:"apt_sources"`
}

var applyDryRun bool

var applyCmd = &cobra.Command{
	Use:   "apply <project>",
	Short: "Apply coderaft.lock.json: set registries and apt sources, then reconcile packages",
	Long: `Apply a coderaft.lock.json to the running island.

Configures registries (pip, npm, yarn, pnpm) and apt sources to match the
lock file, then reconciles every package set so the island ends up with
exactly the versions recorded in the lock.

Use --dry-run to preview the changes without modifying the island.`,
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

		var lf applyLockFile
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

		var applyCmds []string

		if len(lf.AptSources.SourcesLists) > 0 {

			heredoc := "cat > /etc/apt/sources.list <<'EOF'\n" + strings.Join(lf.AptSources.SourcesLists, "\n") + "\nEOF"
			applyCmds = append(applyCmds,
				"cp /etc/apt/sources.list /etc/apt/sources.list.bak 2>/dev/null || true",
				"rm -f /etc/apt/sources.list.d/*.list 2>/dev/null || true",
				heredoc,
			)
		}
		if lf.AptSources.PinnedRelease != "" {
			applyCmds = append(applyCmds, fmt.Sprintf("echo 'APT::Default-Release \"%s\";' > /etc/apt/apt.conf.d/99defaultrelease", escapeBash(lf.AptSources.PinnedRelease)))
		}
		if len(lf.AptSources.SourcesLists) > 0 {
			applyCmds = append(applyCmds, "apt update -y")
		}

		if lf.Registries.PipIndexURL != "" || len(lf.Registries.PipExtraIndex) > 0 {
			var b strings.Builder
			b.WriteString("cat > /etc/pip.conf <<'EOF'\n[global]\n")
			if lf.Registries.PipIndexURL != "" {
				b.WriteString("index-url = ")
				b.WriteString(lf.Registries.PipIndexURL)
				b.WriteString("\n")
			}
			for _, u := range lf.Registries.PipExtraIndex {
				if strings.TrimSpace(u) == "" {
					continue
				}
				b.WriteString("extra-index-url = ")
				b.WriteString(u)
				b.WriteString("\n")
			}
			b.WriteString("EOF")
			applyCmds = append(applyCmds, b.String())
		}

		if lf.Registries.NpmRegistry != "" {
			applyCmds = append(applyCmds, fmt.Sprintf("npm config set registry %s -g", lf.Registries.NpmRegistry))
		}
		if lf.Registries.YarnRegistry != "" {
			applyCmds = append(applyCmds, fmt.Sprintf("yarn config set npmRegistryServer %s -g", lf.Registries.YarnRegistry))
		}
		if lf.Registries.PnpmRegistry != "" {
			applyCmds = append(applyCmds, fmt.Sprintf("pnpm config set registry %s -g", lf.Registries.PnpmRegistry))
		}

		curApt, curPip, curNpm, curYarn, curPnpm := dockerClient.QueryPackagesParallel(proj.IslandName)
		actions := buildReconcileActions(lf.Packages, curApt, curPip, curNpm, curYarn, curPnpm)

		if applyDryRun {
			ui.Status("dry run — the following changes would be applied:")
			if len(applyCmds) > 0 {
				ui.Detail("registry/source commands", fmt.Sprintf("%d", len(applyCmds)))
				for _, c := range applyCmds {
					lines := strings.SplitN(c, "\n", 2)
					ui.Item(lines[0])
				}
			}
			if len(actions) > 0 {
				ui.Detail("package reconciliation commands", fmt.Sprintf("%d", len(actions)))
				for _, a := range actions {
					ui.Item(a)
				}
			}
			if len(applyCmds) == 0 && len(actions) == 0 {
				ui.Success("island already matches lockfile — nothing to do")
			}
			return nil
		}

		if err := dockerClient.ExecuteSetupCommandsWithOutput(proj.IslandName, applyCmds, false); err != nil {
			return fmt.Errorf("failed applying registries/sources: %w", err)
		}

		if len(actions) > 0 {
			if err := dockerClient.ExecuteSetupCommandsWithOutput(proj.IslandName, actions, true); err != nil {
				return fmt.Errorf("failed to reconcile packages: %w", err)
			}
		}

		ui.Success("applied lockfile: registries/sources configured and packages reconciled")
		return nil
	},
}

func escapeBash(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}

func parseMap(list []string, sep string) map[string]string {
	m := map[string]string{}
	for _, line := range list {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if sep == "==" {
			if i := strings.Index(s, "=="); i != -1 {
				name := strings.ToLower(strings.TrimSpace(s[:i]))
				ver := strings.TrimSpace(s[i+2:])
				m[name] = ver
			}
			continue
		}
		if sep == "@" {

			idx := strings.LastIndex(s, "@")
			if idx > 0 {
				name := strings.ToLower(strings.TrimSpace(s[:idx]))
				ver := strings.TrimSpace(s[idx+1:])
				m[name] = ver
			}
			continue
		}
		if sep == "=" {
			if i := strings.Index(s, "="); i != -1 {
				name := strings.ToLower(strings.TrimSpace(s[:i]))
				ver := strings.TrimSpace(s[i+1:])
				m[name] = ver
			}
		}
	}
	return m
}

func keysNotIn(a, b map[string]string) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

func buildReconcileActions(lockPkgs lockPackages, curApt, curPip, curNpm, curYarn, curPnpm []string) []string {
	var cmds []string

	lockA := parseMap(lockPkgs.Apt, "=")
	curA := parseMap(curApt, "=")
	lockP := parseMap(lockPkgs.Pip, "==")
	curP := parseMap(curPip, "==")
	lockN := parseMap(lockPkgs.Npm, "@")
	curN := parseMap(curNpm, "@")
	lockY := parseMap(lockPkgs.Yarn, "@")
	curY := parseMap(curYarn, "@")
	lockQ := parseMap(lockPkgs.Pnpm, "@")
	curQ := parseMap(curPnpm, "@")

	var aptInstall []string
	for name, ver := range lockA {
		if curVer, ok := curA[name]; !ok || curVer != ver {
			aptInstall = append(aptInstall, fmt.Sprintf("%s=%s", name, ver))
		}
	}
	if len(aptInstall) > 0 {
		cmds = append(cmds, "apt update -y", "DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(aptInstall, " "))
	}

	for _, extra := range keysNotIn(curA, lockA) {
		cmds = append(cmds, fmt.Sprintf("apt-get remove -y %s", extra))
	}
	if len(keysNotIn(curA, lockA)) > 0 {
		cmds = append(cmds, "apt-get autoremove -y")
	}

	for name, ver := range lockP {
		if curVer, ok := curP[name]; !ok || curVer != ver {
			cmds = append(cmds, fmt.Sprintf("python3 -m pip install %s==%s", name, ver))
		}
	}
	for _, extra := range keysNotIn(curP, lockP) {
		cmds = append(cmds, fmt.Sprintf("python3 -m pip uninstall -y %s", extra))
	}

	for name, ver := range lockN {
		if curVer, ok := curN[name]; !ok || curVer != ver {
			cmds = append(cmds, fmt.Sprintf("npm i -g %s@%s", name, ver))
		}
	}
	for _, extra := range keysNotIn(curN, lockN) {
		cmds = append(cmds, fmt.Sprintf("npm rm -g %s", extra))
	}

	for name, ver := range lockY {
		if curVer, ok := curY[name]; !ok || curVer != ver {
			cmds = append(cmds, fmt.Sprintf("yarn global add %s@%s", name, ver))
		}
	}
	for _, extra := range keysNotIn(curY, lockY) {
		cmds = append(cmds, fmt.Sprintf("yarn global remove %s", extra))
	}

	for name, ver := range lockQ {
		if curVer, ok := curQ[name]; !ok || curVer != ver {
			cmds = append(cmds, fmt.Sprintf("pnpm add -g %s@%s", name, ver))
		}
	}
	for _, extra := range keysNotIn(curQ, lockQ) {
		cmds = append(cmds, fmt.Sprintf("pnpm remove -g %s", extra))
	}

	return cmds
}

func init() {
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Preview changes without modifying the island")
}
