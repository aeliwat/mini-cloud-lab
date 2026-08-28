package sim

import (
	"testing"
	"time"

	"minicloud/internal/models"
)

func TestParseRAMGB(t *testing.T) {
	gb, err := ParseRAMGB("2G")
	if err != nil || gb != 2 {
		t.Fatalf("ParseRAMGB(2G) = %v, %v", gb, err)
	}
}

func TestRunOverload(t *testing.T) {
	servers := []models.Server{
		{ID: "w1", Role: models.RoleWeb, RAM: "1G", Disk: "10G", Status: models.StatusRunning},
	}
	res, err := Run(servers, Config{RPS: 1000, Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// After UnhealthyAfterSec of overload, node is removed from routing,
	// so totals are lower than a full 10s at 500 rps.
	if res.Success <= 0 || res.Failed <= 0 {
		t.Fatalf("expected some success and fail, got %d/%d", res.Success, res.Failed)
	}
	if res.Latency.P95 <= 0 {
		t.Fatalf("expected p95 latency > 0, got %+v", res.Latency)
	}
	if len(res.Ticks) != 10 {
		t.Fatalf("ticks=%d, want 10", len(res.Ticks))
	}
}

func TestRunRequiresWeb(t *testing.T) {
	servers := []models.Server{
		{ID: "db1", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
	}
	_, err := Run(servers, Config{RPS: 100, Duration: time.Second})
	if err == nil {
		t.Fatal("expected error when no web servers")
	}
}

func TestRunSplitAcrossTwo(t *testing.T) {
	servers := []models.Server{
		{ID: "w1", Role: models.RoleWeb, RAM: "2G", Disk: "10G", Status: models.StatusRunning},
		{ID: "w2", Role: models.RoleWeb, RAM: "2G", Disk: "10G", Status: models.StatusRunning},
	}
	res, err := Run(servers, Config{RPS: 1000, Duration: 10 * time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Failed != 0 || res.Success != 10000 {
		t.Fatalf("success=%d failed=%d, want 10000/0", res.Success, res.Failed)
	}
}

func TestStoppedWebExcluded(t *testing.T) {
	servers := []models.Server{
		{ID: "w1", Role: models.RoleWeb, RAM: "2G", Disk: "10G", Status: models.StatusRunning},
		{ID: "w2", Role: models.RoleWeb, RAM: "2G", Disk: "10G", Status: models.StatusStopped},
	}
	res, err := Run(servers, Config{RPS: 500, Duration: 2 * time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	routed := 0
	for _, st := range res.Servers {
		if st.Success > 0 || st.Failed > 0 {
			routed++
			if st.Server.ID != "w1" {
				t.Fatalf("traffic reached stopped node %s", st.Server.ID)
			}
		}
	}
	if routed == 0 {
		t.Fatal("expected traffic on w1")
	}
}

func TestUnhealthyAfterOverload(t *testing.T) {
	servers := []models.Server{
		{ID: "w1", Role: models.RoleWeb, RAM: "1G", Disk: "10G", Status: models.StatusRunning},
		{ID: "w2", Role: models.RoleWeb, RAM: "4G", Disk: "10G", Status: models.StatusRunning},
	}
	// Overwhelm w1 share; after 3s it should flip unhealthy and traffic shift to w2.
	res, err := Run(servers, Config{RPS: 3000, Duration: 8 * time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var w1Healthy bool
	for _, s := range res.ServersOut {
		if s.ID == "w1" {
			w1Healthy = s.IsHealthy()
		}
	}
	if w1Healthy {
		t.Fatal("expected w1 to become unhealthy after sustained overload")
	}
}

func TestLBBottleneck(t *testing.T) {
	servers := []models.Server{
		{ID: "lb1", Role: models.RoleLB, RAM: "512M", Disk: "5G", Status: models.StatusRunning},
		{ID: "w1", Role: models.RoleWeb, RAM: "4G", Disk: "50G", Status: models.StatusRunning},
	}
	res, err := Run(servers, Config{RPS: 2000, Duration: 5 * time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.ViaLB {
		t.Fatal("expected via LB")
	}
	if res.Failed == 0 {
		t.Fatal("expected LB failures")
	}
}
