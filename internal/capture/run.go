package capture

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

type Run struct {
	mu      sync.Mutex
	f       *os.File
	path    string
	ifaces  int
	packets uint64
	first   time.Time
	last    time.Time
}

func NewRunID() string {
	var suffix [2]byte
	_, _ = rand.Read(suffix[:])
	return time.Now().Format("20060102-150405") + fmt.Sprintf("-%02x%02x", suffix[0], suffix[1])
}

func DefaultRunsDir() (string, error) {
	// XDG_DATA_HOME is honored on all platforms so tests can redirect the
	// runs directory via t.Setenv (os.UserCacheDir ignores it on Windows).
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gonetsim", "runs"), nil
	}
	var base string
	switch runtime.GOOS {
	case "windows":
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		base = dir
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "gonetsim", "runs"), nil
}

func RunPath(output string) (string, error) {
	if output != "" {
		if dir := filepath.Dir(output); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("create output dir %q: %w", dir, err)
			}
		}
		return output, nil
	}
	dir, err := DefaultRunsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create runs dir %q: %w", dir, err)
	}
	return filepath.Join(dir, NewRunID()+".pcapng"), nil
}

func NewRun(path string) (*Run, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create pcapng %q: %w", path, err)
	}
	r := &Run{f: f, path: path}
	if err := r.writeSHB(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return r, nil
}

func (r *Run) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

func (r *Run) NewInterface(name string) (int, error) {
	if r == nil {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	opt := encodeOption(2, append([]byte(name), 0))
	opt = append(opt, encodeOption(0, nil)...)
	length := 16 + len(opt) + 4
	b := make([]byte, 16)
	binary.LittleEndian.PutUint32(b[0:4], 1)
	binary.LittleEndian.PutUint32(b[4:8], uint32(length))
	binary.LittleEndian.PutUint16(b[8:10], uint16(layers.LinkTypeEthernet))
	binary.LittleEndian.PutUint16(b[10:12], 0)
	binary.LittleEndian.PutUint32(b[12:16], snapLen)
	if err := writeAll(r.f, b); err != nil {
		return 0, err
	}
	if err := writeAll(r.f, opt); err != nil {
		return 0, err
	}
	if err := r.writeTrailerLocked(length); err != nil {
		return 0, err
	}
	id := r.ifaces
	r.ifaces++
	return id, nil
}

func (r *Run) NewSession(network string, local, remote netip.AddrPort, iface int) (*Session, error) {
	if r == nil {
		return nil, nil
	}
	return &Session{run: r, netw: network, local: local, remote: remote, iface: iface}, nil
}

func (r *Run) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}

func (r *Run) Stats() (packets uint64, first, last time.Time) {
	if r == nil {
		return 0, time.Time{}, time.Time{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.packets, r.first, r.last
}

func (r *Run) writeSHB() error {
	opt := encodeOption(2, []byte("GoNetSim simulated network"))
	opt = append(opt, encodeOption(3, []byte(runtime.GOOS+"/"+runtime.GOARCH))...)
	opt = append(opt, encodeOption(4, []byte("gonetsim"))...)
	opt = append(opt, encodeOption(0, nil)...)
	length := 28 + len(opt)
	b := make([]byte, 24)
	binary.LittleEndian.PutUint32(b[0:4], 0x0A0D0D0A)
	binary.LittleEndian.PutUint32(b[4:8], uint32(length))
	binary.LittleEndian.PutUint32(b[8:12], 0x1A2B3C4D)
	binary.LittleEndian.PutUint16(b[12:14], 1)
	binary.LittleEndian.PutUint16(b[14:16], 0)
	binary.LittleEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
	if _, err := r.f.Write(b); err != nil {
		return err
	}
	if _, err := r.f.Write(opt); err != nil {
		return err
	}
	return r.writeTrailerLocked(length)
}

func (r *Run) writeTrailerLocked(length int) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(length))
	_, err := r.f.Write(b[:])
	return err
}
