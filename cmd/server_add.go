package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"minicloud/internal/models"
	"minicloud/internal/store"
)

var (
	serverType string
	serverRAM  string
	serverDisk string
)

var serverAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a simulated server to local state",
	Long: `Add a server with the given role and resource sizes.

Examples:
  minicloud server add --type web --ram 2G --disk 50G
  minicloud server add --type db  --ram 4G --disk 100G
  minicloud server add --type lb  --ram 1G --disk 10G`,
	RunE: runServerAdd,
}

func init() {
	serverAddCmd.Flags().StringVar(&serverType, "type", "", "server role: web, db, or lb")
	serverAddCmd.Flags().StringVar(&serverRAM, "ram", "", "RAM size, e.g. 2G")
	serverAddCmd.Flags().StringVar(&serverDisk, "disk", "", "disk size, e.g. 50G")

	_ = serverAddCmd.MarkFlagRequired("type")
	_ = serverAddCmd.MarkFlagRequired("ram")
	_ = serverAddCmd.MarkFlagRequired("disk")

	serverCmd.AddCommand(serverAddCmd)
}

func runServerAdd(cmd *cobra.Command, args []string) error {
	role, err := parseRole(serverType)
	if err != nil {
		return err
	}
	if serverRAM == "" || serverDisk == "" {
		return fmt.Errorf("--ram and --disk must be non-empty")
	}

	s, err := store.Default()
	if err != nil {
		return err
	}

	id, err := newServerID()
	if err != nil {
		return fmt.Errorf("generate server id: %w", err)
	}

	srv := models.Server{
		ID:     id,
		Role:   role,
		RAM:    serverRAM,
		Disk:   serverDisk,
		Status: models.StatusRunning,
		Health: models.HealthHealthy,
	}

	if err := s.Add(srv); err != nil {
		return err
	}

	fmt.Printf("Added server %s (%s, ram=%s, disk=%s)\n", srv.ID, srv.Role, srv.RAM, srv.Disk)
	return nil
}

func parseRole(raw string) (models.Role, error) {
	switch models.Role(raw) {
	case models.RoleWeb, models.RoleDB, models.RoleLB:
		return models.Role(raw), nil
	default:
		return "", fmt.Errorf("invalid --type %q; must be %q, %q, or %q", raw, models.RoleWeb, models.RoleDB, models.RoleLB)
	}
}

// newServerID returns a short unique id like "srv-a1b2c3d4".
func newServerID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "srv-" + hex.EncodeToString(buf), nil
}
