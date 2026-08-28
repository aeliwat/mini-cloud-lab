package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"minicloud/internal/models"
	"minicloud/internal/store"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show servers in the local minicloud state",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Default()
		if err != nil {
			return err
		}

		servers, err := s.Read()
		if err != nil {
			return err
		}

		fmt.Printf("State:    %s\n", s.Path)
		fmt.Printf("Servers:  %d\n", len(servers))

		if len(servers) == 0 {
			fmt.Println("\nNo servers provisioned yet.")
			return nil
		}

		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tROLE\tRAM\tDISK\tSTATUS\tHEALTH")
		for _, srv := range servers {
			health := models.HealthHealthy
			if !srv.IsHealthy() {
				health = models.HealthUnhealthy
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				srv.ID, srv.Role, srv.RAM, srv.Disk, srv.Status, health)
		}
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
