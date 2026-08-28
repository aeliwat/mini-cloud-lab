package cmd

import (
	"github.com/spf13/cobra"
)

// serverCmd is the parent for server-related subcommands.
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage simulated servers",
	Long:  "Provision and inspect virtual servers in the local minicloud simulation.",
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
