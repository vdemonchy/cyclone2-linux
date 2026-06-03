package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	if got := Path(); got != "/cfg/cyclone2-battery/config.json" {
		t.Fatalf("got %q", got)
	}
}

func TestReadValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cdir := filepath.Join(dir, "cyclone2-battery")
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "config.json"), []byte(`{"interval_seconds":30}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if c.IntervalSeconds != 30 {
		t.Fatalf("got %d, want 30", c.IntervalSeconds)
	}
}

func TestReadMissingIsZeroNoError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := Read()
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if c.IntervalSeconds != 0 {
		t.Fatalf("got %d, want 0", c.IntervalSeconds)
	}
}
