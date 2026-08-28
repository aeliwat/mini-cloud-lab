package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"minicloud/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a local minicloud state file",
	Long:  "Creates state.json in the current directory to track simulated servers.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Default()
		if err != nil {
			return err
		}

		if err := s.Init(); err != nil {
			return err
		}

		fmt.Printf("Initialized minicloud at %s\n", s.Path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
