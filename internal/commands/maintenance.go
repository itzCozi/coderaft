package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var (
	updateFlag      bool
	healthCheckFlag bool
	rebuildFlag     bool
	restartFlag     bool
	statusCheckFlag bool
	autoRepairFlag  bool
)

var maintenanceCmd = &cobra.Command{
	Use:   "maintenance [flags]",
	Short: "Perform maintenance tasks on coderaft projects and boxes",
	Long: `Perform various maintenance tasks to keep your coderaft environment healthy:

- Update system packages in boxes
- Check health status of all projects
- Rebuild boxes from latest base images
- Restart stopped or problematic boxes
- Auto-repair common issues
- System status checks

Examples:
  coderaft maintenance                     # Interactive maintenance menu
  coderaft maintenance --update            # Update all boxes
  coderaft maintenance --health-check      # Check health of all projects
  coderaft maintenance --restart           # Restart all stopped boxes
  coderaft maintenance --rebuild           # Rebuild all boxes
  coderaft maintenance --status            # Show detailed status
  coderaft maintenance --auto-repair       # Auto-fix common issues`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		if !updateFlag && !healthCheckFlag && !rebuildFlag && !restartFlag && !statusCheckFlag && !autoRepairFlag {
			return runInteractiveMaintenance()
		}

		var maintenanceTasks []func() error

		if statusCheckFlag {
			maintenanceTasks = append(maintenanceTasks, performStatusCheck)
		}

		if healthCheckFlag {
			maintenanceTasks = append(maintenanceTasks, performHealthCheck)
		}

		if updateFlag {
			maintenanceTasks = append(maintenanceTasks, updateAllboxes)
		}

		if restartFlag {
			maintenanceTasks = append(maintenanceTasks, restartStoppedboxes)
		}

		if rebuildFlag {
			maintenanceTasks = append(maintenanceTasks, rebuildAllboxes)
		}

		if autoRepairFlag {
			maintenanceTasks = append(maintenanceTasks, autoRepairIssues)
		}

		for _, task := range maintenanceTasks {
			if err := task(); err != nil {
				return err
			}
		}

		if len(maintenanceTasks) > 0 {
			ui.Blank()
			ui.Success("maintenance completed")
		}

		return nil
	},
}

func runInteractiveMaintenance() error {
	ui.Header("Coderaft Maintenance")
	ui.Blank()
	ui.Info("Available options:")
	ui.Info("  1. Check system status")
	ui.Info("  2. Health check all projects")
	ui.Info("  3. Update system packages in all boxes")
	ui.Info("  4. Restart stopped boxes")
	ui.Info("  5. Rebuild all boxes from latest images")
	ui.Info("  6. Auto-repair common issues")
	ui.Info("  7. Full maintenance (2-4)")
	ui.Info("  q. Quit")
	ui.Blank()

	reader := bufio.NewReader(os.Stdin)

	for {
		ui.Prompt("Select an option [1-7, q]: ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		switch response {
		case "1":
			return performStatusCheck()
		case "2":
			return performHealthCheck()
		case "3":
			return updateAllboxes()
		case "4":
			return restartStoppedboxes()
		case "5":
			return rebuildAllboxes()
		case "6":
			return autoRepairIssues()
		case "7":
			ui.Blank()
			ui.Status("running full maintenance...")
			tasks := []func() error{
				performHealthCheck,
				updateAllboxes,
				restartStoppedboxes,
			}
			for _, task := range tasks {
				if err := task(); err != nil {
					return err
				}
			}
			ui.Blank()
			ui.Success("full maintenance completed")
			return nil
		case "q", "quit", "exit":
			ui.Info("maintenance cancelled.")
			return nil
		default:
			ui.Info("invalid option. please select 1-7 or q.")
		}
	}
}

