package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"devbox/internal/config"
	"devbox/internal/ui"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage devbox project templates",
}

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		names := configManager.GetAvailableTemplates()
		if len(names) == 0 {
			ui.Info("no templates available.")
			return nil
		}
		ui.Header("available templates")
		for _, n := range names {
			ui.Item(n)
		}
		return nil
	},
}

var templatesShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show template JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		tplCfg, err := configManager.CreateProjectConfigFromTemplate(name, "example")
		if err != nil {
			return fmt.Errorf("template '%s' not found", name)
		}
		out := config.ConfigTemplate{Name: name, Description: "", Config: *tplCfg}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

var templatesCreateCmd = &cobra.Command{
	Use:   "create <name> [project]",
	Short: "Create devbox.json from a template in the current directory",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		project := ""
		if len(args) == 2 {
			project = args[1]
		}
		if project == "" {

			wd, _ := os.Getwd()
			project = filepath.Base(wd)
		}
		cfg, err := configManager.CreateProjectConfigFromTemplate(name, project)
		if err != nil {
			return fmt.Errorf("failed to create project config from template: %w", err)
		}
		wd, _ := os.Getwd()
		if err := configManager.SaveProjectConfig(wd, cfg); err != nil {
			return fmt.Errorf("failed to save project config: %w", err)
		}
		ui.Success("generated devbox.json from template '%s' for project '%s'", name, project)
		return nil
	},
}

var templatesSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save current devbox.json as a reusable template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		wd, _ := os.Getwd()
		pc, err := configManager.LoadProjectConfig(wd)
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}
		if pc == nil {
			return fmt.Errorf("no devbox.json found in %s", wd)
		}
		tpl := &config.ConfigTemplate{Name: name, Description: fmt.Sprintf("Saved from %s", filepath.Base(wd)), Config: *pc}
		if err := configManager.SaveUserTemplate(tpl); err != nil {
			return fmt.Errorf("failed to save user template: %w", err)
		}
		ui.Success("saved template '%s'", name)
		return nil
	},
}

var templatesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a user template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := configManager.DeleteUserTemplate(name); err != nil {
			return fmt.Errorf("failed to delete user template: %w", err)
		}
		ui.Success("deleted template '%s'", name)
		return nil
	},
}

func init() {
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesShowCmd)
	templatesCmd.AddCommand(templatesCreateCmd)
	templatesCmd.AddCommand(templatesSaveCmd)
	templatesCmd.AddCommand(templatesDeleteCmd)
	rootCmd.AddCommand(templatesCmd)
}
