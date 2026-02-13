package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status [project]",
	Short: "Show detailed status for a coderaft project",
	Long:  "Displays island state, resource usage, uptime, ports, mounts, and other diagnostics for the project's island.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectName string
		if len(args) == 1 {
			projectName = args[0]
		} else {

			islands, err := dockerClient.ListIslands()
			if err != nil {
				return fmt.Errorf("failed to list islands: %w", err)
			}
			if len(islands) == 0 {
				ui.Info("no coderaft islands found.")
				return nil
			}
			ui.Header("coderaft islands")
			for _, b := range islands {
				name := ""
				if len(b.Names) > 0 {
					name = b.Names[0]
				}
				ui.Item("%s\t%s\t%s", name, b.Status, b.Image)
			}
			ui.Blank()
			ui.Info("tip: coderaft status <project> for detailed view.")
			return nil
		}

		if err := validateProjectName(projectName); err != nil {
			return fmt.Errorf("invalid project name: %w", err)
		}

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		project, ok := cfg.GetProject(projectName)
		if !ok {
			return fmt.Errorf("project '%s' not found", projectName)
		}

		island := project.IslandName
		if island == "" {
			island = fmt.Sprintf("coderaft_%s", projectName)
		}

		exists, err := dockerClient.IslandExists(island)
		if err != nil {
			return fmt.Errorf("failed to check if island exists: %w", err)
		}
		if !exists {
			ui.Detail("project", projectName)
			ui.Detail("island", fmt.Sprintf("%s (not found)", island))
			return nil
		}

		status, err := dockerClient.GetIslandStatus(island)
		if err != nil {
			return fmt.Errorf("failed to get island status: %w", err)
		}
		stats, _ := dockerClient.GetContainerStats(island)
		uptime, _ := dockerClient.GetUptime(island)
		ports, _ := dockerClient.GetPortMappings(island)
		mounts, _ := dockerClient.GetMounts(island)

		ui.Header("coderaft status")
		ui.Detail("project", projectName)
		ui.Detail("island", island)
		ui.Detail("image", project.BaseImage)
		ui.Detail("state", status)
		if uptime > 0 {
			ui.Detail("uptime", humanizeDuration(uptime))
		} else {
			ui.Detail("uptime", "-")
		}
		if stats != nil {
			ui.Detail("cpu", stats.CPUPercent)
			ui.Detail("memory", fmt.Sprintf("%s (%s)", stats.MemUsage, stats.MemPercent))
			if stats.NetIO != "" {
				ui.Detail("net i/o", stats.NetIO)
			}
			if stats.BlockIO != "" {
				ui.Detail("block i/o", stats.BlockIO)
			}
			if stats.PIDs != "" {
				ui.Detail("pids", stats.PIDs)
			}
		}
		if len(ports) > 0 {
			ui.Detail("ports", strings.Join(ports, ", "))
		} else {
			ui.Detail("ports", "-")
		}
		if len(mounts) > 0 {
			ui.Detail("mounts", strings.Join(mounts, ", "))
		}

		if pcfg, err := configManager.LoadProjectConfig(project.WorkspacePath); err == nil && pcfg != nil && pcfg.HealthCheck != nil {
			if len(pcfg.HealthCheck.Test) > 0 {
				ui.Detail("health check", strings.Join(pcfg.HealthCheck.Test, " "))
			}
			if pcfg.HealthCheck.Interval != "" {
				ui.Detail("health interval", pcfg.HealthCheck.Interval)
			}
		}

		return nil
	},
}

func humanizeDuration(d time.Duration) string {
	d = d.Round(time.Second)
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, mins, secs)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func init() {
}
