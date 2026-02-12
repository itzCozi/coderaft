package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"coderaft/internal/config"
	"coderaft/internal/ui"
)

var configCmd = &cobra.Command{
	Use:   "config <command>",
	Short: "Manage coderaft configurations",
	Long: `Manage coderaft configurations including project-specific settings and global options.

Available commands:
  generate <project>    Generate coderaft.json for project
  validate <project>    Validate project configuration
	schema                Print JSON Schema for coderaft.json
  show <project>        Show project configuration
  templates             List available templates
  global               Show global configuration`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		subCommand := args[0]

		switch subCommand {
		case "generate":
			if len(args) < 2 {
				return fmt.Errorf("project name required for generate command")
			}
			return generateProjectConfig(args[1])
		case "validate":
			if len(args) < 2 {
				return fmt.Errorf("project name required for validate command")
			}
			return validateProjectConfig(args[1])
		case "schema":
			fmt.Println(config.ProjectConfigJSONSchema)
			return nil
		case "show":
			if len(args) < 2 {
				return fmt.Errorf("project name required for show command")
			}
			return showProjectConfig(args[1])
		case "templates":
			return showTemplates()
		case "global":
			return showGlobalConfig()
		default:
			return fmt.Errorf("unknown config command: %s", subCommand)
		}
	},
}

