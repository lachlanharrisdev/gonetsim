package listener

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/handler"
)

type tcpService struct {
	conf    Config
	handler handler.Handler
	log     *slog.Logger
	store   *captureStore

	conns connSet
	wg    sync.WaitGroup
}

func (s *tcpService) Name() string { return s.conf.Name }

func (s *tcpService) Stop(_ context.Context) error { return nil }

func (s *tcpService) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.conf.Addr)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	if s.conf.TLS != nil {
		tlsConf, err := s.conf.TLS.TLSConfig()
		if err != nil {
			return err
		}
		ln = tls.NewListener(ln, tlsConf)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-done:
		}
	}()

	s.log.Info("listening", "on", s.conf.Addr, "handler", s.conf.HandlerSpec)
	if err := s.accept(ctx, ln); err != nil && !errors.Is(err, net.ErrClosed) && ctx.Err() == nil {
		return err
	}

	s.conns.closeAll() // unblock handlers still serving connections
	s.wg.Wait()
	return nil
}

func (s *tcpService) accept(ctx context.Context, ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		s.conns.add(conn)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(ctx, conn)
			s.conns.remove(conn)
		}()
	}
}

func (s *tcpService) handleConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	remote := conn.RemoteAddr().String()
	defer s.store.release(remote)

	w, err := s.store.writer(remote)
	if err != nil {
		s.log.Warn("capture unavailable", "remote", remote, "err", err)
	}
	conn = newIdleConn(conn, s.conf.ReadTimeout)

	env := handler.Env{Logger: s.log, Capture: w, IdleTimeout: s.conf.ReadTimeout}
	err = s.handler.HandleTCP(ctx, conn, env)
	switch {
	case err == nil,
		errors.Is(err, net.ErrClosed),
		errors.Is(err, os.ErrDeadlineExceeded),
		errors.Is(err, context.Canceled):
		s.log.Debug("connection closed", "remote", remote)
	default:
		s.log.Info("connection handler error", "remote", remote, "err", err)
	}
}

// idleConn pushes the deadline forward on every read/write, capping how long
// a handler blocks on a quiet connection.
type idleConn struct {
	net.Conn
	timeout time.Duration
}

func newIdleConn(conn net.Conn, timeout time.Duration) idleConn {
	return idleConn{Conn: conn, timeout: timeout}
}

func (c idleConn) Read(p []byte) (int, error) {
	_ = c.SetDeadline(time.Now().Add(c.timeout))
	return c.Conn.Read(p)
}

func (c idleConn) Write(p []byte) (int, error) {
	_ = c.SetDeadline(time.Now().Add(c.timeout))
	return c.Conn.Write(p)
}

// ConnectionState/HandshakeContext forward TLS operations through the wrapper.
func (c idleConn) ConnectionState() tls.ConnectionState {
	if tc, ok := c.Conn.(interface {
		ConnectionState() tls.ConnectionState
		HandshakeContext(ctx context.Context) error
	}); ok {
		return tc.ConnectionState()
	}
	return tls.ConnectionState{}
}

func (c idleConn) HandshakeContext(ctx context.Context) error {
	if tc, ok := c.Conn.(interface {
		ConnectionState() tls.ConnectionState
		HandshakeContext(ctx context.Context) error
	}); ok {
		return tc.HandshakeContext(ctx)
	}
	return nil
}

type connSet struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func (cs *connSet) add(c net.Conn) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.conns == nil {
		cs.conns = make(map[net.Conn]struct{})
	}
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
