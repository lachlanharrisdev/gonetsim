package listener

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/handler"
	"github.com/lachlanharrisdev/gonetsim/internal/netx"
	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

const maxPacketSize = 65535

// udpService handles datagrams sequentially on one socket, so replies keep
// receive order and scripts never run concurrently.
type udpService struct {
	conf    Config
	handler handler.UDPHandler
	log     *slog.Logger
	run     *capture.Run
	global  *state.Store

	mu sync.Mutex
	pc *capture.PacketConn
}

func (s *udpService) Name() string { return s.conf.Name }

func (s *udpService) Stop(_ context.Context) error {
	s.mu.Lock()
	pc := s.pc
	s.mu.Unlock()
	if pc != nil {
		_ = pc.Close()
		pc.CloseAll()
	}
	return nil
}

func (s *udpService) Start(ctx context.Context) error {
	iface, err := s.run.NewInterface("gonetsim " + s.conf.Name + " udp")
	if err != nil {
		return err
	}
	rec, err := netx.ListenUDP(s.conf.Addr, s.run, iface, s.conf.ReadTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = rec.Close() }()

	s.mu.Lock()
	s.pc = rec
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.pc = nil
		s.mu.Unlock()
	}()

	done := netx.CloseOnCancel(ctx, rec)
	defer done()

	s.log.Info("listening", "on", s.conf.Addr, "handler", s.conf.HandlerSpec, "net", "udp")
	if err := s.readLoop(ctx, rec); err != nil && !netx.IsExpectedClose(err, ctx) {
		return err
	}
	rec.CloseAll()
	return nil
}

func (s *udpService) readLoop(ctx context.Context, pc *capture.PacketConn) error {
	buf := make([]byte, maxPacketSize)
	for {
		n, remote, err := pc.ReadFrom(buf)
		if err != nil {
			return err
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		env := handler.Env{Logger: s.log, Capture: pc.SessionFor(remote), Global: s.global}

		reply, err := s.handler.HandleUDP(ctx, data, remote, env)
		if err != nil {
			s.log.Info("packet handler error", "remote", remote.String(), "err", err)
			continue
		}
		if reply != nil {
			if _, err := pc.WriteTo(reply, remote); err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
				s.log.Debug("reply failed", "remote", remote.String(), "err", err)
			}
		}
	}
}
