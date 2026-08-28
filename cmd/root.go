package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "minicloud",
	Short: "A local system design simulator CLI",
	Long: `minicloud is a local System Design Simulator CLI.

Provision virtual architecture (web servers, databases) locally
to practice system design, load balancing, and resource constraint emulation.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
