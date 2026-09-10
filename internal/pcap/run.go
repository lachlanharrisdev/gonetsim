// Package pcap writes simulated network traffic to a pcapng file.
//
// GoNetSim does not sniff anything: every handler read/write is synthesised
// into fake Ethernet/IP/TCP|UDP frames and appended to a single run file that
// opens in Wireshark, tshark, or any pcapng reader. Handshakes are synthesised
// (sequence numbers start at 0, MACs are fake) and TLS sessions capture
// ciphertext as seen on the socket, not plaintext.
package pcap

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

// Run is a single capture file: one section header, one interface block per
// listener, and one enhanced packet block per frame.
type Run struct {
	mu       sync.Mutex
	f        *os.File
	path     string
	manifest string
	ifaces   int
	packets  uint64
	first    time.Time
	last     time.Time
	off      int64 // current append offset, for retroactive annotations
}

func NewRunID() string {
	var suffix [2]byte
	_, _ = rand.Read(suffix[:])
	return time.Now().Format("20060102-150405") + fmt.Sprintf("-%02x%02x", suffix[0], suffix[1])
}

// DefaultPath returns output if set (creating its parent directory), otherwise
// a fresh single-file path in the current directory. Relative by default so it
// works anywhere without platform-specific home-directory lookups.
func DefaultPath(output string) (string, error) {
	if output != "" {
		if dir := filepath.Dir(output); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("create output dir %q: %w", dir, err)
			}
		}
		return output, nil
	}
	return NewRunID() + ".pcapng", nil
}

// NewRun opens path and writes the section header block. When manifest is
// non-empty it is embedded in the SHB as an extra comment option so the single
// file is self-describing to an analyst opening it in Wireshark.
func NewRun(path string, manifest ...string) (*Run, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create pcapng %q: %w", path, err)
	}
	r := &Run{f: f, path: path}
	if len(manifest) > 0 {
		r.manifest = manifest[0]
	}
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
	if err := r.write(b); err != nil {
		return 0, err
	}
	if err := r.write(opt); err != nil {
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
	opt = append(opt, encodeOption(3, []byte("gonetsim"))...)
	if r.manifest != "" {
		opt = append(opt, encodeOption(2, []byte(r.manifest))...)
	}
	opt = append(opt, encodeOption(0, nil)...)
	length := 28 + len(opt)
	b := make([]byte, 24)
	binary.LittleEndian.PutUint32(b[0:4], 0x0A0D0D0A)
	binary.LittleEndian.PutUint32(b[4:8], uint32(length))
	binary.LittleEndian.PutUint32(b[8:12], 0x1A2B3C4D)
	binary.LittleEndian.PutUint16(b[12:14], 1)
	binary.LittleEndian.PutUint16(b[14:16], 0)
	binary.LittleEndian.PutUint64(b[16:24], 0xFFFFFFFFFFFFFFFF)
	if err := r.write(b); err != nil {
		return err
	}
	if err := r.write(opt); err != nil {
		return err
	}
	return r.writeTrailerLocked(length)
}

func (r *Run) writeTrailerLocked(length int) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(length))
	return r.write(b[:])
}

// write appends b to the capture and tracks the file offset so sessions can
// retroactively annotate already-written packets. Callers hold r.mu.
func (r *Run) write(b []byte) error {
	n, err := r.f.Write(b)
	r.off += int64(n)
	return err
}
