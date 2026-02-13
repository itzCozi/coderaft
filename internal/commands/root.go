package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/spf13/cobra"

	"coderaft/internal/config"
	"coderaft/internal/docker"
	"coderaft/internal/ui"
)

var (
	configManager *config.ConfigManager
	dockerClient  DockerEngine
)

var rootCmd = &cobra.Command{
	Use:   "coderaft",
	Short: "Isolated development islands for anything",
	Long:  `coderaft creates isolated development islands, contained in a project's Docker island. Each project operates in its own disposable island, while your code remains neatly organized in a simple, flat folder on the host machine.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		switch runtime.GOOS {
		case "linux", "darwin", "windows":

		default:
			return fmt.Errorf("unsupported platform: %s. coderaft supports Linux, macOS, and Windows", runtime.GOOS)
		}

		switch cmd.Name() {
		case "version", "completion", "help":
			return nil
		}

		var err error
		configManager, err = config.NewConfigManager()
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		dockerClient, err = docker.NewClient()
		if err != nil {
			hint := ""
			switch runtime.GOOS {
			case "windows":
				hint = " Please ensure Docker Desktop is installed and running."
			case "darwin":
				hint = " Please ensure Docker Desktop for Mac is installed and running."
			default:
				hint = " Please ensure Docker is installed and its daemon is running."
			}
			return fmt.Errorf("docker is not available.%s\n  %w", hint, err)
		}

		if err := dockerClient.IsDockerAvailableWith(); err != nil {
			dockerClient.Close()
			dockerClient = nil
			return fmt.Errorf("docker availability check failed: %w", err)
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {

		if dockerClient != nil {
			dockerClient.Close()
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	rootCmd.PersistentFlags().BoolVar(&ui.Verbose, "verbose", false, "Show detailed progress messages")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statusCmd)

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(templatesCmd)
	rootCmd.AddCommand(devcontainerCmd)

	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)

	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(maintenanceCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
}

func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	matched, err := regexp.MatchString("^[a-zA-Z0-9_-]+$", name)
	if err != nil {
		return fmt.Errorf("error validating project name: %w", err)
	}

	if !matched {
		return fmt.Errorf("project name can only contain alphanumeric characters, hyphens, and underscores")
	}

	return nil
}

func getWorkspacePath(projectName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, "coderaft", projectName), nil
}
