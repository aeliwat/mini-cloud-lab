package store

import (
	"path/filepath"
	"testing"

	"minicloud/internal/models"
)

func TestInitReadWrite(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Path: filepath.Join(dir, DefaultFile)}

	if s.Exists() {
		t.Fatal("expected store not to exist yet")
	}

	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Init(); err != ErrAlreadyInitialized {
		t.Fatalf("second Init error = %v, want %v", err, ErrAlreadyInitialized)
	}

	servers, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("len(servers) = %d, want 0", len(servers))
	}

	want := []models.Server{
		{ID: "srv-1", Role: models.RoleWeb, RAM: "2G", Disk: "50G", Status: models.StatusRunning},
	}
	if err := s.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := s.Read()
	if err != nil {
		t.Fatalf("Read after write: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0] != want[0] {
		t.Fatalf("got %+v, want %+v", got[0], want[0])
	}
}

func TestReadNotInitialized(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "missing.json")}
	_, err := s.Read()
	if err != ErrNotInitialized {
		t.Fatalf("Read error = %v, want %v", err, ErrNotInitialized)
	}
}

func TestAdd(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Path: filepath.Join(dir, DefaultFile)}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	srv := models.Server{
		ID: "srv-1", Role: models.RoleDB, RAM: "4G", Disk: "100G", Status: models.StatusRunning,
	}
	if err := s.Add(srv); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 || got[0] != srv {
		t.Fatalf("got %+v, want [%+v]", got, srv)
	}
}
