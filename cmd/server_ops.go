package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"minicloud/internal/models"
	"minicloud/internal/store"
)

var serverLSCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Default()
		if err != nil {
			return err
		}
		servers, err := s.Read()
		if err != nil {
			return err
		}
		if len(servers) == 0 {
			fmt.Println("No servers.")
			return nil
		}
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

var serverRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Delete a server by id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Default()
		if err != nil {
			return err
		}
		id := args[0]
		if err := s.Delete(id); err != nil {
			return err
		}
		fmt.Printf("Deleted server %s\n", id)
		return nil
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop <id>",
	Short: "Stop a server (LB will skip it)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Default()
		if err != nil {
			return err
		}
		srv, err := s.SetStatus(args[0], models.StatusStopped)
		if err != nil {
			return err
		}
		fmt.Printf("Stopped %s (%s)\n", srv.ID, srv.Role)
		return nil
	},
}

var serverStartCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Start a server and mark it healthy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Default()
		if err != nil {
			return err
		}
		srv, err := s.SetStatus(args[0], models.StatusRunning)
		if err != nil {
			return err
		}
		fmt.Printf("Started %s (%s, healthy)\n", srv.ID, srv.Role)
		return nil
	},
}

func init() {
	serverCmd.AddCommand(serverLSCmd)
	serverCmd.AddCommand(serverRmCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStartCmd)
}
