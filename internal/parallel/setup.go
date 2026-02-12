package parallel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"coderaft/internal/ui"
)

type ExecFunc func(ctx context.Context, containerID string, cmd []string, showOutput bool) (stdout, stderr string, exitCode int, err error)

type SetupCommandExecutor struct {
	islandName    string
	workerPool *WorkerPool
	showOutput bool
	execFunc   ExecFunc
}

func NewSetupCommandExecutor(islandName string, showOutput bool, maxWorkers int) *SetupCommandExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 3
	}

	return &SetupCommandExecutor{
		islandName:    islandName,
		workerPool: NewWorkerPool(maxWorkers, 10*time.Minute),
		showOutput: showOutput,
	}
}

func NewSetupCommandExecutorWithSDK(islandName string, showOutput bool, maxWorkers int, execFn ExecFunc) *SetupCommandExecutor {
	e := NewSetupCommandExecutor(islandName, showOutput, maxWorkers)
	e.execFunc = execFn
	return e
}

type CommandGroup struct {
	Name     string
	Commands []string
	Parallel bool
}

func (sce *SetupCommandExecutor) ExecuteCommandGroups(groups []CommandGroup) error {
	if len(groups) == 0 {
		return nil
	}

	var parallelBatches []Batch
	var sequentialGroups []CommandGroup

	for _, group := range groups {
		if group.Parallel {

			tasks := make([]Task, len(group.Commands))
			for i, cmd := range group.Commands {
				tasks[i] = sce.createCommandTask(cmd, i+1, len(group.Commands), group.Name)
			}
			parallelBatches = append(parallelBatches, Batch{Name: group.Name, Tasks: tasks})
		} else {
			sequentialGroups = append(sequentialGroups, group)
		}
	}

	if len(parallelBatches) > 0 {
		if sce.showOutput {
			ui.Status("executing %d parallel command groups...", len(parallelBatches))
		}

		batchResults := sce.workerPool.ExecuteBatches(parallelBatches)

		for batchName, results := range batchResults {
			for i, err := range results {
				if err != nil {
					return fmt.Errorf("parallel command group '%s', command %d failed: %w", batchName, i+1, err)
				}
			}
		}

		if sce.showOutput {
			ui.Success("all parallel command groups completed")
		}
	}

	for _, group := range sequentialGroups {
		if sce.showOutput {
			ui.Status("executing sequential group: %s", group.Name)
		}

		for i, cmd := range group.Commands {
			if err := sce.executeCommand(cmd, i+1, len(group.Commands), group.Name); err != nil {
				return fmt.Errorf("sequential command group '%s', command %d failed: %w", group.Name, i+1, err)
			}
		}

		if sce.showOutput {
			ui.Success("sequential group '%s' completed", group.Name)
		}
	}

	return nil
}

func (sce *SetupCommandExecutor) ExecuteParallel(commands []string) error {
	if len(commands) == 0 {
		return nil
	}

	groups := sce.categorizeCommands(commands)
	return sce.ExecuteCommandGroups(groups)
}

