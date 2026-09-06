////----------------------------------------------------------------------------
// NOTICE: to save development time, test files (including this) have been
// generated with LLMs. The author(s) do not claim credit for these tests
// and exist purely for maximising code quality and reliability
//
// For more information please see `/.github/AI_USAGE.md`
//----------------------------------------------------------------------------//

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestListenersConfig(t *testing.T) {
	t.Run("parse and defaults", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "gonetsim.toml")
		content := `
[[listeners]]
name = "irc"
type = "tcp"
listen = ":6667"
handler = "lua:handlers/irc.lua"
read_timeout = "45s"

[[listeners]]
name = "off"
enabled = false
type = "tcp"
listen = ":1234"
handler = "builtin:echo"

[[listeners]]
name = "sink"
type = "udp"
listen = ":9999"
handler = "builtin:sink"
capture = false
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		res, err := LoadOrCreate(path)
		if err != nil {
			t.Fatalf("LoadOrCreate: %v", err)
		}
		if err := res.Config.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(res.Config.Listeners) != 3 {
			t.Fatalf("expected 3 listeners, got %d", len(res.Config.Listeners))
		}

		irc := res.Config.Listeners[0]
		if irc.Name != "irc" || irc.Type != "tcp" || irc.Listen != ":6667" || irc.Handler != "lua:handlers/irc.lua" {
			t.Fatalf("unexpected irc listener %+v", irc)
		}
		if !irc.IsEnabled() || !irc.ShouldCapture() {
			t.Fatalf("expected enabled and capture to default true")
		}
		if irc.ReadTimeout != 45*time.Second {
			t.Fatalf("expected read_timeout 45s, got %v", irc.ReadTimeout)
		}
		if res.Config.Listeners[1].IsEnabled() {
			t.Fatalf("expected enabled=false to be respected")
		}
		if res.Config.Listeners[2].ShouldCapture() {
			t.Fatalf("expected capture=false to be respected")
		}
	})

	t.Run("duplicate names rejected", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "gonetsim.toml")
		content := `
[[listeners]]
name = "dup"
type = "tcp"
listen = ":1"
handler = "builtin:echo"

[[listeners]]
name = "dup"
type = "tcp"
listen = ":2"
handler = "builtin:echo"
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		res, err := LoadOrCreate(path)
		if err != nil {
			t.Fatalf("LoadOrCreate: %v", err)
		}
		if err := res.Config.Validate(); err == nil {
			t.Fatalf("expected duplicate listener name error")
		}
	})
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

func TestLegacyCaptureDirIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gonetsim.toml")
	content := `
[http]
enabled = true
listen = "127.0.0.1:0"
capture = true
capture_dir = "/tmp/should-be-ignored"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if err := res.Config.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !res.Config.HTTP.Capture {
		t.Fatalf("expected http.capture to survive")
	}
}
