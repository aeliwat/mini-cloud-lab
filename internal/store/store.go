// Package store persists minicloud state as a local JSON file.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"minicloud/internal/models"
)

const (
	// DefaultFile is the JSON state filename written in the working directory.
	DefaultFile = "state.json"
)

var (
	// ErrNotInitialized is returned when state.json does not exist yet.
	ErrNotInitialized = errors.New("minicloud is not initialized; run 'minicloud init' first")
	// ErrAlreadyInitialized is returned when init is run and state.json already exists.
	ErrAlreadyInitialized = errors.New("minicloud is already initialized")
	// ErrNotFound is returned when a server id does not exist.
	ErrNotFound = errors.New("server not found")
)

// Store reads and writes a slice of servers to a JSON file.
type Store struct {
	Path string
}

// Default returns a store that uses state.json in the current working directory.
func Default() (*Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return &Store{Path: filepath.Join(cwd, DefaultFile)}, nil
}

// Exists reports whether the state file is present on disk.
func (s *Store) Exists() bool {
	_, err := os.Stat(s.Path)
	return err == nil
}

// Init creates an empty state.json file containing an empty server list.
func (s *Store) Init() error {
	if s.Exists() {
		return ErrAlreadyInitialized
	}
	return s.Write(nil)
}

// Read loads the slice of servers from state.json.
func (s *Store) Read() ([]models.Server, error) {
	if !s.Exists() {
		return nil, ErrNotInitialized
	}

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	// Empty or whitespace-only file → treat as no servers.
	if len(data) == 0 {
		return []models.Server{}, nil
	}

	var servers []models.Server
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if servers == nil {
		servers = []models.Server{}
	}
	return servers, nil
}

// Add appends a server to the existing state and persists it.
func (s *Store) Add(server models.Server) error {
	servers, err := s.Read()
	if err != nil {
		return err
	}
	servers = append(servers, server)
	return s.Write(servers)
}

// Get returns a server by id.
func (s *Store) Get(id string) (models.Server, error) {
	servers, err := s.Read()
	if err != nil {
		return models.Server{}, err
	}
	for _, srv := range servers {
		if srv.ID == id {
			return srv, nil
		}
	}
	return models.Server{}, fmt.Errorf("%w: %s", ErrNotFound, id)
}

// Update replaces the server with the same id.
func (s *Store) Update(server models.Server) error {
	servers, err := s.Read()
	if err != nil {
		return err
	}
	for i, srv := range servers {
		if srv.ID == server.ID {
			servers[i] = server
			return s.Write(servers)
		}
	}
	return fmt.Errorf("%w: %s", ErrNotFound, server.ID)
}

// Delete removes a server by id.
func (s *Store) Delete(id string) error {
	servers, err := s.Read()
	if err != nil {
		return err
	}
	out := make([]models.Server, 0, len(servers))
	found := false
	for _, srv := range servers {
		if srv.ID == id {
			found = true
			continue
		}
		out = append(out, srv)
	}
	if !found {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return s.Write(out)
}

// SetStatus updates status (and resets health to healthy when starting).
func (s *Store) SetStatus(id string, status models.Status) (models.Server, error) {
	srv, err := s.Get(id)
	if err != nil {
		return models.Server{}, err
	}
	srv.Status = status
	if status == models.StatusRunning {
		srv.Health = models.HealthHealthy
	}
	if err := s.Update(srv); err != nil {
		return models.Server{}, err
	}
	return srv, nil
}

// Replace overwrites the entire server list (creates state.json if missing).
func (s *Store) Replace(servers []models.Server) error {
	return s.Write(servers)
}

// Write saves the slice of servers to state.json (creates parent dirs if needed).
func (s *Store) Write(servers []models.Server) error {
	if servers == nil {
		servers = []models.Server{}
	}

	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	// Write via a temp file then rename for a safer replace.
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit state: %w", err)
	}
	return nil
}