func (sce *SetupCommandExecutor) categorizeCommands(commands []string) []CommandGroup {
	var groups []CommandGroup

	var aptCommands []string
	var pipCommands []string
	var npmCommands []string
	var yarnCommands []string
	var pnpmCommands []string
	var systemCommands []string
	var otherCommands []string

	for _, cmd := range commands {
		cmdLower := strings.ToLower(strings.TrimSpace(cmd))

		switch {
		case strings.HasPrefix(cmdLower, "apt ") || strings.HasPrefix(cmdLower, "apt-get "):
			aptCommands = append(aptCommands, cmd)
		case strings.HasPrefix(cmdLower, "pip ") || strings.HasPrefix(cmdLower, "pip3 "):
			pipCommands = append(pipCommands, cmd)
		case strings.HasPrefix(cmdLower, "npm "):
			npmCommands = append(npmCommands, cmd)
		case strings.HasPrefix(cmdLower, "yarn "):
			yarnCommands = append(yarnCommands, cmd)
		case strings.HasPrefix(cmdLower, "pnpm "):
			pnpmCommands = append(pnpmCommands, cmd)
		case strings.HasPrefix(cmdLower, "systemctl ") || strings.HasPrefix(cmdLower, "service ") ||
			strings.HasPrefix(cmdLower, "update-alternatives ") || strings.HasPrefix(cmdLower, "adduser ") ||
			strings.HasPrefix(cmdLower, "usermod "):
			systemCommands = append(systemCommands, cmd)
		default:
			otherCommands = append(otherCommands, cmd)
		}
	}

	if len(systemCommands) > 0 {
		groups = append(groups, CommandGroup{Name: "System Commands", Commands: systemCommands, Parallel: false})
	}

	if len(aptCommands) > 0 {
		groups = append(groups, CommandGroup{Name: "APT Packages", Commands: aptCommands, Parallel: false})
	}

	var packageGroups []CommandGroup
	if len(pipCommands) > 0 {
		packageGroups = append(packageGroups, CommandGroup{Name: "Python Packages", Commands: pipCommands, Parallel: true})
	}
	if len(npmCommands) > 0 {
		packageGroups = append(packageGroups, CommandGroup{Name: "NPM Packages", Commands: npmCommands, Parallel: true})
	}
	if len(yarnCommands) > 0 {
		packageGroups = append(packageGroups, CommandGroup{Name: "Yarn Packages", Commands: yarnCommands, Parallel: true})
	}
	if len(pnpmCommands) > 0 {
		packageGroups = append(packageGroups, CommandGroup{Name: "PNPM Packages", Commands: pnpmCommands, Parallel: true})
	}

	groups = append(groups, packageGroups...)

	if len(otherCommands) > 0 {
		groups = append(groups, CommandGroup{Name: "Other Commands", Commands: otherCommands, Parallel: false})
	}

	return groups
}

func (sce *SetupCommandExecutor) createCommandTask(command string, step, total int, groupName string) Task {
	return func() error {
		return sce.executeCommand(command, step, total, groupName)
	}
}

func (sce *SetupCommandExecutor) executeCommand(command string, step, total int, groupName string) error {
	if sce.showOutput {
		ui.Step(step, total, command)
	}

	wrapped := ". /root/.bashrc >/dev/null 2>&1 || true; " + command

	if sce.execFunc != nil {
		ctx := context.Background()
		stdout, stderr, exitCode, err := sce.execFunc(ctx, sce.islandName, []string{"bash", "-c", wrapped}, sce.showOutput)
		if err != nil {
			return fmt.Errorf("command failed: %s: %w", command, err)
		}
		if exitCode != 0 {
			if !sce.showOutput && stderr != "" {
				ui.Error("command failed: %s", command)
				ui.Detail("stderr", stderr)
			}
			_ = stdout
			return fmt.Errorf("command failed: %s: exit code %d", command, exitCode)
		}
		return nil
	}

	cmd := exec.Command("docker", "exec", sce.islandName, "bash", "-c", wrapped)

	if sce.showOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed: %s: %w", command, err)
		}
	} else {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			ui.Error("command failed: %s", command)
			if stderr.Len() > 0 {
				ui.Detail("stderr", stderr.String())
			}
			if stdout.Len() > 0 {
				ui.Detail("stdout", stdout.String())
			}
			return fmt.Errorf("command failed: %s: %w", command, err)
		}
	}

	return nil
}

type PackageQueryExecutor struct {
	islandName    string
	workerPool *WorkerPool
	execFunc   ExecFunc
}

func NewPackageQueryExecutor(islandName string) *PackageQueryExecutor {
	return &PackageQueryExecutor{
		islandName:    islandName,
		workerPool: NewWorkerPool(5, 2*time.Minute),
	}
}

func NewPackageQueryExecutorWithSDK(islandName string, execFn ExecFunc) *PackageQueryExecutor {
	e := NewPackageQueryExecutor(islandName)
	e.execFunc = execFn
	return e
}

type PackageQuery struct {
	Name    string
	Command string
}

