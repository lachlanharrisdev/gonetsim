package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCapture(t *testing.T) {
	t.Run("connection files", func(t *testing.T) {
		base := t.TempDir()
		store, err := NewStore(base, "test")
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}

		w, err := store.Conn("203.0.113.10:43210", time.Now())
		if err != nil {
			t.Fatalf("Conn: %v", err)
		}
		w.Write("", []byte("captured data"))
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		entries, err := os.ReadDir(filepath.Join(base, "test"))
		if err != nil || len(entries) != 1 {
			t.Fatalf("expected 1 capture file, got %d (%v)", len(entries), err)
		}
		if !strings.HasSuffix(entries[0].Name(), "203.0.113.10_43210.log") {
			t.Fatalf("unexpected capture file name %q", entries[0].Name())
		}
		data, _ := os.ReadFile(filepath.Join(base, "test", entries[0].Name()))
		if string(data) != "captured data" {
			t.Fatalf("unexpected capture content %q", data)
		}
	})

	t.Run("named sections", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "capture.log")
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		w := &Writer{f: f}
		w.Write("request", []byte("GET /"))
		w.Write("request", []byte("multi\nline\n"))
		w.Write("", []byte("raw"))
		if err := f.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		want := "=== request ===\nGET /\n=== request ===\nmulti\nline\nraw"
		if string(data) != want {
			t.Fatalf("unexpected content:\n%q\nwant:\n%q", data, want)
		}
	})

	t.Run("nil values are no-ops", func(t *testing.T) {
		var store *Store
		w, err := store.Conn("1.2.3.4:5", time.Now())
		if err != nil || w != nil {
			t.Fatalf("expected nil writer from nil store, got %v, %v", w, err)
		}
		w.Write("name", []byte("data"))
		if err := w.Close(); err != nil {
			t.Fatalf("Close on nil writer: %v", err)
		}
	})

	t.Run("sanitize", func(t *testing.T) {
		if got := sanitize("2001:db8::1%eth0/a b"); got != "2001_db8__1_eth0_a_b" {
			t.Fatalf("unexpected sanitized name %q", got)
		}
	})
}
