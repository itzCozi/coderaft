package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version = "1.0"

	CommitHash = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Display the version and build information for coderaft.`,
	Run: func(cmd *cobra.Command, args []string) {
		if CommitHash != "" && CommitHash != "unknown" {
			fmt.Printf("coderaft (v%s, commit %s)\n", Version, CommitHash)
		} else {
			fmt.Printf("coderaft (v%s)\n", Version)
		}
	},
}
