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

func TestUnhealthySkipped(t *testing.T) {
	servers := []models.Server{
		{ID: "w1", Role: models.RoleWeb, RAM: "1G", Disk: "10G", Status: models.StatusRunning, Health: models.HealthUnhealthy},
		{ID: "w2", Role: models.RoleWeb, RAM: "4G", Disk: "10G", Status: models.StatusRunning},
	}
	// Run resets health at start, so both are healthy; verify fill-forward prefers w1.
	res, err := Run(servers, Config{RPS: 800, Duration: time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var w1, w2 int
	for _, st := range res.Servers {
		switch st.Server.ID {
		case "w1":
			w1 = st.TargetRPS
		case "w2":
			w2 = st.TargetRPS
		}
	}
	if w1 != 500 {
		t.Fatalf("w1 target=%d, want 500 (filled first)", w1)
	}
	if w2 != 300 {
		t.Fatalf("w2 target=%d, want 300 (forwarded remainder)", w2)
	}
}

func TestFillForwardWeb1First(t *testing.T) {
	servers := []models.Server{
		{ID: "w1", Role: models.RoleWeb, RAM: "1G", Disk: "10G", Status: models.StatusRunning}, // 500
		{ID: "w2", Role: models.RoleWeb, RAM: "4G", Disk: "10G", Status: models.StatusRunning}, // 2000
	}
	res, err := Run(servers, Config{RPS: 1500, Duration: time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var w1, w2 int
	for _, st := range res.Servers {
		switch st.Server.ID {
		case "w1":
			w1 = st.TargetRPS
		case "w2":
			w2 = st.TargetRPS
		}
	}
	if w1 != 500 {
		t.Fatalf("w1 target=%d, want 500 (filled first)", w1)
	}
	if w2 != 1000 {
		t.Fatalf("w2 target=%d, want 1000 (forwarded remainder)", w2)
	}
	if res.Failed != 0 {
		t.Fatalf("failed=%d, want 0", res.Failed)
	}
}

func TestWebConnectsToOwnDB(t *testing.T) {
	servers := []models.Server{
		{ID: "w1", Role: models.RoleWeb, RAM: "2G", Disk: "20G", Status: models.StatusRunning, DBID: "db1"},
		{ID: "w2", Role: models.RoleWeb, RAM: "2G", Disk: "20G", Status: models.StatusRunning, DBID: "db2"},
		{ID: "db1", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
		{ID: "db2", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning},
	}
	res, err := Run(servers, Config{RPS: 1500, Duration: 3 * time.Second})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var db1, db2 int64
	for _, st := range res.Servers {
		switch st.Server.ID {
		case "db1":
			db1 = st.Success
		case "db2":
			db2 = st.Success
		}
	}
	// fill-forward: w1(1000) → db1, remainder w2(500) → db2
	if db1 == 0 || db2 == 0 {
		t.Fatalf("expected each web to hit its own DB, db1=%d db2=%d", db1, db2)
	}
	if res.Success <= 0 {
		t.Fatalf("expected e2e success via paired DBs, got %d", res.Success)
	}
}
