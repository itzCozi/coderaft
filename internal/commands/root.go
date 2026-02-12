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
)

var (
	configManager *config.ConfigManager
	dockerClient  *docker.Client
	forceFlag     bool
)

var rootCmd = &cobra.Command{
	Use:   "coderaft",
	Short: "Isolated development environments for anything",
	Long:  `coderaft creates isolated development environments, contained in a project's Docker island. Each project operates in its own disposable environment, while your code remains neatly organized in a simple, flat folder on the host machine.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			return fmt.Errorf("coderaft requires Docker and supports Linux, macOS, and Windows")
		}

		var err error
		configManager, err = config.NewConfigManager()
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		dockerClient, err = docker.NewClient()
		if err != nil {
			return fmt.Errorf("docker is not available. Please ensure Docker is installed and its daemon is running: %w", err)
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
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("failed to execute root command: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(maintenanceCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)

	destroyCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force operation without confirmation")
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
