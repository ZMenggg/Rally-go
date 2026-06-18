package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rally.pid")

	writePIDFile(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}
	if got, want := string(data), fmt.Sprintf("%d", os.Getpid()); got != want {
		t.Fatalf("PID file = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat PID file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("PID file permissions = %v, want 0600", got)
	}
}
