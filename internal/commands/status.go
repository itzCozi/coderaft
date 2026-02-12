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
	Long:  "Displays container state, resource usage, uptime, ports, mounts, and other diagnostics for the project's box.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectName string
		if len(args) == 1 {
			projectName = args[0]
		} else {

			boxes, err := dockerClient.ListBoxes()
			if err != nil {
				return fmt.Errorf("failed to list boxes: %w", err)
			}
			if len(boxes) == 0 {
				ui.Info("no coderaft containers found.")
				return nil
			}
			ui.Header("coderaft containers")
			for _, b := range boxes {
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

		box := project.BoxName
		if box == "" {
			box = fmt.Sprintf("coderaft_%s", projectName)
		}

		exists, err := dockerClient.BoxExists(box)
		if err != nil {
			return fmt.Errorf("failed to check if box exists: %w", err)
		}
		if !exists {
			ui.Detail("project", projectName)
			ui.Detail("box", fmt.Sprintf("%s (not found)", box))
			return nil
		}

		status, err := dockerClient.GetBoxStatus(box)
		if err != nil {
			return fmt.Errorf("failed to get box status: %w", err)
		}
		stats, _ := dockerClient.GetContainerStats(box)
		uptime, _ := dockerClient.GetUptime(box)
		ports, _ := dockerClient.GetPortMappings(box)
		mounts, _ := dockerClient.GetMounts(box)

		ui.Header("coderaft status")
		ui.Detail("project", projectName)
		ui.Detail("box", box)
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
	rootCmd.AddCommand(statusCmd)
}
