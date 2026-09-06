////----------------------------------------------------------------------------
// NOTICE: to save development time, test files (including this) have been
// generated with LLMs. The author(s) do not claim credit for these tests
// and exist purely for maximising code quality and reliability
//
// For more information please see `/.github/AI_USAGE.md`
//----------------------------------------------------------------------------//

package capture

import (
	"bytes"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

type frame struct {
	src, dst      netip.AddrPort
	syn, ack, fin bool
	seq, ackNum   uint32
	payload       string
}

func testRun(t *testing.T) (*Run, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.pcapng")
	run, err := NewRun(path)
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	t.Cleanup(func() { _ = run.Close() })
	if _, err := run.NewInterface("test"); err != nil {
		t.Fatalf("NewInterface: %v", err)
	}
	return run, path
}

func testSession(t *testing.T, run *Run, network string, local, remote netip.AddrPort) *Session {
	t.Helper()
	ses, err := run.NewSession(network, local, remote, 0)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return ses
}

func readFrames(t *testing.T, path string) []gopacket.Packet {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = f.Close() }()
	r, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("NewNgReader: %v", err)
	}
	if r.LinkType() != layers.LinkTypeEthernet {
		t.Fatalf("LinkType = %v, want Ethernet", r.LinkType())
	}
	var out []gopacket.Packet
	for {
		data, _, err := r.ReadPacketData()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadPacketData: %v", err)
		}
		p := gopacket.NewPacket(data, layers.LinkTypeEthernet, gopacket.Default)
		if p.ErrorLayer() != nil {
			t.Fatalf("packet failed to decode: %v", p.ErrorLayer().Error())
		}
		out = append(out, p)
	}
	return out
}

func checkTCP(t *testing.T, p gopacket.Packet, want frame) {
	t.Helper()
	tcp, ok := p.Layer(layers.LayerTypeTCP).(*layers.TCP)
	if !ok {
		t.Fatalf("packet is not TCP: %v", p.Layers())
	}
	if tcp.SrcPort != layers.TCPPort(want.src.Port()) || tcp.DstPort != layers.TCPPort(want.dst.Port()) {
		t.Errorf("ports = %s:%s, want %d:%d", tcp.SrcPort, tcp.DstPort, want.src.Port(), want.dst.Port())
	}
	if tcp.SYN != want.syn || tcp.ACK != want.ack || tcp.FIN != want.fin {
		t.Errorf("flags SYN=%v ACK=%v FIN=%v, want SYN=%v ACK=%v FIN=%v", tcp.SYN, tcp.ACK, tcp.FIN, want.syn, want.ack, want.fin)
	}
	if tcp.Seq != want.seq || tcp.Ack != want.ackNum {
		t.Errorf("seq/ack = %d/%d, want %d/%d", tcp.Seq, tcp.Ack, want.seq, want.ackNum)
	}
	if string(tcp.Payload) != want.payload {
		t.Errorf("payload = %q, want %q", tcp.Payload, want.payload)
	}
}

func TestSessionTCP(t *testing.T) {
	local := netip.MustParseAddrPort("127.0.0.1:8080")
	remote := netip.MustParseAddrPort("10.0.0.5:40000")
	run, path := testRun(t)

	ses := testSession(t, run, "tcp", local, remote)
	if err := ses.Write([]byte("hello"), true); err != nil {
		t.Fatalf("Write client: %v", err)
	}
	if err := ses.Write([]byte("world"), false); err != nil {
		t.Fatalf("Write server: %v", err)
	}
	if err := ses.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	pkts := readFrames(t, path)
	want := []frame{
		{remote, local, true, false, false, 0, 0, ""},
		{local, remote, true, true, false, 0, 1, ""},
		{remote, local, false, true, false, 1, 1, "hello"},
		{local, remote, false, true, false, 1, 6, "world"},
		{remote, local, false, true, true, 6, 6, ""},
		{local, remote, false, true, true, 6, 7, ""},
	}
	if len(pkts) != len(want) {
		t.Fatalf("got %d packets, want %d (SYN, SYN-ACK, 2 data, 2 FIN)", len(pkts), len(want))
	}
	for i, w := range want {
		checkTCP(t, pkts[i], w)
	}
}

func TestSessionTCPIPv6(t *testing.T) {
	local := netip.MustParseAddrPort("[::1]:8080")
	remote := netip.MustParseAddrPort("[2001:db8::5]:40000")
	run, path := testRun(t)

	ses := testSession(t, run, "tcp", local, remote)
	if err := ses.Write([]byte("ping"), true); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := ses.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	pkts := readFrames(t, path)
	if len(pkts) != 5 {
		t.Fatalf("got %d packets, want 5", len(pkts))
	}
	if pkts[0].Layer(layers.LayerTypeIPv6) == nil {
		t.Fatalf("expected IPv6 frames, got %v", pkts[0].Layers())
	}
}

