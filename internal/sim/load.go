package sim

import (
	"fmt"
	"math"
	"sort"
	"time"

	"minicloud/internal/models"
)

const (
	// UnhealthyAfterSec marks a web node unhealthy after this many overloaded seconds.
	UnhealthyAfterSec = 3

	latencyLBMS  = 2.0
	latencyWebMS = 12.0
	latencyDBMS  = 5.0
)

// Config controls a load simulation run.
type Config struct {
	RPS      int
	Duration time.Duration
	Users    int
}

// ServerStat is the aggregate per-node outcome of a simulation.
type ServerStat struct {
	Server    models.Server
	Capacity  int
	TargetRPS int
	Success   int64
	Failed    int64
	Healthy   bool
}

// TickNode is one node's stats for a single simulated second.
type TickNode struct {
	ID        string  `json:"id"`
	Role      string  `json:"role"`
	Capacity  int     `json:"capacity"`
	TargetRPS int     `json:"target_rps"`
	Success   int64   `json:"success"`
	Failed    int64   `json:"failed"`
	Healthy   bool    `json:"healthy"`
	LatencyMS float64 `json:"latency_ms"`
}

// Tick is one second of the live timeline.
type Tick struct {
	Second    int        `json:"second"`
	Success   int64      `json:"success"`
	Failed    int64      `json:"failed"`
	LatencyMS float64    `json:"latency_ms"`
	Nodes     []TickNode `json:"nodes"`
}

// LatencySummary holds end-to-end latency percentiles (ms).
type LatencySummary struct {
	Avg float64 `json:"avg_ms"`
	P50 float64 `json:"p50_ms"`
	P95 float64 `json:"p95_ms"`
	N   int     `json:"samples"`
}

// Result is the aggregate outcome of a simulation.
type Result struct {
	Duration      time.Duration
	Users         int
	TargetRPS     int
	TotalRequests int64
	Success       int64
	Failed        int64
	ViaLB         bool
	Servers       []ServerStat
	Ticks         []Tick
	Latency       LatencySummary
	// ServersOut is the full fleet with updated health/status after the run.
	ServersOut []models.Server
}

