package pcap

import (
	"context"
	"crypto/tls"
	"net"
	"net/netip"
	"sync"
	"time"
)

// Conn wraps a socket so reads/writes are recorded into a Session. The wrapped
// conn is placed *under* any TLS layer, so a TLS listener records ciphertext
// exactly as it appears on the wire while still exposing the Session for
// capture:comment annotations.
type Conn struct {
	net.Conn
	ses *Session
}

// NewConn wraps c in a recorder when a run is active and the endpoints can be
// mapped to netip.AddrPort. Returns (c, nil) when there is nothing to record.
func NewConn(c net.Conn, run *Run, iface int) (net.Conn, *Session) {
	if run == nil {
		return c, nil
	}
	local, okL := toAddrPort(c.LocalAddr())
	remote, okR := toAddrPort(c.RemoteAddr())
	if !okL || !okR {
		return c, nil
	}
	ses, err := run.NewSession("tcp", local, remote, iface)
	if err != nil || ses == nil {
		return c, nil
	}
	return &Conn{Conn: c, ses: ses}, ses
}

func (c *Conn) Session() *Session {
	if c == nil {
		return nil
	}
	return c.ses
}

func (c *Conn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		_ = c.ses.Write(p[:n], true)
	}
	return n, err
}

func (c *Conn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 {
		_ = c.ses.Write(p[:n], false)
	}
	return n, err
}

func (c *Conn) Close() error {
	err := c.Conn.Close()
	if c.ses != nil {
		_ = c.ses.Close()
	}
	return err
}

func (c *Conn) ConnectionState() tls.ConnectionState {
	if tc, ok := c.Conn.(interface{ ConnectionState() tls.ConnectionState }); ok {
		return tc.ConnectionState()
	}
	return tls.ConnectionState{}
}

func (c *Conn) HandshakeContext(ctx context.Context) error {
	if tc, ok := c.Conn.(interface {
		HandshakeContext(ctx context.Context) error
	}); ok {
		return tc.HandshakeContext(ctx)
	}
	return nil
}

type udpFlow struct {
	ses  *Session
	last time.Time
}

// PacketConn wraps a UDP socket, tracking one Session per remote peer. Flows
// idle for longer than idle are closed so a long-running sim does not leak
// sessions or memory.
type PacketConn struct {
	net.PacketConn
	run   *Run
	iface int
	idle  time.Duration

	mu    sync.Mutex
	flows map[string]*udpFlow
}

func NewPacketConn(pc net.PacketConn, run *Run, iface int, idle time.Duration) *PacketConn {
	return &PacketConn{PacketConn: pc, run: run, iface: iface, idle: idle, flows: make(map[string]*udpFlow)}
}

func (c *PacketConn) SessionFor(remote net.Addr) *Session {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if f, ok := c.flows[remote.String()]; ok {
		return f.ses
	}
	return nil
}

func (c *PacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(p)
	if n > 0 {
		c.record(addr, p[:n], true)
	}
	return n, addr, err
}

func (c *PacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	n, err := c.PacketConn.WriteTo(p, addr)
	if n > 0 {
		c.record(addr, p[:n], false)
	}
	return n, err
}

func (c *PacketConn) record(remote net.Addr, data []byte, fromClient bool) {
	if c.run == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if c.idle > 0 {
		for k, f := range c.flows {
			if now.Sub(f.last) > c.idle {
				_ = f.ses.Close()
				delete(c.flows, k)
			}
		}
	}

	key := remote.String()
	f, ok := c.flows[key]
	if !ok {
		local, okL := toAddrPort(c.LocalAddr())
		rem, okR := toAddrPort(remote)
		if !okL || !okR {
			return
		}
		ses, err := c.run.NewSession("udp", local, rem, c.iface)
		if err != nil || ses == nil {
			return
		}
		f = &udpFlow{ses: ses}
		c.flows[key] = f
	}
	f.last = now
	_ = f.ses.Write(data, fromClient)
}

func (c *PacketConn) CloseAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, f := range c.flows {
		_ = f.ses.Close()
		delete(c.flows, k)
	}
}

func toAddrPort(a net.Addr) (netip.AddrPort, bool) {
	switch v := a.(type) {
	case *net.TCPAddr:
		return v.AddrPort(), true
	case *net.UDPAddr:
		return v.AddrPort(), true
	default:
		return netip.AddrPort{}, false
	}
}
