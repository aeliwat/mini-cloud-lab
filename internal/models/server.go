// Package models defines the domain types for the minicloud simulator.
package models

// Role describes the function of a simulated server.
type Role string

const (
	RoleWeb Role = "web"
	RoleDB  Role = "db"
	RoleLB  Role = "lb" // load balancer — sits in front of web servers
)

// Status is the lifecycle state of a server.
type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

// Health is the health-check result used by the load balancer.
type Health string

const (
	HealthHealthy   Health = "healthy"
	HealthUnhealthy Health = "unhealthy"
)

// Server represents a virtual machine in the local cloud simulation.
type Server struct {
	ID     string `json:"id"`
	Role   Role   `json:"role"`             // web, db, or lb
	RAM    string `json:"ram"`              // human-readable, e.g. "2G"
	Disk   string `json:"disk"`             // human-readable, e.g. "50G"
	Status Status `json:"status"`           // running or stopped
	Health Health `json:"health,omitempty"` // healthy (default) or unhealthy
}

// IsRoutable reports whether the LB/clients may send traffic to this node.
func (s Server) IsRoutable() bool {
	return s.Status == StatusRunning && s.IsHealthy()
}

// IsHealthy treats missing/empty health as healthy (backward compatible).
func (s Server) IsHealthy() bool {
	return s.Health == "" || s.Health == HealthHealthy
}