func performStatusCheck() error {
	ui.Header("System Status")
	ui.Blank()

	ui.Status("checking docker...")
	if err := dockerClient.RunDockerCommand([]string{"version", "--format", "Server: {{.Server.Version}}"}); err != nil {
		ui.Error("docker not available: %v", err)
		return fmt.Errorf("docker is not available: %w", err)
	}

	cfg, err := configManager.Load()
	if err != nil {
		ui.Error("failed to load config: %v", err)
		return fmt.Errorf("failed to load config: %w", err)
	}

	projects := cfg.GetProjects()
	ui.Blank()
	ui.Info("projects: %d total", len(projects))

	boxes, err := dockerClient.ListBoxes()
	if err != nil {
		ui.Error("failed to list boxes: %v", err)
		return fmt.Errorf("failed to list docker boxes: %w", err)
	}

	boxStatus := make(map[string]string)
	for _, box := range boxes {
		for _, name := range box.Names {
			cleanName := strings.TrimPrefix(name, "/")
			boxStatus[cleanName] = box.Status
		}
	}

	var running, stopped, missing int
	ui.Blank()
	ui.Info("box status:")
	for projectName, project := range projects {
		status := boxStatus[project.BoxName]
		if status == "" {
			ui.Item("%s -> %s (missing)", projectName, project.BoxName)
			missing++
		} else if strings.Contains(status, "Up") {
			ui.Item("%s -> %s (running)", projectName, project.BoxName)
			running++
		} else {
			ui.Item("%s -> %s (stopped)", projectName, project.BoxName)
			stopped++
		}
	}

	ui.Blank()
	ui.Summary("%d running, %d stopped, %d missing", running, stopped, missing)

	ui.Blank()
	ui.Info("docker disk usage:")
	if err := dockerClient.RunDockerCommand([]string{"system", "df"}); err != nil {
		ui.Error("failed to get disk usage: %v", err)
	}

	return nil
}

func performHealthCheck() error {
	ui.Status("scanning all coderaft projects...")

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	projects := cfg.GetProjects()
	if len(projects) == 0 {
		ui.Info("no projects to check.")
		return nil
	}

	boxes, err := dockerClient.ListBoxes()
	if err != nil {
		return fmt.Errorf("failed to list boxes: %w", err)
	}

	boxStatus := make(map[string]string)
	for _, box := range boxes {
		for _, name := range box.Names {
			cleanName := strings.TrimPrefix(name, "/")
			boxStatus[cleanName] = box.Status
		}
	}

	var healthy, unhealthy, missing int

	ui.Blank()
	ui.Info("Health Report:")

	for projectName, project := range projects {
		status := boxStatus[project.BoxName]
		if status == "" {
			ui.Item("%s: box missing", projectName)
			missing++
			continue
		}

		if !strings.Contains(status, "Up") {
			ui.Item("%s: box stopped (%s)", projectName, status)
			unhealthy++
			continue
		}

		if _, err := os.Stat(project.WorkspacePath); os.IsNotExist(err) {
			ui.Item("%s: workspace directory missing", projectName)
			unhealthy++
			continue
		}

		if err := dockerClient.RunDockerCommand([]string{"exec", project.BoxName, "echo", "health-check"}); err != nil {
			ui.Item("%s: box not responsive", projectName)
			unhealthy++
			continue
		}

		ui.Item("%s: healthy", projectName)
		healthy++
	}

	ui.Blank()
	ui.Summary("%d healthy, %d unhealthy, %d missing", healthy, unhealthy, missing)

	if unhealthy > 0 || missing > 0 {
		ui.Blank()
		ui.Info("hint: run 'coderaft maintenance --auto-repair' to fix common issues")
	}

	return nil
}

func updateAllboxes() error {
	ui.Status("updating system packages in all boxes...")

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	projects := cfg.GetProjects()
	if len(projects) == 0 {
		ui.Info("no projects to update.")
		return nil
	}

	var updated, failed int

	for projectName, project := range projects {
		ui.Blank()
		ui.Status("updating %s...", projectName)

		status, err := dockerClient.GetBoxStatus(project.BoxName)
		if err != nil {
			ui.Error("failed to check status for %s: %v", projectName, err)
			failed++
			continue
		}

		if status == "not found" {
			ui.Warning("box %s not found, skipping", project.BoxName)
			continue
		}

		if status != "running" {
			ui.Status("starting %s...", project.BoxName)
			if err := dockerClient.StartBox(project.BoxName); err != nil {
				ui.Error("failed to start %s: %v", project.BoxName, err)
				failed++
				continue
			}

			time.Sleep(2 * time.Second)
		}

		updateCommands := []string{
			"apt update -y",
			"apt full-upgrade -y",
			"apt autoremove -y",
			"apt autoclean",
		}

		if err := dockerClient.ExecuteSetupCommandsWithOutput(project.BoxName, updateCommands, false); err != nil {
			ui.Error("failed to update %s: %v", projectName, err)
			failed++
		} else {
			ui.Success("%s updated", projectName)
			updated++
		}
	}

	ui.Blank()
	ui.Summary("%d updated, %d failed", updated, failed)
	if failed > 0 {
		return fmt.Errorf("failed to update %d box(s)", failed)
	}

	return nil
}

