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

		base := forwardRPS / len(web)
		rem := forwardRPS % len(web)

		nodes := make([]TickNode, 0, len(web)+1)
		var webTickOK, webTickFail, lbTickFail int64
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

		for i, srv := range web {
			capRPS, err := MaxRPS(srv)
			if err != nil {
				return nil, err
			}
			capacity[srv.ID] = capRPS
			target := base
			if i < rem {
				target++
			}
			lastTarget[srv.ID] = target

			offered := int64(target)
			ok := offered
			var fail int64
			if offered > int64(capRPS) {
				ok = int64(capRPS)
				fail = offered - int64(capRPS)
			}
			cumOK[srv.ID] += ok
			cumFail[srv.ID] += fail
			webTickOK += ok
			webTickFail += fail

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

			// Collect latency samples for successes (capped per second).
			sampleN := int(ok)
			if sampleN > 80 {
				sampleN = 80
			}
			for j := 0; j < sampleN; j++ {
				latencySamples = append(latencySamples, lat)
			}
			tickLatencySum += lat * float64(ok)
			tickLatencyN += int(ok)

			nodes = append(nodes, TickNode{
				ID: srv.ID, Role: string(models.RoleWeb), Capacity: capRPS,
				TargetRPS: target, Success: ok, Failed: fail,
				Healthy: srv.IsHealthy(), LatencyMS: lat,
			})

			// Health check: sustained overload → unhealthy (LB will skip next ticks).
			if fail > 0 && target >= capRPS {
				overloadStreak[srv.ID]++
			} else {
				overloadStreak[srv.ID] = 0
			}
			if overloadStreak[srv.ID] >= UnhealthyAfterSec {
				markUnhealthy(fleet, srv.ID)
			}
		}

		avgTickLat := 0.0
		if tickLatencyN > 0 {
			avgTickLat = round1(tickLatencySum / float64(tickLatencyN))
		}

		ticks = append(ticks, Tick{
			Second:    sec,
			Success:   webTickOK,
			Failed:    lbTickFail + webTickFail,
			LatencyMS: avgTickLat,
			Nodes:     nodes,
		})
	}

	stats := make([]ServerStat, 0)
	var totalWebOK, totalWebFail, totalLBFail int64
	for _, srv := range fleet {
		if srv.Role != models.RoleWeb && srv.Role != models.RoleLB {
			continue
		}
		okN := cumOK[srv.ID]
		failN := cumFail[srv.ID]
		if okN == 0 && failN == 0 && lastTarget[srv.ID] == 0 && capacity[srv.ID] == 0 {
			// never targeted; still show if web/lb exists
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
		if srv.Role == models.RoleLB {
			totalLBFail += failN
		} else if srv.Role == models.RoleWeb {
			totalWebOK += okN
			totalWebFail += failN
		}
	}

	totalRequests := int64(cfg.RPS) * int64(nSeconds)
	return &Result{
		Duration:      time.Duration(nSeconds) * time.Second,
		Users:         cfg.Users,
		TargetRPS:     cfg.RPS,
		TotalRequests: totalRequests,
		Success:       totalWebOK,
		Failed:        totalLBFail + totalWebFail,
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

func markUnhealthy(fleet []models.Server, id string) {
	for i := range fleet {
		if fleet[i].ID == id {
			fleet[i].Health = models.HealthUnhealthy
			return
		}
	}
}
