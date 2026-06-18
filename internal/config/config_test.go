package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rally.yaml")
	cfg := &Config{
		Bind:    ":1080",
		Balance: "roundrobin",
		VPS: []VPS{{
			Name:     "node-1",
			Server:   "example.com",
			Port:     443,
			Password: "secret",
		}},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config permissions = %v, want 0600", got)
	}
}