func restartStoppedboxes() error {
	ui.Status("restarting stopped boxes...")

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	projects := cfg.GetProjects()
	if len(projects) == 0 {
		ui.Info("no projects to restart.")
		return nil
	}

	var restarted, failed int

	for projectName, project := range projects {
		status, err := dockerClient.GetBoxStatus(project.BoxName)
		if err != nil {
			ui.Error("failed to check status for %s: %v", projectName, err)
			failed++
			continue
		}

		if status == "not found" {
			ui.Warning("box %s not found, skipping", project.BoxName)
			continue
		}

		if status != "running" {
			ui.Status("starting %s...", projectName)
			if err := dockerClient.StartBox(project.BoxName); err != nil {
				ui.Error("failed to start %s: %v", projectName, err)
				failed++
			} else {
				ui.Success("%s started", projectName)
				restarted++
			}
		} else {
			ui.Info("%s already running", projectName)
		}
	}

	ui.Blank()
	ui.Summary("%d restarted, %d failed", restarted, failed)
	if failed > 0 {
		return fmt.Errorf("failed to restart %d box(s)", failed)
	}

	return nil
}

func rebuildAllboxes() error {
	ui.Status("rebuilding all boxes from latest images...")

	if !forceFlag {
		ui.Prompt("This will destroy and recreate all boxes. Continue? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			ui.Info("rebuild cancelled.")
			return nil
		}
	}

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	projects := cfg.GetProjects()
	if len(projects) == 0 {
		ui.Info("no projects to rebuild.")
		return nil
	}

	var rebuilt, failed int

	for projectName, project := range projects {
		ui.Blank()
		ui.Status("rebuilding %s...", projectName)

		if exists, err := dockerClient.BoxExists(project.BoxName); err != nil {
			ui.Error("failed to check if %s exists: %v", project.BoxName, err)
			failed++
			continue
		} else if exists {
			ui.Status("stopping and removing existing box...")
			dockerClient.StopBox(project.BoxName)
			if err := dockerClient.RemoveBox(project.BoxName); err != nil {
				ui.Error("failed to remove %s: %v", project.BoxName, err)
				failed++
				continue
			}
		}

		ui.Status("recreating box...")

		projectConfig, err := configManager.LoadProjectConfig(project.WorkspacePath)
		if err != nil {
			ui.Warning("could not load project config: %v", err)
		}

		baseImage := cfg.GetEffectiveBaseImage(project, projectConfig)
		if err := dockerClient.PullImage(baseImage); err != nil {
			ui.Error("failed to pull %s: %v", baseImage, err)
			failed++
			continue
		}

		workspaceBox := "/workspace"
		if projectConfig != nil && projectConfig.WorkingDir != "" {
			workspaceBox = projectConfig.WorkingDir
		}

		boxID, err := dockerClient.CreateBox(project.BoxName, baseImage, project.WorkspacePath, workspaceBox)
		if err != nil {
			ui.Error("failed to create %s: %v", project.BoxName, err)
			failed++
			continue
		}

		if err := dockerClient.StartBox(boxID); err != nil {
			ui.Error("failed to start %s: %v", project.BoxName, err)
			failed++
			continue
		}

		if err := dockerClient.WaitForBox(project.BoxName, 30*time.Second); err != nil {
			ui.Error("box %s failed to start: %v", project.BoxName, err)
			failed++
			continue
		}

		updateCommands := []string{
			"apt update -y",
			"apt full-upgrade -y",
		}
		if err := dockerClient.ExecuteSetupCommandsWithOutput(project.BoxName, updateCommands, false); err != nil {
			ui.Warning("failed to update system packages: %v", err)
		}

		if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
			if err := dockerClient.ExecuteSetupCommandsWithOutput(project.BoxName, projectConfig.SetupCommands, false); err != nil {
				ui.Warning("failed to execute setup commands: %v", err)
			}
		}

		if err := dockerClient.SetupCoderaftInBoxWithUpdate(project.BoxName, projectName); err != nil {
			ui.Warning("failed to setup coderaft environment: %v", err)
		}

		ui.Success("%s rebuilt", projectName)
		rebuilt++
	}

	ui.Blank()
	ui.Summary("%d rebuilt, %d failed", rebuilt, failed)
	if failed > 0 {
		return fmt.Errorf("failed to rebuild %d box(s)", failed)
	}

	return nil
}