// Run simulates traffic second-by-second with health checks and latency.
func Run(servers []models.Server, cfg Config) (*Result, error) {
	if cfg.RPS <= 0 {
		return nil, fmt.Errorf("rps must be > 0")
	}
	if cfg.Duration <= 0 {
		return nil, fmt.Errorf("duration must be > 0")
	}

	fleet := cloneServers(servers)
	resetHealth(fleet)
	nSeconds := int(math.Round(cfg.Duration.Seconds()))
	if nSeconds < 1 {
		nSeconds = 1
	}

	lbs := filterRoutable(fleet, models.RoleLB)
	viaLB := len(lbs) > 0

	cumOK := map[string]int64{}
	cumFail := map[string]int64{}
	lastTarget := map[string]int{}
	capacity := map[string]int{}
	overloadStreak := map[string]int{}
	latencySamples := make([]float64, 0, 4096)

	ticks := make([]Tick, 0, nSeconds)

	for sec := 1; sec <= nSeconds; sec++ {
		web := filterRoutable(fleet, models.RoleWeb)
		if len(web) == 0 {
			if sec == 1 {
				return nil, fmt.Errorf("no healthy running web servers to receive traffic")
			}
			// Fleet exhausted — remaining seconds are total failures.
			for ; sec <= nSeconds; sec++ {
				ticks = append(ticks, Tick{
					Second:  sec,
					Success: 0,
					Failed:  int64(cfg.RPS),
					Nodes:   nil,
				})
			}
			break
		}

		forwardRPS := cfg.RPS
		var lb *models.Server
		var lbCap int
		if viaLB {
			routableLB := filterRoutable(fleet, models.RoleLB)
			if len(routableLB) == 0 {
				return nil, fmt.Errorf("load balancer is stopped or unhealthy")
			}
			lb = &routableLB[0]
			var err error
			lbCap, err = MaxRPS(*lb)
			if err != nil {
				return nil, err
			}
			if cfg.RPS > lbCap {
				forwardRPS = lbCap
			}
			capacity[lb.ID] = lbCap
		}

		caps := make([]int, len(web))
		targets := make([]int, len(web))
		for i, srv := range web {
			capRPS, err := MaxRPS(srv)
			if err != nil {
				return nil, err
			}
			caps[i] = capRPS
			capacity[srv.ID] = capRPS
		}
		// LB / clients send to web-1 first, then forward leftover to web-2, etc.
		unplaced := fillForward(targets, caps, forwardRPS)

		nodes := make([]TickNode, 0, len(web)+2)
		var webTickOK, webTickFail, lbTickFail, dbTickFail int64
		var tickLatencySum float64
		var tickLatencyN int

		if lb != nil {
			offered := int64(cfg.RPS)
			ok := offered
			var fail int64
			if offered > int64(lbCap) {
				ok = int64(lbCap)
				fail = offered - int64(lbCap)
			}
			cumOK[lb.ID] += ok
			cumFail[lb.ID] += fail
			lastTarget[lb.ID] = cfg.RPS
			lbTickFail = fail
			lat := latencyLBMS
			if float64(cfg.RPS) > float64(lbCap) {
				lat += (float64(cfg.RPS)/float64(lbCap) - 1) * 8
			}
			nodes = append(nodes, TickNode{
				ID: lb.ID, Role: string(models.RoleLB), Capacity: lbCap,
				TargetRPS: cfg.RPS, Success: ok, Failed: fail,
				Healthy: lb.IsHealthy(), LatencyMS: round1(lat),
			})
		}

		webTickFail += int64(unplaced)
		for i, srv := range web {
			capRPS := caps[i]
			target := targets[i]
			lastTarget[srv.ID] = target

			ok := int64(target)
			cumOK[srv.ID] += ok
			webTickOK += ok

			lat := latencyWebMS
			if viaLB {
				lat += latencyLBMS
			}
			loadRatio := float64(target) / float64(max(capRPS, 1))
			if loadRatio > 1 {
				lat += (loadRatio - 1) * 25
			} else {
				lat += loadRatio * 4
			}
			lat = round1(lat)

			nodes = append(nodes, TickNode{
				ID: srv.ID, Role: string(models.RoleWeb), Capacity: capRPS,
				TargetRPS: target, Success: ok, Failed: 0,
				Healthy: srv.IsHealthy(), LatencyMS: lat,
			})
		}
		if unplaced > 0 {
			// Fleet saturated — attribute leftover fails to the last web that is full.
			for i := len(nodes) - 1; i >= 0; i-- {
				if nodes[i].Role != string(models.RoleWeb) {
					continue
				}
				if nodes[i].TargetRPS >= nodes[i].Capacity && nodes[i].Capacity > 0 {
					nodes[i].Failed = int64(unplaced)
					cumFail[nodes[i].ID] += int64(unplaced)
					break
				}
			}
		}

		// Each web sends successful requests to its own paired database.
		e2eOK := webTickOK
		dbOffer := map[string]int{}
		pairedHits := 0
		for i, srv := range web {
			okN := targets[i]
			if okN <= 0 {
				continue
			}
			db := findPairedDB(fleet, srv)
			if db == nil || !db.IsRoutable() {
				dbTickFail += int64(okN)
				continue
			}
			dbOffer[db.ID] += okN
			pairedHits += okN
		}

		if pairedHits > 0 || len(dbOffer) > 0 {
			var dbOKTotal int64
			for _, srv := range fleet {
				if srv.Role != models.RoleDB {
					continue
				}
				offered := dbOffer[srv.ID]
				if offered == 0 {
					continue
				}
				capRPS, err := MaxRPS(srv)
				if err != nil {
					return nil, err
				}
				capacity[srv.ID] = capRPS
				lastTarget[srv.ID] = offered

				ok := int64(offered)
				var fail int64
				if offered > capRPS {
					ok = int64(capRPS)
					fail = int64(offered - capRPS)
				}
				cumOK[srv.ID] += ok
				cumFail[srv.ID] += fail
				dbOKTotal += ok
				dbTickFail += fail

				lat := latencyDBMS
				loadRatio := float64(offered) / float64(max(capRPS, 1))
				if loadRatio > 1 {
					lat += (loadRatio - 1) * 20
				} else {
					lat += loadRatio * 3
				}
				nodes = append(nodes, TickNode{
					ID: srv.ID, Role: string(models.RoleDB), Capacity: capRPS,
					TargetRPS: offered, Success: ok, Failed: fail,
					Healthy: srv.IsHealthy(), LatencyMS: round1(lat),
				})

				if fail > 0 && offered >= capRPS {
					overloadStreak[srv.ID]++
				} else {
					overloadStreak[srv.ID] = 0
				}
				if overloadStreak[srv.ID] >= UnhealthyAfterSec {
					markUnhealthy(fleet, srv.ID)
				}
			}
			// Requests that reached web but had no usable paired DB already counted in dbTickFail.
			e2eOK = dbOKTotal
		} else if hasRole(fleet, models.RoleDB) && webTickOK > 0 {
			dbTickFail = webTickOK
			e2eOK = 0
		}

		// Collect latency samples for end-to-end successes.
		sampleLat := latencyWebMS
		if viaLB {
			sampleLat += latencyLBMS
		}
		if pairedHits > 0 {
			sampleLat += latencyDBMS
		}
		sampleN := int(e2eOK)
		if sampleN > 80 {
			sampleN = 80
		}
		for j := 0; j < sampleN; j++ {
			latencySamples = append(latencySamples, round1(sampleLat))
		}
		tickLatencySum += sampleLat * float64(e2eOK)
		tickLatencyN += int(e2eOK)

		avgTickLat := 0.0
		if tickLatencyN > 0 {
			avgTickLat = round1(tickLatencySum / float64(tickLatencyN))
		}

		ticks = append(ticks, Tick{
			Second:    sec,
			Success:   e2eOK,
			Failed:    lbTickFail + webTickFail + dbTickFail,
			LatencyMS: avgTickLat,
			Nodes:     nodes,
		})
	}

	stats := make([]ServerStat, 0)
	var totalWebOK, totalWebFail, totalLBFail, totalDBFail, totalDBOK int64
	for _, srv := range fleet {
		if srv.Role != models.RoleWeb && srv.Role != models.RoleLB && srv.Role != models.RoleDB {
			continue
		}
		okN := cumOK[srv.ID]
		failN := cumFail[srv.ID]
		if okN == 0 && failN == 0 && lastTarget[srv.ID] == 0 && capacity[srv.ID] == 0 {
			if capRPS, err := MaxRPS(srv); err == nil {
				capacity[srv.ID] = capRPS
			}
		}
		stats = append(stats, ServerStat{
			Server:    srv,
			Capacity:  capacity[srv.ID],
			TargetRPS: lastTarget[srv.ID],
			Success:   okN,
			Failed:    failN,
			Healthy:   srv.IsHealthy(),
		})
		switch srv.Role {
		case models.RoleLB:
			totalLBFail += failN
		case models.RoleWeb:
			totalWebOK += okN
			totalWebFail += failN
		case models.RoleDB:
			totalDBOK += okN
			totalDBFail += failN
		}
	}

	e2eSuccess := totalWebOK
	if hasRole(fleet, models.RoleDB) {
		e2eSuccess = totalDBOK
	}

	totalRequests := int64(cfg.RPS) * int64(nSeconds)
	return &Result{
		Duration:      time.Duration(nSeconds) * time.Second,
		Users:         cfg.Users,
		TargetRPS:     cfg.RPS,
		TotalRequests: totalRequests,
		Success:       e2eSuccess,
		Failed:        totalLBFail + totalWebFail + totalDBFail,
		ViaLB:         viaLB,
		Servers:       stats,
		Ticks:         ticks,
		Latency:       summarizeLatency(latencySamples),
		ServersOut:    fleet,
	}, nil
}