func (pqe *PackageQueryExecutor) QueryAllPackages() (map[string][]string, error) {
	queries := []PackageQuery{
		{"apt", "dpkg-query -W -f='${Package}=${Version}\\n' $(apt-mark showmanual 2>/dev/null || true) 2>/dev/null | sort"},
		{"pip", "python3 -m pip freeze 2>/dev/null || pip3 freeze 2>/dev/null || true"},
		{"npm", "npm list -g --depth=0 --json 2>/dev/null || true"},
		{"yarn", "node -e \"(async()=>{const cp=require('child_process');function sh(c){try{return cp.execSync(c,{stdio:['ignore','pipe','ignore']}).toString()}catch(e){return ''}}const dir=sh('yarn global dir').trim();if(!dir){process.exit(0)}const fs=require('fs'),path=require('path');const pkgLock=path.join(dir,'package.json');let deps={};try{const pkg=JSON.parse(fs.readFileSync(pkgLock,'utf8'));deps=Object.assign({},pkg.dependencies||{},pkg.devDependencies||{})}catch{}Object.keys(deps).forEach(n=>{let v='';try{const pj=JSON.parse(fs.readFileSync(path.join(dir,'node_modules',n,'package.json'),'utf8'));v=pj.version||''}catch{}if(v)console.log(n+'@'+v)});})();\" 2>/dev/null || true"},
		{"pnpm", "pnpm ls -g --depth=0 --json 2>/dev/null || true"},
	}

	tasks := make([]StringTask, len(queries))
	for i, query := range queries {
		tasks[i] = pqe.createQueryTask(query.Command)
	}

	results, errors := pqe.workerPool.ExecuteStringTasks(tasks)

	packageLists := make(map[string][]string)
	for i, query := range queries {
		if errors[i] != nil {
			ui.Warning("failed to query %s packages: %v", query.Name, errors[i])
			packageLists[query.Name] = nil
			continue
		}

		switch query.Name {
		case "apt", "pip":
			packageLists[query.Name] = ParseLineList(results[i])
		case "npm", "pnpm":
			packageLists[query.Name] = ParseJSONPackageList(results[i])
		case "yarn":
			packageLists[query.Name] = ParseLineList(results[i])
		}
	}

	return packageLists, nil
}

func (pqe *PackageQueryExecutor) createQueryTask(command string) StringTask {
	return func() (string, error) {

		if pqe.execFunc != nil {
			ctx := context.Background()
			stdout, _, _, err := pqe.execFunc(ctx, pqe.islandName, []string{"bash", "-c", command}, false)
			if err != nil {
				return "", fmt.Errorf("query failed: %w", err)
			}
			return stdout, nil
		}

		cmd := exec.Command("docker", "exec", pqe.islandName, "bash", "-c", command)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("query failed: %w", err)
		}

		return stdout.String(), nil
	}
}

func ParseLineList(output string) []string {
	var result []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func ParseJSONPackageList(output string) []string {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	type npmDep struct {
		Version string `json:"version"`
	}
	var obj struct {
		Dependencies map[string]npmDep `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(output), &obj); err == nil && obj.Dependencies != nil {
		var res []string
		for name, dep := range obj.Dependencies {
			if dep.Version != "" {
				res = append(res, fmt.Sprintf("%s@%s", name, dep.Version))
			}
		}
		sort.Strings(res)
		return res
	}

	var arr []map[string]any
	if err := json.Unmarshal([]byte(output), &arr); err == nil {
		depSet := map[string]string{}
		for _, node := range arr {
			if deps, ok := node["dependencies"].(map[string]any); ok {
				for name, v := range deps {
					if m, ok := v.(map[string]any); ok {
						if ver, ok := m["version"].(string); ok && ver != "" {
							depSet[name] = ver
						}
					}
				}
			}
		}
		if len(depSet) > 0 {
			var res []string
			for name, ver := range depSet {
				res = append(res, fmt.Sprintf("%s@%s", name, ver))
			}
			sort.Strings(res)
			return res
		}
	}

	return nil
}
