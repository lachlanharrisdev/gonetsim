package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// base directory capture files are written to, relative to cwd
const DefaultDir = "artifacts"

type Store struct {
	dir string
}

func NewStore(baseDir, listener string) (*Store, error) {
	dir := filepath.Join(baseDir, sanitize(listener))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create capture dir %q: %w", dir, err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Conn(remote string, now time.Time) (*Writer, error) {
	if s == nil {
		return nil, nil
	}
	name := now.Format("20060102-150405.000000") + "-" + sanitize(remote) + ".log"
	f, err := os.OpenFile(filepath.Join(s.dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create capture file: %w", err)
	}
	return &Writer{f: f}, nil
}

type Writer struct {
	f *os.File
}

func (w *Writer) Write(name string, data []byte) {
	if w == nil {
		return
	}
	if name != "" {
		_, _ = fmt.Fprintf(w.f, "=== %s ===\n", name)
	}
	_, _ = w.f.Write(data)
	if name != "" && (len(data) == 0 || data[len(data)-1] != '\n') {
		_, _ = w.f.Write([]byte{'\n'})
	}
}

func (w *Writer) Close() error {
	if w == nil || w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, s)
}