func summarizeLatency(samples []float64) LatencySummary {
	if len(samples) == 0 {
		return LatencySummary{}
	}
	sort.Float64s(samples)
	var sum float64
	for _, v := range samples {
		sum += v
	}
	return LatencySummary{
		Avg: round1(sum / float64(len(samples))),
		P50: round1(percentile(samples, 50)),
		P95: round1(percentile(samples, 95)),
		N:   len(samples),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(len(sorted)-1)
	i := int(math.Floor(rank))
	frac := rank - float64(i)
	if i+1 >= len(sorted) {
		return sorted[i]
	}
	return sorted[i]*(1-frac) + sorted[i+1]*frac
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cloneServers(in []models.Server) []models.Server {
	out := make([]models.Server, len(in))
	copy(out, in)
	return out
}

func filterRoutable(servers []models.Server, role models.Role) []models.Server {
	out := make([]models.Server, 0)
	for _, s := range servers {
		if s.Role == role && s.IsRoutable() {
			out = append(out, s)
		}
	}
	return out
}

// fillForward sends traffic to web-1 first up to capacity, then forwards
// leftover RPS to web-2, and so on. Returns RPS that could not be placed.
func fillForward(targets, caps []int, total int) int {
	remaining := total
	for i := range targets {
		take := remaining
		if take > caps[i] {
			take = caps[i]
		}
		if take < 0 {
			take = 0
		}
		targets[i] = take
		remaining -= take
	}
	if remaining < 0 {
		return 0
	}
	return remaining
}

func resetHealth(fleet []models.Server) {
	for i := range fleet {
		if fleet[i].Status == models.StatusRunning {
			fleet[i].Health = models.HealthHealthy
		}
	}
}

// findPairedDB returns the database linked to a web server via db_id.
func findPairedDB(fleet []models.Server, web models.Server) *models.Server {
	if web.DBID == "" {
		return nil
	}
	for i := range fleet {
		if fleet[i].ID == web.DBID && fleet[i].Role == models.RoleDB {
			return &fleet[i]
		}
	}
	return nil
}

func hasRole(servers []models.Server, role models.Role) bool {
	for _, s := range servers {
		if s.Role == role {
			return true
		}
	}
	return false
}

func markUnhealthy(fleet []models.Server, id string) {
	for i := range fleet {
		if fleet[i].ID == id {
			fleet[i].Health = models.HealthUnhealthy
			return
		}
	}
}
