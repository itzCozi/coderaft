package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"coderaft/internal/secrets"
	"coderaft/internal/ui"
)

var secretsVault *secrets.Vault

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage encrypted secrets for coderaft projects",
	Long: `Manage project secrets stored in an encrypted vault.

Secrets are encrypted locally using AES-256-GCM and injected into
your island as environment variables at runtime.

First-time setup:
  coderaft secrets init          # Create vault with master password

Daily usage:
  coderaft secrets set <project> <KEY>           # Set a secret (prompts for value)
  coderaft secrets set <project> <KEY>=<value>   # Set inline
  coderaft secrets get <project> <KEY>           # Retrieve a secret
  coderaft secrets list <project>                # List secret keys
  coderaft secrets remove <project> <KEY>        # Remove a secret
  coderaft secrets import <project> .env         # Import from .env file

Secrets are automatically injected when running 'coderaft up' or 'coderaft shell'.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "init" {
			return nil
		}

		var err error
		secretsVault, err = secrets.NewVault()
		if err != nil {
			return fmt.Errorf("failed to load secrets vault: %w", err)
		}

		if !secretsVault.IsInitialized() {
			return fmt.Errorf("secrets vault not initialized. Run 'coderaft secrets init' first")
		}

		password, err := promptPassword("Enter vault password: ")
		if err != nil {
			return err
		}

		if err := secretsVault.Unlock(password); err != nil {
			return fmt.Errorf("failed to unlock vault: %w", err)
		}

		return nil
	},
}

var secretsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the secrets vault with a master password",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		vault, err := secrets.NewVault()
		if err != nil {
			return err
		}

		if vault.IsInitialized() {
			ui.Warning("vault already initialized. Use 'coderaft secrets set' to add secrets.")
			return nil
		}

		ui.Info("initializing secrets vault...")
		ui.Info("choose a strong master password. this cannot be recovered if lost.")
		ui.Blank()

		password, err := promptPassword("Enter master password: ")
		if err != nil {
			return err
		}

		confirm, err := promptPassword("Confirm password: ")
		if err != nil {
			return err
		}

		if password != confirm {
			return fmt.Errorf("passwords do not match")
		}

		if len(password) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}

		if err := vault.Initialize(password); err != nil {
			return fmt.Errorf("failed to initialize vault: %w", err)
		}

		ui.Success("secrets vault initialized")
		ui.Info("use 'coderaft secrets set <project> <KEY>' to add secrets")
		return nil
	},
}

var secretsSetCmd = &cobra.Command{
	Use:   "set <project> <KEY>[=value]",
	Short: "Set a secret for a project",
	Long: `Set an encrypted secret for a project.

Examples:
  coderaft secrets set myproject API_KEY              # prompts for value
  coderaft secrets set myproject API_KEY=sk-12345    # inline value
  coderaft secrets set myproject DATABASE_URL        # prompts for value`,
	Args: cobra.RangeArgs(2, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		keyValue := args[1]

		var key, value string
		if strings.Contains(keyValue, "=") {
			parts := strings.SplitN(keyValue, "=", 2)
			key = parts[0]
			value = parts[1]
		} else {
			key = keyValue
			var err error
			value, err = promptPassword(fmt.Sprintf("Enter value for %s: ", key))
			if err != nil {
				return err
			}
		}

		if key == "" {
			return fmt.Errorf("secret key cannot be empty")
		}

		if err := secretsVault.Set(project, key, value); err != nil {
			return fmt.Errorf("failed to set secret: %w", err)
		}

		ui.Success("secret '%s' set for project '%s'", key, project)
		return nil
	},
}

var secretsGetCmd = &cobra.Command{
	Use:   "get <project> <KEY>",
	Short: "Get a secret value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		key := args[1]

		value, err := secretsVault.Get(project, key)
		if err != nil {
			return err
		}

		fmt.Println(value)
		return nil
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list [project]",
	Short: "List secrets (keys only, not values)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			projects := secretsVault.ListProjects()
			if len(projects) == 0 {
				ui.Info("no secrets stored.")
				return nil
			}
			ui.Header("projects with secrets")
			for _, p := range projects {
				keys := secretsVault.List(p)
				ui.Item("%s (%d secrets)", p, len(keys))
			}
			return nil
		}

		project := args[0]
		keys := secretsVault.List(project)
		if len(keys) == 0 {
			ui.Info("no secrets for project '%s'.", project)
			return nil
		}

		ui.Header("secrets for %s", project)
		for _, k := range keys {
			ui.Item(k)
		}
		return nil
	},
}

var secretsRemoveCmd = &cobra.Command{
	Use:   "remove <project> <KEY>",
	Short: "Remove a secret",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		key := args[1]

		if err := secretsVault.Remove(project, key); err != nil {
			return err
		}

		ui.Success("secret '%s' removed from project '%s'", key, project)
		return nil
	},
}

var secretsImportCmd = &cobra.Command{
	Use:   "import <project> <envfile>",
	Short: "Import secrets from a .env file",
	Long: `Import key-value pairs from a .env file into the encrypted vault.

Example:
  coderaft secrets import myproject .env
  coderaft secrets import myproject .env.production`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		envFile := args[1]

		env, err := secrets.LoadEnvFile(envFile)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", envFile, err)
		}

		if len(env) == 0 {
			ui.Info("no variables found in %s", envFile)
			return nil
		}

		imported := 0
		for k, v := range env {
			if err := secretsVault.Set(project, k, v); err != nil {
				ui.Warning("failed to import %s: %v", k, err)
				continue
			}
			imported++
		}

		ui.Success("imported %d secrets from %s into project '%s'", imported, envFile, project)
		return nil
	},
}

var secretsExportCmd = &cobra.Command{
	Use:   "export <project>",
	Short: "Export secrets as environment variables (for piping)",
	Long: `Export secrets in shell-compatible format.

Example:
  eval $(coderaft secrets export myproject)
  coderaft secrets export myproject > .env.local`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]

		secrets, err := secretsVault.GetAll(project)
		if err != nil {
			return err
		}

		if len(secrets) == 0 {
			return nil
		}

		for k, v := range secrets {
			// Escape special characters for shell
			escaped := strings.ReplaceAll(v, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			escaped = strings.ReplaceAll(escaped, `$`, `\$`)
			escaped = strings.ReplaceAll(escaped, "`", "\\`")
			fmt.Printf("export %s=\"%s\"\n", k, escaped)
		}
		return nil
	},
}

func init() {
	secretsCmd.AddCommand(secretsInitCmd)
	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsGetCmd)
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsRemoveCmd)
	secretsCmd.AddCommand(secretsImportCmd)
	secretsCmd.AddCommand(secretsExportCmd)
}

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Check if we're in a terminal
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		password, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return string(password), nil
	}

	// Fallback for non-terminal (piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return strings.TrimSpace(password), nil
}
