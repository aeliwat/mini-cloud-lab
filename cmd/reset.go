package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"minicloud/internal/store"
)

var resetForce bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete local state.json so you can run init again",
	Long: `Removes the minicloud state file in the current directory.

After reset, run:
  minicloud init`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Default()
		if err != nil {
			return err
		}

		if !s.Exists() {
			fmt.Println("Nothing to reset; state.json does not exist.")
			return nil
		}

		if !resetForce {
			fmt.Printf("This will delete %s\n", s.Path)
			fmt.Println("Re-run with --force to confirm:")
			fmt.Println("  minicloud reset --force")
			return nil
		}

		if err := os.Remove(s.Path); err != nil {
			return fmt.Errorf("delete state: %w", err)
		}
		fmt.Printf("Reset complete. Deleted %s\n", s.Path)
		fmt.Println("Next: minicloud init")
		return nil
	},
}

func init() {
	resetCmd.Flags().BoolVar(&resetForce, "force", false, "confirm deletion of state.json")
	rootCmd.AddCommand(resetCmd)
}
