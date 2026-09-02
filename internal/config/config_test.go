package config

import (
	"os"
	"path/filepath"
	"testing"
)

// / <summary>
// / verifies that a new config file is created when one doesn't exist, checks loading, & that a second call doesn't overwrite the file
// / </summary>
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

func TestLoadOrCreateWithOverrides_AppliesOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gonetsim.toml")

	res, err := LoadOrCreateWithOverrides(path, map[string]any{
		"http.mode":     "real",
		"http.root_dir": dir,
		"dns.enabled":   false,
	})
	if err != nil {
		t.Fatalf("LoadOrCreateWithOverrides: %v", err)
	}

	if res.Config.HTTP.Mode != "real" {
		t.Fatalf("expected http.mode=real, got %q", res.Config.HTTP.Mode)
	}
	if res.Config.HTTP.RootDir != dir {
		t.Fatalf("expected http.root_dir=%q, got %q", dir, res.Config.HTTP.RootDir)
	}
	if res.Config.DNS.Enabled {
		t.Fatalf("expected dns.enabled=false via override")
	}
}

func TestFirstExistingFile_PrefersLocalThenUserThenSystem(t *testing.T) {
	if _, ok := firstExistingFile([]string{
		"/definitely/not/here/gonetsim.toml",
		"/also/not/here/gonetsim.toml",
		"/nor/here/gonetsim.toml",
	}); ok {
		t.Fatalf("expected no existing file")
	}

	dir := t.TempDir()
	a := filepath.Join(dir, "a.toml")
	b := filepath.Join(dir, "b.toml")
	if err := os.WriteFile(a, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile a: %v", err)
	}
	if err := os.WriteFile(b, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile b: %v", err)
	}

	got, ok := firstExistingFile([]string{a, b})
	if !ok {
		t.Fatalf("expected to find a file")
	}
	if got != a {
		t.Fatalf("expected first (highest precedence) file %q, got %q", a, got)
	}
}
