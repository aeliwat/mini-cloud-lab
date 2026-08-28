// Package sim simulates traffic against provisioned minicloud servers.
package sim

import (
	"fmt"
	"strconv"
	"strings"

	"minicloud/internal/models"
)

const (
	// RPSPerGB is capacity for web servers (teaching simplification).
	RPSPerGB = 500
	// RPSPerGBLB is capacity for load balancers (higher throughput per GB).
	RPSPerGBLB = 2000
	// RPSPerGBDB is query capacity for databases.
	RPSPerGBDB = 1000
)

// ParseRAMGB converts values like "2G", "2g", "512M" into gigabytes (float).
func ParseRAMGB(ram string) (float64, error) {
	ram = strings.TrimSpace(ram)
	if ram == "" {
		return 0, fmt.Errorf("ram is empty")
	}

	unit := ram[len(ram)-1]
	numPart := ram[:len(ram)-1]
	n, err := strconv.ParseFloat(numPart, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid ram %q", ram)
	}

	switch unit {
	case 'G', 'g':
		return n, nil
	case 'M', 'm':
		return n / 1024, nil
	default:
		return 0, fmt.Errorf("invalid ram %q; use values like 2G or 512M", ram)
	}
}

// MaxRPS returns the simulated request capacity for a server based on role + RAM.
func MaxRPS(srv models.Server) (int, error) {
	gb, err := ParseRAMGB(srv.RAM)
	if err != nil {
		return 0, fmt.Errorf("server %s: %w", srv.ID, err)
	}
	perGB := RPSPerGB
	switch srv.Role {
	case models.RoleLB:
		perGB = RPSPerGBLB
	case models.RoleDB:
		perGB = RPSPerGBDB
	}
	return int(gb * float64(perGB)), nil
}
