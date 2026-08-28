package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"minicloud/internal/store"
	"minicloud/internal/ui"
)

var uiPort string

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Open the animated topology simulator in the browser",
	Long: `Starts a local web UI that visualizes provisioned servers and
animates simulated request traffic.

Run this from the same directory as your state.json (after minicloud init).

Example:
  minicloud ui
  minicloud ui --port 7474`,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.Default()
		if err != nil {
			return err
		}
		if !st.Exists() {
			return fmt.Errorf("%w", store.ErrNotInitialized)
		}

		addr := ui.ParseAddr(uiPort)
		srv := ui.New(addr, st)
		return srv.ListenAndServe()
	},
}

func init() {
	uiCmd.Flags().StringVar(&uiPort, "port", "7474", "HTTP port for the UI")
	rootCmd.AddCommand(uiCmd)
}
