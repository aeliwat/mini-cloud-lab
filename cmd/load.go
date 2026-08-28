package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"minicloud/internal/sim"
	"minicloud/internal/store"
)

var (
	loadRPS      int
	loadDuration time.Duration
	loadUsers    int
)

var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Simulate traffic against the provisioned architecture",
	Long: `Simulate request load across your cloud.

Path without LB:  users → web servers (even split)
Path with LB:     users → load balancer → web servers

Capacity (teaching model):
  web = RAM_GB × 500 RPS
  lb  = RAM_GB × 2000 RPS

Examples:
  minicloud load --rps 1000 --duration 30s
  minicloud load --rps 5000 --duration 60s --users 100000`,
	RunE: runLoad,
}

func init() {
	loadCmd.Flags().IntVar(&loadRPS, "rps", 100, "target requests per second")
	loadCmd.Flags().DurationVar(&loadDuration, "duration", 10*time.Second, "how long to simulate load")
	loadCmd.Flags().IntVar(&loadUsers, "users", 0, "optional virtual user count (shown in the report)")

	rootCmd.AddCommand(loadCmd)
}

func runLoad(cmd *cobra.Command, args []string) error {
	s, err := store.Default()
	if err != nil {
		return err
	}

	servers, err := s.Read()
	if err != nil {
		return err
	}

	result, err := sim.Run(servers, sim.Config{
		RPS:      loadRPS,
		Duration: loadDuration,
		Users:    loadUsers,
	})
	if err != nil {
		return err
	}

	// Persist health changes discovered during the run.
	if len(result.ServersOut) > 0 {
		if err := s.Replace(result.ServersOut); err != nil {
			return fmt.Errorf("save health state: %w", err)
		}
	}

	printLoadReport(result)
	return nil
}

func printLoadReport(r *sim.Result) {
	fmt.Println("Load simulation report")
	fmt.Println("----------------------")
	if r.Users > 0 {
		fmt.Printf("Virtual users:  %d\n", r.Users)
	}
	fmt.Printf("Via LB:         %v\n", r.ViaLB)
	fmt.Printf("Target RPS:     %d\n", r.TargetRPS)
	fmt.Printf("Duration:       %s\n", r.Duration)
	fmt.Printf("Total requests: %d\n", r.TotalRequests)
	fmt.Printf("Succeeded:      %d\n", r.Success)
	fmt.Printf("Failed:         %d\n", r.Failed)
	if r.TotalRequests > 0 {
		okRate := float64(r.Success) / float64(r.TotalRequests) * 100
		fmt.Printf("Success rate:   %.1f%%\n", okRate)
	}
	if r.Latency.N > 0 {
		fmt.Printf("Latency avg:    %.1f ms\n", r.Latency.Avg)
		fmt.Printf("Latency p50:    %.1f ms\n", r.Latency.P50)
		fmt.Printf("Latency p95:    %.1f ms\n", r.Latency.P95)
	}

	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVER\tROLE\tRAM\tCAPACITY\tTARGET_RPS\tOK\tFAIL\tHEALTH")
	for _, st := range r.Servers {
		health := "healthy"
		if !st.Healthy {
			health = "unhealthy"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d rps\t%d\t%d\t%d\t%s\n",
			st.Server.ID, st.Server.Role, st.Server.RAM, st.Capacity, st.TargetRPS, st.Success, st.Failed, health)
	}
	_ = w.Flush()

	fmt.Println()
	fmt.Println("Hint: web capacity = RAM_GB×500 RPS; lb capacity = RAM_GB×2000 RPS.")
	fmt.Println("Health: nodes stay unhealthy after sustained overload until 'minicloud server start <id>'.")
}