func TestSessionUDP(t *testing.T) {
	local := netip.MustParseAddrPort("127.0.0.1:12345")
	remote := netip.MustParseAddrPort("10.0.0.5:5000")
	run, path := testRun(t)

	ses := testSession(t, run, "udp", local, remote)
	if err := ses.Write([]byte("query"), true); err != nil {
		t.Fatalf("Write client: %v", err)
	}
	if err := ses.Write([]byte("answer"), false); err != nil {
		t.Fatalf("Write server: %v", err)
	}
	if err := ses.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	pkts := readFrames(t, path)
	if len(pkts) != 2 {
		t.Fatalf("got %d packets, want 2", len(pkts))
	}
	for i, want := range []struct {
		src, dst netip.AddrPort
		payload  string
	}{
		{remote, local, "query"},
		{local, remote, "answer"},
	} {
		udp, ok := pkts[i].Layer(layers.LayerTypeUDP).(*layers.UDP)
		if !ok {
			t.Fatalf("packet %d is not UDP", i)
		}
		if udp.SrcPort != layers.UDPPort(want.src.Port()) || udp.DstPort != layers.UDPPort(want.dst.Port()) {
			t.Errorf("packet %d ports = %s:%s, want %d:%d", i, udp.SrcPort, udp.DstPort, want.src.Port(), want.dst.Port())
		}
		if string(udp.Payload) != want.payload {
			t.Errorf("packet %d payload = %q, want %q", i, udp.Payload, want.payload)
		}
	}
}

func TestSessionComment(t *testing.T) {
	local := netip.MustParseAddrPort("127.0.0.1:8080")
	remote := netip.MustParseAddrPort("10.0.0.5:40000")
	run, path := testRun(t)

	ses := testSession(t, run, "tcp", local, remote)
	ses.Comment("he-lo")
	if err := ses.Write([]byte("hello"), true); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := ses.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// the comment attaches to the next frame (here SYN); the file must still
	// parse cleanly as pcapng
	if !bytes.Contains(raw, []byte("he-lo")) {
		t.Fatal("comment text not found in pcapng bytes")
	}
	if pkts := readFrames(t, path); len(pkts) != 5 {
		t.Fatalf("got %d packets, want 5", len(pkts))
	}
}

func TestRun(t *testing.T) {
	t.Run("one file holds many flows", func(t *testing.T) {
		run, path := testRun(t)
		local := netip.MustParseAddrPort("127.0.0.1:53")
		remote := netip.MustParseAddrPort("203.0.113.10:43210")

		udp := testSession(t, run, "udp", local, remote)
		if err := udp.Write([]byte("query"), true); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := udp.Write([]byte("answer"), false); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := udp.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		tcp := testSession(t, run, "tcp", local, remote)
		if err := tcp.Write([]byte("hello"), true); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := tcp.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		info, err := Inspect(path)
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.LinkType != layers.LinkTypeEthernet || info.Packets != 7 {
			t.Fatalf("unexpected inspect result %+v", info)
		}
		if len(info.Interfaces) != 1 || info.Interfaces[0] != "test" {
			t.Fatalf("unexpected interfaces %+v", info.Interfaces)
		}
		if info.CreatedBy != "gonetsim" {
			t.Fatalf("unexpected created-by %q", info.CreatedBy)
		}
		if packets, _, _ := run.Stats(); packets != 7 {
			t.Fatalf("Stats packets = %d, want 7", packets)
		}
	})

	t.Run("run path resolution", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_DATA_HOME", dir)
		got, err := RunPath("")
		if err != nil {
			t.Fatalf("RunPath: %v", err)
		}
		wantDir := filepath.Join(dir, "gonetsim", "runs")
		if filepath.Dir(got) != wantDir || !strings.HasSuffix(got, ".pcapng") {
			t.Fatalf("RunPath = %q, want dir %q with .pcapng suffix", got, wantDir)
		}

		explicit := filepath.Join(dir, "case", "run.pcapng")
		got, err = RunPath(explicit)
		if err != nil {
			t.Fatalf("RunPath explicit: %v", err)
		}
		if got != explicit {
			t.Fatalf("RunPath explicit = %q, want %q", got, explicit)
		}
		if st, err := os.Stat(filepath.Join(dir, "case")); err != nil || !st.IsDir() {
			t.Fatalf("expected parent dir to be created: %v", err)
		}
	})
}

func TestInspect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inspect", "manual.pcapng")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	w, err := pcapgo.NewNgWriter(f, layers.LinkTypeEthernet)
	if err != nil {
		t.Fatalf("NewNgWriter: %v", err)
	}
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i, p := range [][]byte{{0xde, 0xad}, {0xca, 0xfe}} {
		ci := gopacket.CaptureInfo{Timestamp: ts.Add(time.Duration(i) * time.Second), CaptureLength: len(p), Length: len(p)}
		if err := w.WritePacket(ci, p); err != nil {
			t.Fatalf("WritePacket: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := Inspect(path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.LinkType != layers.LinkTypeEthernet {
		t.Fatalf("LinkType = %v, want Ethernet", info.LinkType)
	}
	if info.Packets != 2 {
		t.Fatalf("Packets = %d, want 2", info.Packets)
	}
	if !info.First.Equal(ts) || !info.Last.Equal(ts.Add(time.Second)) {
		t.Fatalf("First/Last timestamps = %v/%v, want %v/%v", info.First, info.Last, ts, ts.Add(time.Second))
	}

	legacy := filepath.Join(dir, "legacy.pcap")
	if err := os.WriteFile(legacy, []byte{0xd4, 0xc3, 0xb2, 0xa1}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := Inspect(legacy); err == nil || !strings.Contains(err.Error(), "legacy pcap") {
		t.Fatalf("expected legacy pcap error, got %v", err)
	}
}
