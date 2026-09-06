package capture

import (
	"context"
	"crypto/tls"
	"net"
	"net/netip"
	"sync"
	"time"
)

type Conn struct {
	net.Conn
	ses *Session
}

type ConnListener struct {
	net.Listener
	run   *Run
	iface int
}

type udpFlow struct {
	ses  *Session
	last time.Time
}

type PacketConn struct {
	net.PacketConn
	run   *Run
	iface int
	idle  time.Duration

	mu    sync.Mutex
	flows map[string]*udpFlow
}

func NewConnListener(ln net.Listener, run *Run, iface int) net.Listener {
	if run == nil {
		return ln
	}
	return &ConnListener{Listener: ln, run: run, iface: iface}
}

func (l *ConnListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewConn(c, l.run, l.iface), nil
}

func NewConn(c net.Conn, run *Run, iface int) net.Conn {
	if run == nil {
		return c
	}
	local, okL := toAddrPort(c.LocalAddr())
	remote, okR := toAddrPort(c.RemoteAddr())
	if !okL || !okR {
		return c
	}
	ses, err := run.NewSession("tcp", local, remote, iface)
	if err != nil || ses == nil {
		return c
	}
	return &Conn{Conn: c, ses: ses}
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
	_ = f.ses.Flush()
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
