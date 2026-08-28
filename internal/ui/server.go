package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"minicloud/internal/models"
	"minicloud/internal/scenario"
	"minicloud/internal/sim"
	"minicloud/internal/store"
)

//go:embed static/*
var staticFS embed.FS

// Server serves the animated topology dashboard and JSON APIs.
type Server struct {
	store *store.Store
	addr  string
}

// New creates a UI server bound to addr (e.g. ":7474").
func New(addr string, st *store.Store) *Server {
	return &Server{store: st, addr: addr}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static assets: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("GET /api/servers", s.handleServers)
	mux.HandleFunc("DELETE /api/servers/{id}", s.handleServerDelete)
	mux.HandleFunc("POST /api/load", s.handleLoad)
	mux.HandleFunc("GET /api/scenarios", s.handleScenariosList)
	mux.HandleFunc("POST /api/scenarios/{id}", s.handleScenarioApply)

	fmt.Printf("minicloud UI → http://localhost%s\n", s.addr)
	fmt.Printf("State file     → %s\n", s.store.Path)
	return http.ListenAndServe(s.addr, withCORS(mux))
}

func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.Read()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"servers": enrich(servers),
		"path":    s.store.Path,
	})
}

func (s *Server) handleServerDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("missing server id"))
		return
	}
	if err := s.store.Delete(id); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	servers, err := s.store.Read()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": id,
		"servers": enrich(servers),
	})
}

func (s *Server) handleScenariosList(w http.ResponseWriter, r *http.Request) {
	type item struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Users       int    `json:"users"`
		RPS         int    `json:"rps"`
		Duration    string `json:"duration"`
	}
	out := make([]item, 0)
	for _, sc := range scenario.All() {
		out = append(out, item{
			ID: sc.ID, Name: sc.Name, Description: sc.Description,
			Users: sc.Users, RPS: sc.RPS, Duration: sc.Duration,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"scenarios": out})
}

func (s *Server) handleScenarioApply(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	spec, err := scenario.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err := s.store.Replace(spec.Servers); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          spec.ID,
		"name":        spec.Name,
		"description": spec.Description,
		"users":       spec.Users,
		"rps":         spec.RPS,
		"duration":    spec.Duration,
		"servers":     enrich(spec.Servers),
	})
}

func (s *Server) handleLoad(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RPS      int    `json:"rps"`
		Duration string `json:"duration"`
		Users    int    `json:"users"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid JSON body"))
		return
	}
	if body.RPS <= 0 {
		body.RPS = 1000
	}
	dur := 10 * time.Second
	if body.Duration != "" {
		parsed, err := time.ParseDuration(body.Duration)
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid duration: %w", err))
			return
		}
		dur = parsed
	}

	servers, err := s.store.Read()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	result, err := sim.Run(servers, sim.Config{
		RPS:      body.RPS,
		Duration: dur,
		Users:    body.Users,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if len(result.ServersOut) > 0 {
		_ = s.store.Replace(result.ServersOut)
	}

	type statDTO struct {
		ID        string `json:"id"`
		Role      string `json:"role"`
		RAM       string `json:"ram"`
		Capacity  int    `json:"capacity"`
		TargetRPS int    `json:"target_rps"`
		Success   int64  `json:"success"`
		Failed    int64  `json:"failed"`
		Healthy   bool   `json:"healthy"`
	}
	stats := make([]statDTO, 0, len(result.Servers))
	for _, st := range result.Servers {
		stats = append(stats, statDTO{
			ID:        st.Server.ID,
			Role:      string(st.Server.Role),
			RAM:       st.Server.RAM,
			Capacity:  st.Capacity,
			TargetRPS: st.TargetRPS,
			Success:   st.Success,
			Failed:    st.Failed,
			Healthy:   st.Healthy,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"duration":       result.Duration.String(),
		"users":          result.Users,
		"target_rps":     result.TargetRPS,
		"total_requests": result.TotalRequests,
		"success":        result.Success,
		"failed":         result.Failed,
		"via_lb":         result.ViaLB,
		"servers":        stats,
		"ticks":          result.Ticks,
		"latency":        result.Latency,
	})
}

type nodeDTO struct {
	models.Server
	CapacityRPS int  `json:"capacity_rps"`
	Healthy     bool `json:"healthy"`
}

func enrich(servers []models.Server) []nodeDTO {
	out := make([]nodeDTO, 0, len(servers))
	for _, srv := range servers {
		n := nodeDTO{Server: srv, Healthy: srv.IsHealthy()}
		if srv.Role == models.RoleWeb || srv.Role == models.RoleLB || srv.Role == models.RoleDB {
			if capRPS, err := sim.MaxRPS(srv); err == nil {
				n.CapacityRPS = capRPS
			}
		}
		out = append(out, n)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ParseAddr normalizes a port flag into ":PORT".
func ParseAddr(port string) string {
	if port == "" {
		return ":7474"
	}
	if port[0] == ':' {
		return port
	}
	if _, err := strconv.Atoi(port); err == nil {
		return ":" + port
	}
	return port
}
