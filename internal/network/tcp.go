package network

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/pcap"
)

// TCPHandler serves one accepted connection. conn is the fully-wrapped socket
// (capture recorder underneath, optional TLS above); ses is the connection's
// capture session, or nil when capture is disabled. Returning nil (or a
// deadline/close error) is a clean close.
type TCPHandler func(ctx context.Context, conn net.Conn, ses *pcap.Session) error

// ServeTCP accepts connections on addr until ctx is cancelled or the listener
// fails. Each accepted connection is handled in its own goroutine.
func ServeTCP(ctx context.Context, name, addr string, tlsCfg *tls.Config, run *pcap.Run, idle time.Duration, log *slog.Logger, h TCPHandler) error {
	iface, err := run.NewInterface("gonetsim " + name + " tcp")
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	stop := closeOnCancel(ctx, ln)
	defer stop()

	msg := "listening on " + addr
	if tlsCfg != nil {
		msg += " (tls)"
	}
	log.Info(msg)

	var cs connSet = newConnSet()
	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			cs.closeAll() // unblock handlers still serving connections
			wg.Wait()
			if expectedClose(err, ctx) {
				return nil
			}
			return err
		}
		cs.add(conn)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cs.remove(conn)
			serveConn(ctx, conn, tlsCfg, run, iface, idle, log, h)
		}()
	}
}

func serveConn(ctx context.Context, conn net.Conn, tlsCfg *tls.Config, run *pcap.Run, iface int, idle time.Duration, log *slog.Logger, h TCPHandler) {
	var ses *pcap.Session
	final := conn
	if run != nil {
		var rec net.Conn
		rec, ses = pcap.NewConn(conn, run, iface)
		if rec != conn {
			final = rec
		}
	}
	if tlsCfg != nil {
		// TLS above the recorder: ciphertext is captured exactly as it hits
		// the wire, and the session stays available for annotations.
		final = tls.Server(final, tlsCfg)
	}
	final = idleConn{Conn: final, timeout: idle}

	// close the fully-wrapped conn so the recorder emits FIN/FIN-ACK.
	defer func() { _ = final.Close() }()

	err := h(ctx, final, ses)
	switch {
	case err == nil,
		errors.Is(err, net.ErrClosed),
		errors.Is(err, os.ErrDeadlineExceeded),
		errors.Is(err, context.Canceled):
		log.Debug("connection from " + conn.RemoteAddr().String() + " closed")
	default:
		log.Info("connection from " + conn.RemoteAddr().String() + " failed: " + err.Error())
	}
}

// idleConn pushes the deadline forward on every read/write, capping how long a
// handler blocks on a quiet connection. A zero timeout disables the deadline.
// TLS operations pass through it.
type idleConn struct {
	net.Conn
	timeout time.Duration
}

func (c idleConn) Read(p []byte) (int, error) {
	if c.timeout > 0 {
		_ = c.SetDeadline(time.Now().Add(c.timeout))
	}
	return c.Conn.Read(p)
}

func (c idleConn) Write(p []byte) (int, error) {
	if c.timeout > 0 {
		_ = c.SetDeadline(time.Now().Add(c.timeout))
	}
	return c.Conn.Write(p)
}

type tlsIntros interface {
	ConnectionState() tls.ConnectionState
	HandshakeContext(ctx context.Context) error
}

func (c idleConn) ConnectionState() tls.ConnectionState {
	if tc, ok := c.Conn.(tlsIntros); ok {
		return tc.ConnectionState()
	}
	return tls.ConnectionState{}
}

func (c idleConn) HandshakeContext(ctx context.Context) error {
	if tc, ok := c.Conn.(tlsIntros); ok {
		return tc.HandshakeContext(ctx)
	}
	return nil
}

type connSet struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func newConnSet() connSet {
	return connSet{conns: make(map[net.Conn]struct{})}
}

func (cs *connSet) add(c net.Conn) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.conns[c] = struct{}{}
}

func (cs *connSet) remove(c net.Conn) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.conns, c)
}

func (cs *connSet) closeAll() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for c := range cs.conns {
		_ = c.Close()
	}
}
