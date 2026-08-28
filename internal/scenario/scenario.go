package scenario

import (
	"fmt"

	"minicloud/internal/models"
)

// Spec describes a one-click demo architecture + suggested load knobs.
type Spec struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Servers     []models.Server `json:"servers"`
	Users       int             `json:"users"`
	RPS         int             `json:"rps"`
	Duration    string          `json:"duration"`
}

// All returns the built-in scenario presets.
func All() []Spec {
	return []Spec{
		{
			ID:          "black_friday",
			Name:        "Black Friday",
			Description: "Huge traffic spike across an LB and three small web nodes. Expect overload.",
			Users:       500000,
			RPS:         8000,
			Duration:    "20s",
			Servers: []models.Server{
				{ID: "lb-1", Role: models.RoleLB, RAM: "1G", Disk: "10G", Status: models.StatusRunning},
				{ID: "web-1", Role: models.RoleWeb, RAM: "1G", Disk: "20G", Status: models.StatusRunning, DBID: "db-1"},
				{ID: "web-2", Role: models.RoleWeb, RAM: "1G", Disk: "20G", Status: models.StatusRunning, DBID: "db-2"},
				{ID: "web-3", Role: models.RoleWeb, RAM: "1G", Disk: "20G", Status: models.StatusRunning, DBID: "db-3"},
				{ID: "db-1", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
				{ID: "db-2", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
				{ID: "db-3", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
			},
		},
		{
			ID:          "az_failure",
			Name:        "Single AZ failure",
			Description: "One web node is stopped (failed AZ). LB fronts the remaining healthy web.",
			Users:       80000,
			RPS:         1500,
			Duration:    "15s",
			Servers: []models.Server{
				{ID: "lb-1", Role: models.RoleLB, RAM: "2G", Disk: "10G", Status: models.StatusRunning},
				{ID: "web-a", Role: models.RoleWeb, RAM: "2G", Disk: "40G", Status: models.StatusRunning, DBID: "db-a"},
				{ID: "web-b", Role: models.RoleWeb, RAM: "2G", Disk: "40G", Status: models.StatusStopped, DBID: "db-b"},
				{ID: "db-a", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
				{ID: "db-b", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
			},
		},
		{
			ID:          "tiny_lb",
			Name:        "Tiny LB bottleneck",
			Description: "Strong web tier, undersized load balancer. Failures appear on the LB first.",
			Users:       200000,
			RPS:         3000,
			Duration:    "15s",
			Servers: []models.Server{
				{ID: "lb-1", Role: models.RoleLB, RAM: "512M", Disk: "5G", Status: models.StatusRunning},
				{ID: "web-1", Role: models.RoleWeb, RAM: "4G", Disk: "50G", Status: models.StatusRunning, DBID: "db-1"},
				{ID: "web-2", Role: models.RoleWeb, RAM: "4G", Disk: "50G", Status: models.StatusRunning, DBID: "db-2"},
				{ID: "db-1", Role: models.RoleDB, RAM: "8G", Disk: "200G", Status: models.StatusRunning},
				{ID: "db-2", Role: models.RoleDB, RAM: "8G", Disk: "200G", Status: models.StatusRunning},
			},
		},
	}
}

// Get returns a preset by id.
func Get(id string) (Spec, error) {
	for _, s := range All() {
		if s.ID == id {
			return s, nil
		}
	}
	return Spec{}, fmt.Errorf("unknown scenario %q", id)
}
