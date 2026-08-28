package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the CLI version. Override at build time with:
//
//	go build -ldflags "-X minicloud/cmd.Version=1.0.0"
var Version = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the minicloud version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("minicloud version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