func autoRepairIssues() error {
	ui.Status("auto-repairing common issues...")

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	projects := cfg.GetProjects()
	if len(projects) == 0 {
		ui.Info("no projects to repair.")
		return nil
	}

	var repaired, failed int

	for projectName, project := range projects {
		ui.Blank()
		ui.Status("checking %s...", projectName)

		issuesFound := false

		if _, err := os.Stat(project.WorkspacePath); os.IsNotExist(err) {
			ui.Status("creating missing workspace directory...")
			if err := os.MkdirAll(project.WorkspacePath, 0755); err != nil {
				ui.Error("failed to create workspace: %v", err)
				failed++
				continue
			}
			issuesFound = true
		}

		status, err := dockerClient.GetBoxStatus(project.BoxName)
		if err != nil {
			ui.Error("failed to check box status: %v", err)
			failed++
			continue
		}

		if status == "not found" {
			ui.Status("recreating missing box...")

			projectConfig, _ := configManager.LoadProjectConfig(project.WorkspacePath)
			baseImage := cfg.GetEffectiveBaseImage(project, projectConfig)

			workspaceBox := "/workspace"
			if projectConfig != nil && projectConfig.WorkingDir != "" {
				workspaceBox = projectConfig.WorkingDir
			}

			boxID, err := dockerClient.CreateBox(project.BoxName, baseImage, project.WorkspacePath, workspaceBox)
			if err != nil {
				ui.Error("failed to recreate box: %v", err)
				failed++
				continue
			}

			if err := dockerClient.StartBox(boxID); err != nil {
				ui.Error("failed to start box: %v", err)
				failed++
				continue
			}

			if err := dockerClient.SetupCoderaftInBoxWithUpdate(project.BoxName, projectName); err != nil {
				ui.Warning("failed to setup coderaft environment: %v", err)
			}

			issuesFound = true
		} else if status != "running" {
			ui.Status("starting stopped box...")
			if err := dockerClient.StartBox(project.BoxName); err != nil {
				ui.Error("failed to start box: %v", err)
				failed++
				continue
			}
			issuesFound = true
		}

		if status != "not found" {
			if err := dockerClient.RunDockerCommand([]string{"exec", project.BoxName, "echo", "test"}); err != nil {
				ui.Status("box unresponsive, restarting...")
				dockerClient.StopBox(project.BoxName)
				if err := dockerClient.StartBox(project.BoxName); err != nil {
					ui.Error("failed to restart box: %v", err)
					failed++
					continue
				}
				issuesFound = true
			}
		}

		if issuesFound {
			ui.Success("%s repaired", projectName)
			repaired++
		} else {
			ui.Info("%s is healthy", projectName)
		}
	}

	ui.Blank()
	ui.Summary("%d repaired, %d failed", repaired, failed)
	if failed > 0 {
		return fmt.Errorf("failed to repair %d project(s)", failed)
	}

	return nil
}

func init() {
	maintenanceCmd.Flags().BoolVar(&updateFlag, "update", false, "Update system packages in all boxes")
	maintenanceCmd.Flags().BoolVar(&healthCheckFlag, "health-check", false, "Perform health check on all projects")
	maintenanceCmd.Flags().BoolVar(&rebuildFlag, "rebuild", false, "Rebuild all boxes from latest base images")
	maintenanceCmd.Flags().BoolVar(&restartFlag, "restart", false, "Restart stopped boxes")
	maintenanceCmd.Flags().BoolVar(&statusCheckFlag, "status", false, "Show detailed system status")
	maintenanceCmd.Flags().BoolVar(&autoRepairFlag, "auto-repair", false, "Automatically repair common issues")
	maintenanceCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force operations without confirmation prompts")
}
