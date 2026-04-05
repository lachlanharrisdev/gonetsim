package config

import (
	"os"
	"path/filepath"
	"testing"
)

// / <summary>
// / verifies that a new config file is created when one doesn't exist, checks loading, & that a second call doesn't overwrite the file
func TestLoadOrCreate_CreatesAndLoadsConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gonetsim.toml")

	res1, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if !res1.Created {
		t.Fatalf("expected Created=true on first call")
	}
	if res1.Path != path {
		t.Fatalf("expected Path=%q, got %q", path, res1.Path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist at %q: %v", path, err)
	}
	if err := res1.Config.Validate(); err != nil {
		t.Fatalf("expected loaded config to validate: %v", err)
	}

	res2, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate (second): %v", err)
	}
	if res2.Created {
		t.Fatalf("expected Created=false on second call")
	}
	if res2.Path != path {
		t.Fatalf("expected Path=%q, got %q", path, res2.Path)
	}
}