func generateProjectConfig(projectName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	project, exists := cfg.GetProject(projectName)
	if !exists {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	configPath := filepath.Join(project.WorkspacePath, "coderaft.json")
	if _, err := os.Stat(configPath); err == nil && !forceFlag {
		return fmt.Errorf("coderaft.json already exists. Use --force to overwrite")
	}

	projectConfig := configManager.GetDefaultProjectConfig(projectName)
	projectConfig.BaseImage = project.BaseImage

	if err := configManager.SaveProjectConfig(project.WorkspacePath, projectConfig); err != nil {
		return fmt.Errorf("failed to save project configuration: %w", err)
	}

	ui.Success("generated coderaft.json for project '%s'", projectName)
	ui.Detail("configuration file", configPath)
	ui.Blank()
	ui.Info("edit the file to customize your development environment.")
	ui.Info("available templates: %s", strings.Join(configManager.GetAvailableTemplates(), ", "))

	return nil
}

func validateProjectConfig(projectName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	project, exists := cfg.GetProject(projectName)
	if !exists {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	projectConfig, err := configManager.LoadProjectConfig(project.WorkspacePath)
	if err != nil {
		return fmt.Errorf("failed to load project configuration: %w", err)
	}

	if projectConfig == nil {
		ui.Info("no coderaft.json found for project '%s'", projectName)
		ui.Info("generate one with: coderaft config generate %s", projectName)
		return nil
	}

	if err := configManager.ValidateProjectConfig(projectConfig); err != nil {
		ui.Error("configuration validation failed:")
		ui.Item(err.Error())
		return fmt.Errorf("project config validation failed: %w", err)
	}

	ui.Success("configuration for project '%s' is valid", projectName)

	ui.Blank()
	ui.Header("configuration summary")
	ui.Detail("name", projectConfig.Name)
	ui.Detail("base image", projectConfig.BaseImage)

	if len(projectConfig.SetupCommands) > 0 {
		ui.Detail("setup commands", fmt.Sprintf("%d", len(projectConfig.SetupCommands)))
	}

	if len(projectConfig.Environment) > 0 {
		ui.Detail("environment variables", fmt.Sprintf("%d", len(projectConfig.Environment)))
	}

	if len(projectConfig.Ports) > 0 {
		ui.Detail("port mappings", fmt.Sprintf("%v", projectConfig.Ports))
	}

	if len(projectConfig.Volumes) > 0 {
		ui.Detail("volume mappings", fmt.Sprintf("%v", projectConfig.Volumes))
	}

	return nil
}

func showProjectConfig(projectName string) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	project, exists := cfg.GetProject(projectName)
	if !exists {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	projectConfig, err := configManager.LoadProjectConfig(project.WorkspacePath)
	if err != nil {
		return fmt.Errorf("failed to load project configuration: %w", err)
	}

	ui.Header("project '%s'", projectName)

	ui.Info("global project settings:")
	ui.Detail("name", project.Name)
	ui.Detail("box name", project.BoxName)
	ui.Detail("base image", project.BaseImage)
	ui.Detail("workspace", project.WorkspacePath)
	ui.Detail("status", project.Status)

	if projectConfig == nil {
		ui.Blank()
		ui.Info("no coderaft.json configuration file found.")
		ui.Info("generate one with: coderaft config generate %s", projectName)
		return nil
	}

	ui.Blank()
	ui.Info("project configuration (coderaft.json):")

	if projectConfig.BaseImage != "" && projectConfig.BaseImage != project.BaseImage {
		ui.Detail("base image override", projectConfig.BaseImage)
	}

	if len(projectConfig.SetupCommands) > 0 {
		ui.Info("setup commands:")
		for i, cmd := range projectConfig.SetupCommands {
			ui.Item("%d. %s", i+1, cmd)
		}
	}

	if len(projectConfig.Environment) > 0 {
		ui.Info("environment variables:")
		for key, value := range projectConfig.Environment {
			ui.Detail(key, value)
		}
	}

	if len(projectConfig.Ports) > 0 {
		ui.Info("port mappings:")
		for _, port := range projectConfig.Ports {
			ui.Item(port)
		}
	}

	if len(projectConfig.Volumes) > 0 {
		ui.Info("volume mappings:")
		for _, volume := range projectConfig.Volumes {
			ui.Item(volume)
		}
	}

	if projectConfig.WorkingDir != "" {
		ui.Detail("working directory", projectConfig.WorkingDir)
	}

	if projectConfig.Shell != "" {
		ui.Detail("shell", projectConfig.Shell)
	}

	if projectConfig.User != "" {
		ui.Detail("user", projectConfig.User)
	}

	if len(projectConfig.Capabilities) > 0 {
		ui.Detail("capabilities", fmt.Sprintf("%v", projectConfig.Capabilities))
	}

	if len(projectConfig.Labels) > 0 {
		ui.Info("labels:")
		for key, value := range projectConfig.Labels {
			ui.Detail(key, value)
		}
	}

	if projectConfig.Network != "" {
		ui.Detail("network", projectConfig.Network)
	}

	if projectConfig.Resources != nil {
		ui.Info("resource constraints:")
		if projectConfig.Resources.CPUs != "" {
			ui.Detail("cpus", projectConfig.Resources.CPUs)
		}
		if projectConfig.Resources.Memory != "" {
			ui.Detail("memory", projectConfig.Resources.Memory)
		}
	}

	if projectConfig.HealthCheck != nil {
		ui.Info("health check:")
		if len(projectConfig.HealthCheck.Test) > 0 {
			ui.Detail("test", fmt.Sprintf("%v", projectConfig.HealthCheck.Test))
		}
		if projectConfig.HealthCheck.Interval != "" {
			ui.Detail("interval", projectConfig.HealthCheck.Interval)
		}
		if projectConfig.HealthCheck.Timeout != "" {
			ui.Detail("timeout", projectConfig.HealthCheck.Timeout)
		}
		if projectConfig.HealthCheck.Retries > 0 {
			ui.Detail("retries", fmt.Sprintf("%d", projectConfig.HealthCheck.Retries))
		}
	}

	return nil
}

func showTemplates() error {
	templates := configManager.GetAvailableTemplates()

	ui.Header("available configuration templates")

	for _, templateName := range templates {
		templateConfig, err := configManager.CreateProjectConfigFromTemplate(templateName, "example")
		if err != nil {
			ui.Item("%s: error loading template", templateName)
			continue
		}

		ui.Blank()
		ui.Info("%s:", templateName)
		ui.Detail("base image", templateConfig.BaseImage)

		if len(templateConfig.SetupCommands) > 0 {
			ui.Detail("setup commands", fmt.Sprintf("%d steps", len(templateConfig.SetupCommands)))
		}

		if len(templateConfig.Environment) > 0 {
			ui.Detail("environment", fmt.Sprintf("%d variables", len(templateConfig.Environment)))
		}

		if len(templateConfig.Ports) > 0 {
			ui.Detail("ports", fmt.Sprintf("%v", templateConfig.Ports))
		}
	}

	ui.Blank()
	ui.Info("usage:")
	ui.Info("  coderaft init myproject --template python")
	ui.Info("  coderaft init myproject --template nodejs")

	return nil
}

func showGlobalConfig() error {
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ui.Header("global coderaft configuration")

	if cfg.Settings != nil {
		ui.Info("settings:")
		ui.Detail("default base image", cfg.Settings.DefaultBaseImage)
		ui.Detail("auto update", fmt.Sprintf("%t", cfg.Settings.AutoUpdate))
		ui.Detail("auto stop on exit", fmt.Sprintf("%t", cfg.Settings.AutoStopOnExit))

		if cfg.Settings.ConfigTemplatesPath != "" {
			ui.Detail("templates path", cfg.Settings.ConfigTemplatesPath)
		}

		if len(cfg.Settings.DefaultEnvironment) > 0 {
			ui.Info("default environment:")
			for key, value := range cfg.Settings.DefaultEnvironment {
				ui.Detail(key, value)
			}
		}
	}

	ui.Blank()
	ui.Info("projects: %d total", len(cfg.Projects))

	for name, project := range cfg.Projects {
		ui.Item("%s (%s)", name, project.Status)
	}

	return nil
}

func init() {
	configCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force operation, overwriting existing files")
}
