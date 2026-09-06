package cmd

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
)

func writePcapFixture(t *testing.T, path string, payloads ...string) {
	t.Helper()
	local := netip.MustParseAddrPort("127.0.0.1:8080")
	remote := netip.MustParseAddrPort("10.0.0.5:40000")
	run, err := capture.NewRun(path)
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	defer func() { _ = run.Close() }()
	iface, err := run.NewInterface("test")
	if err != nil {
		t.Fatalf("NewInterface: %v", err)
	}
	ses, err := run.NewSession("tcp", local, remote, iface)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	for _, p := range payloads {
		if err := ses.Write([]byte(p), true); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := ses.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestInspectPcapFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flow.pcapng")
	writePcapFixture(t, path, "hello", "world")

	var out bytes.Buffer
	if err := inspectPcap(&out, path); err != nil {
		t.Fatalf("inspectPcap: %v", err)
	}
	got := out.String()
	for _, want := range []string{"format=pcapng", "packets=", "first=", "last=", "duration="} {
		if !strings.Contains(got, want) {
			t.Errorf("output %q missing %q", got, want)
		}
	}
}

func TestInspectPcapDir(t *testing.T) {
	dir := t.TempDir()
	writePcapFixture(t, filepath.Join(dir, "b.pcapng"), "one")
	writePcapFixture(t, filepath.Join(dir, "a.pcapng"), "one", "two")
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var out bytes.Buffer
	if err := inspectPcap(&out, dir); err != nil {
		t.Fatalf("inspectPcap: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "total: files=2") {
		t.Errorf("missing totals line: %q", got)
	}
	if strings.Contains(got, "notes.txt") {
		t.Errorf("non-pcapng file should be skipped: %q", got)
	}
	if a, b := strings.Index(got, "a.pcapng"), strings.Index(got, "b.pcapng"); a < 0 || b < 0 || a > b {
		t.Errorf("files should be listed sorted: %q", got)
	}
}

func TestInspectPcapDirWithBadFile(t *testing.T) {
	dir := t.TempDir()
	writePcapFixture(t, filepath.Join(dir, "good.pcapng"), "one")
	if err := os.WriteFile(filepath.Join(dir, "bad.pcapng"), []byte("not a capture"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var out bytes.Buffer
	err := inspectPcap(&out, dir)
	if err == nil || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("expected unreadable-file error, got %v", err)
	}
	if got := out.String(); !strings.Contains(got, "good.pcapng") || !strings.Contains(got, "bad.pcapng: ERROR") {
		t.Errorf("good files should still be listed alongside errors: %q", got)
	}
}

func TestInspectPcapFailures(t *testing.T) {
	var out bytes.Buffer
	if err := inspectPcap(&out, filepath.Join(t.TempDir(), "empty")); err == nil {
		t.Errorf("expected error for directory without captures")
	}
	if err := inspectPcap(&out, filepath.Join(t.TempDir(), "missing.pcapng")); err == nil {
		t.Errorf("expected error for missing file")
	}
	bad := filepath.Join(t.TempDir(), "bad.pcapng")
	if err := os.WriteFile(bad, []byte("not a capture"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := inspectPcap(&out, bad); err == nil {
		t.Errorf("expected error for corrupt file")
	}
}
