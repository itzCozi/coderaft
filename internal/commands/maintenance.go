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
	updateFlag       bool
	healthCheckFlag  bool
	rebuildFlag      bool
	restartFlag      bool
	statusCheckFlag  bool
	autoRepairFlag   bool
	maintenanceForce bool
)

var maintenanceCmd = &cobra.Command{
	Use:   "maintenance [flags]",
	Short: "Perform maintenance tasks on coderaft projects and islands",
	Long: `Perform various maintenance tasks to keep your coderaft islands healthy:

- Update system packages in islands
- Check health status of all projects
- Rebuild islands from latest base images
- Restart stopped or problematic islands
- Auto-repair common issues
- System status checks

Examples:
  coderaft maintenance                     # Interactive maintenance menu
  coderaft maintenance --update            # Update all islands
  coderaft maintenance --health-check      # Check health of all projects
  coderaft maintenance --restart           # Restart all stopped islands
  coderaft maintenance --rebuild           # Rebuild all islands
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
			maintenanceTasks = append(maintenanceTasks, updateAllislands)
		}

		if restartFlag {
			maintenanceTasks = append(maintenanceTasks, restartStoppedislands)
		}

		if rebuildFlag {
			maintenanceTasks = append(maintenanceTasks, rebuildAllislands)
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
	ui.Info("  3. Update system packages in all islands")
	ui.Info("  4. Restart stopped islands")
	ui.Info("  5. Rebuild all islands from latest images")
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
			return updateAllislands()
		case "4":
			return restartStoppedislands()
		case "5":
			return rebuildAllislands()
		case "6":
			return autoRepairIssues()
		case "7":
			ui.Blank()
			ui.Status("running full maintenance...")
			tasks := []func() error{
				performHealthCheck,
				updateAllislands,
				restartStoppedislands,
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

	islands, err := dockerClient.ListIslands()
	if err != nil {
		ui.Error("failed to list islands: %v", err)
		return fmt.Errorf("failed to list docker islands: %w", err)
	}

	islandStatus := make(map[string]string)
	for _, island := range islands {
		for _, name := range island.Names {
			cleanName := strings.TrimPrefix(name, "/")
			islandStatus[cleanName] = island.Status
		}
	}

	var running, stopped, missing int
	ui.Blank()
	ui.Info("island status:")
	for projectName, project := range projects {
		status := islandStatus[project.IslandName]
		if status == "" {
			ui.Item("%s -> %s (missing)", projectName, project.IslandName)
			missing++
		} else if strings.Contains(status, "Up") {
			ui.Item("%s -> %s (running)", projectName, project.IslandName)
			running++
		} else {
			ui.Item("%s -> %s (stopped)", projectName, project.IslandName)
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

	islands, err := dockerClient.ListIslands()
	if err != nil {
		return fmt.Errorf("failed to list islands: %w", err)
	}

	islandStatus := make(map[string]string)
	for _, island := range islands {
		for _, name := range island.Names {
			cleanName := strings.TrimPrefix(name, "/")
			islandStatus[cleanName] = island.Status
		}
	}

	var healthy, unhealthy, missing int

	ui.Blank()
	ui.Info("Health Report:")

	for projectName, project := range projects {
		status := islandStatus[project.IslandName]
		if status == "" {
			ui.Item("%s: island missing", projectName)
			missing++
			continue
		}

		if !strings.Contains(status, "Up") {
			ui.Item("%s: island stopped (%s)", projectName, status)
			unhealthy++
			continue
		}

		if _, err := os.Stat(project.WorkspacePath); os.IsNotExist(err) {
			ui.Item("%s: workspace directory missing", projectName)
			unhealthy++
			continue
		}

		if err := dockerClient.RunDockerCommand([]string{"exec", project.IslandName, "echo", "health-check"}); err != nil {
			ui.Item("%s: island not responsive", projectName)
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

func updateAllislands() error {
	ui.Status("updating system packages in all islands...")

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

		status, err := dockerClient.GetIslandStatus(project.IslandName)
		if err != nil {
			ui.Error("failed to check status for %s: %v", projectName, err)
			failed++
			continue
		}

		if status == "not found" {
			ui.Warning("island %s not found, skipping", project.IslandName)
			continue
		}

		if status != "running" {
			ui.Status("starting %s...", project.IslandName)
			if err := dockerClient.StartIsland(project.IslandName); err != nil {
				ui.Error("failed to start %s: %v", project.IslandName, err)
				failed++
				continue
			}

			time.Sleep(2 * time.Second)
		}

		updateCommands := []string{
			"apt update -y",
			"DEBIAN_FRONTEND=noninteractive apt full-upgrade -y",
			"apt autoremove -y",
			"apt autoclean",
		}

		if err := dockerClient.ExecuteSetupCommandsWithOutput(project.IslandName, updateCommands, false); err != nil {
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
		return fmt.Errorf("failed to update %d island(s)", failed)
	}

	return nil
}

func restartStoppedislands() error {
	ui.Status("restarting stopped islands...")

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
		status, err := dockerClient.GetIslandStatus(project.IslandName)
		if err != nil {
			ui.Error("failed to check status for %s: %v", projectName, err)
			failed++
			continue
		}

		if status == "not found" {
			ui.Warning("island %s not found, skipping", project.IslandName)
			continue
		}

		if status != "running" {
			ui.Status("starting %s...", projectName)
			if err := dockerClient.StartIsland(project.IslandName); err != nil {
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
		return fmt.Errorf("failed to restart %d island(s)", failed)
	}

	return nil
}

func rebuildAllislands() error {
	ui.Status("rebuilding all islands from latest images...")

	if !maintenanceForce {
		ui.Prompt("This will destroy and recreate all islands. Continue? (y/N): ")
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

		if exists, err := dockerClient.IslandExists(project.IslandName); err != nil {
			ui.Error("failed to check if %s exists: %v", project.IslandName, err)
			failed++
			continue
		} else if exists {
			ui.Status("stopping and removing existing island...")
			dockerClient.StopIsland(project.IslandName)
			if err := dockerClient.RemoveIsland(project.IslandName); err != nil {
				ui.Error("failed to remove %s: %v", project.IslandName, err)
				failed++
				continue
			}
		}

		ui.Status("recreating island...")

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

		workspaceIsland := "/island"
		if projectConfig != nil && projectConfig.WorkingDir != "" {
			workspaceIsland = projectConfig.WorkingDir
		}

		islandID, err := dockerClient.CreateIsland(project.IslandName, baseImage, project.WorkspacePath, workspaceIsland)
		if err != nil {
			ui.Error("failed to create %s: %v", project.IslandName, err)
			failed++
			continue
		}

		if err := dockerClient.StartIsland(islandID); err != nil {
			ui.Error("failed to start %s: %v", project.IslandName, err)
			failed++
			continue
		}

		if err := dockerClient.WaitForIsland(project.IslandName, 30*time.Second); err != nil {
			ui.Error("island %s failed to start: %v", project.IslandName, err)
			failed++
			continue
		}

		updateCommands := []string{
			"apt update -y",
			"DEBIAN_FRONTEND=noninteractive apt full-upgrade -y",
		}
		if err := dockerClient.ExecuteSetupCommandsWithOutput(project.IslandName, updateCommands, false); err != nil {
			ui.Warning("failed to update system packages: %v", err)
		}

		if projectConfig != nil && len(projectConfig.SetupCommands) > 0 {
			if err := dockerClient.ExecuteSetupCommandsWithOutput(project.IslandName, projectConfig.SetupCommands, false); err != nil {
				ui.Warning("failed to execute setup commands: %v", err)
			}
		}

		if err := dockerClient.SetupCoderaftOnIslandWithUpdate(project.IslandName, projectName); err != nil {
			ui.Warning("failed to setup coderaft on island: %v", err)
		}

		ui.Success("%s rebuilt", projectName)
		rebuilt++
	}

	ui.Blank()
	ui.Summary("%d rebuilt, %d failed", rebuilt, failed)
	if failed > 0 {
		return fmt.Errorf("failed to rebuild %d island(s)", failed)
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

		status, err := dockerClient.GetIslandStatus(project.IslandName)
		if err != nil {
			ui.Error("failed to check island status: %v", err)
			failed++
			continue
		}

		if status == "not found" {
			ui.Status("recreating missing island...")

			projectConfig, _ := configManager.LoadProjectConfig(project.WorkspacePath)
			baseImage := cfg.GetEffectiveBaseImage(project, projectConfig)

			workspaceIsland := "/island"
			if projectConfig != nil && projectConfig.WorkingDir != "" {
				workspaceIsland = projectConfig.WorkingDir
			}

			islandID, err := dockerClient.CreateIsland(project.IslandName, baseImage, project.WorkspacePath, workspaceIsland)
			if err != nil {
				ui.Error("failed to recreate island: %v", err)
				failed++
				continue
			}

			if err := dockerClient.StartIsland(islandID); err != nil {
				ui.Error("failed to start island: %v", err)
				failed++
				continue
			}

			if err := dockerClient.SetupCoderaftOnIslandWithUpdate(project.IslandName, projectName); err != nil {
				ui.Warning("failed to setup coderaft on island: %v", err)
			}

			issuesFound = true
		} else if status != "running" {
			ui.Status("starting stopped island...")
			if err := dockerClient.StartIsland(project.IslandName); err != nil {
				ui.Error("failed to start island: %v", err)
				failed++
				continue
			}
			issuesFound = true
		}

		if status != "not found" {
			if err := dockerClient.RunDockerCommand([]string{"exec", project.IslandName, "echo", "test"}); err != nil {
				ui.Status("island unresponsive, restarting...")
				dockerClient.StopIsland(project.IslandName)
				if err := dockerClient.StartIsland(project.IslandName); err != nil {
					ui.Error("failed to restart island: %v", err)
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
	maintenanceCmd.Flags().BoolVar(&updateFlag, "update", false, "Update system packages in all islands")
	maintenanceCmd.Flags().BoolVar(&healthCheckFlag, "health-check", false, "Perform health check on all projects")
	maintenanceCmd.Flags().BoolVar(&rebuildFlag, "rebuild", false, "Rebuild all islands from latest base images")
	maintenanceCmd.Flags().BoolVar(&restartFlag, "restart", false, "Restart stopped islands")
	maintenanceCmd.Flags().BoolVar(&statusCheckFlag, "status", false, "Show detailed system status")
	maintenanceCmd.Flags().BoolVar(&autoRepairFlag, "auto-repair", false, "Automatically repair common issues")
	maintenanceCmd.Flags().BoolVarP(&maintenanceForce, "force", "f", false, "Force operations without confirmation prompts")
}
