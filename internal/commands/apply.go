package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

type applyLockFile struct {
	Version    int            `json:"version"`
	Project    string         `json:"project"`
	IslandName string         `json:"ISLAND_NAME"`
	Container  lockContainer  `json:"container"`
	Packages   lockPackages   `json:"packages"`
	Registries lockRegistries `json:"registries"`
	AptSources lockAptSources `json:"apt_sources"`
}

var applyDryRun bool
var applyTimeout int

var applyCmd = &cobra.Command{
	Use:   "apply <project>",
	Short: "Apply coderaft.lock.json: set registries and apt sources, then reconcile packages",
	Long: `Apply a coderaft.lock.json to the running island.

Configures registries (pip, npm, yarn, pnpm) and apt sources to match the
lock file, then reconciles every package set so the island ends up with
exactly the versions recorded in the lock.

Container-level configuration (ports, volumes, environment, capabilities,
resources) cannot be reconciled in-place — you will be warned if they
differ. Use 'coderaft destroy' + 'coderaft up' to recreate if needed.

Use --dry-run to preview the changes without modifying the island.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		timeout := time.Duration(applyTimeout) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		type applyResult struct {
			err error
		}
		resultCh := make(chan applyResult, 1)
		go func() {
			resultCh <- applyResult{err: runApply(ctx, projectName)}
		}()

		select {
		case res := <-resultCh:
			return res.err
		case <-ctx.Done():
			return fmt.Errorf("apply timed out after %d seconds", applyTimeout)
		}
	},
}



func validateRegistryURL(name, rawURL string) error {
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid %s registry URL %q: %w", name, rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "file" {
		return fmt.Errorf("invalid %s registry URL %q: scheme must be http, https, or file", name, rawURL)
	}
	return nil
}

func runApply(ctx context.Context, projectName string) error {
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

	
	registryChecks := []struct{ name, url string }{
		{"pip", lf.Registries.PipIndexURL},
		{"npm", lf.Registries.NpmRegistry},
		{"yarn", lf.Registries.YarnRegistry},
		{"pnpm", lf.Registries.PnpmRegistry},
	}
	for _, rc := range registryChecks {
		if err := validateRegistryURL(rc.name, rc.url); err != nil {
			return fmt.Errorf("lock file contains invalid registry: %w", err)
		}
	}
	for _, u := range lf.Registries.PipExtraIndex {
		if err := validateRegistryURL("pip extra-index", u); err != nil {
			return fmt.Errorf("lock file contains invalid registry: %w", err)
		}
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

	
	envMap, workdir, user, restart, _, capabilities, resources, network := dockerClient.GetContainerMeta(proj.IslandName)
	var containerWarnings []string
	if lf.Container.WorkingDir != "" && lf.Container.WorkingDir != workdir {
		containerWarnings = append(containerWarnings, fmt.Sprintf("working_dir: lock=%s current=%s", lf.Container.WorkingDir, workdir))
	}
	if lf.Container.User != "" && lf.Container.User != user {
		containerWarnings = append(containerWarnings, fmt.Sprintf("user: lock=%s current=%s", lf.Container.User, user))
	}
	if lf.Container.Restart != "" && lf.Container.Restart != restart {
		containerWarnings = append(containerWarnings, fmt.Sprintf("restart: lock=%s current=%s", lf.Container.Restart, restart))
	}
	if lf.Container.Network != "" && lf.Container.Network != network {
		containerWarnings = append(containerWarnings, fmt.Sprintf("network: lock=%s current=%s", lf.Container.Network, network))
	}
	if len(lf.Container.Capabilities) > 0 && !stringSetEqual(lf.Container.Capabilities, capabilities) {
		containerWarnings = append(containerWarnings, fmt.Sprintf("capabilities differ (lock=%v current=%v)", lf.Container.Capabilities, capabilities))
	}
	if len(lf.Container.Resources) > 0 {
		for k, lockVal := range lf.Container.Resources {
			if liveVal, ok := resources[k]; !ok || liveVal != lockVal {
				containerWarnings = append(containerWarnings, fmt.Sprintf("resource %s: lock=%s current=%s", k, lockVal, liveVal))
			}
		}
	}
	if len(lf.Container.Environment) > 0 {
		for k, lockVal := range lf.Container.Environment {
			if liveVal, ok := envMap[k]; !ok || liveVal != lockVal {
				containerWarnings = append(containerWarnings, fmt.Sprintf("env %s: lock=%s current=%s", k, lockVal, liveVal))
			}
		}
	}
	if len(containerWarnings) > 0 {
		ui.Warning("container-level config drift detected (%d items). These cannot be reconciled in-place:", len(containerWarnings))
		for _, w := range containerWarnings {
			ui.Item(w)
		}
		ui.Info("hint: run 'coderaft destroy %s && coderaft up' to recreate with correct config.", projectName)
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
		applyCmds = append(applyCmds, fmt.Sprintf("npm config set registry %s -g", escapeBash(lf.Registries.NpmRegistry)))
	}
	if lf.Registries.YarnRegistry != "" {
		applyCmds = append(applyCmds, fmt.Sprintf("yarn config set npmRegistryServer %s -g", escapeBash(lf.Registries.YarnRegistry)))
	}
	if lf.Registries.PnpmRegistry != "" {
		applyCmds = append(applyCmds, fmt.Sprintf("pnpm config set registry %s -g", escapeBash(lf.Registries.PnpmRegistry)))
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

	
	ui.Status("creating pre-apply snapshot for rollback safety...")
	snapshotTag := fmt.Sprintf("coderaft-snapshot/%s:pre-apply-%d", projectName, time.Now().Unix())
	_, snapshotErr := dockerClient.CommitContainer(proj.IslandName, snapshotTag)
	if snapshotErr != nil {
		ui.Warning("failed to create rollback snapshot: %v (continuing without rollback support)", snapshotErr)
		snapshotTag = ""
	}

	if err := dockerClient.ExecuteSetupCommandsWithOutput(proj.IslandName, applyCmds, false); err != nil {
		if snapshotTag != "" {
			ui.Warning("registry/source configuration failed, snapshot available at %s for manual rollback", snapshotTag)
		}
		return fmt.Errorf("failed applying registries/sources: %w", err)
	}

	if len(actions) > 0 {
		if err := dockerClient.ExecuteSetupCommandsWithOutput(proj.IslandName, actions, true); err != nil {
			if snapshotTag != "" {
				ui.Warning("package reconciliation failed, snapshot available at %s for manual rollback", snapshotTag)
			}
			return fmt.Errorf("failed to reconcile packages: %w", err)
		}
	}

	
	if snapshotTag != "" {
		ui.Status("cleaning up pre-apply snapshot...")
		_ = dockerClient.RunDockerCommand([]string{"rmi", snapshotTag})
	}

	ui.Success("applied lockfile: registries/sources configured and packages reconciled")
	return nil
}




func escapeBash(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", "'\\''")
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", `\$`)
	return s
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

	extraApt := keysNotIn(curA, lockA)
	if len(extraApt) > 0 {
		cmds = append(cmds, "apt-get remove -y "+strings.Join(extraApt, " "))
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
	applyCmd.Flags().IntVar(&applyTimeout, "timeout", 600, "Timeout in seconds for the apply operation")
}
