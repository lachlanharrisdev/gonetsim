package listener

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"

	"github.com/lachlanharrisdev/gonetsim/internal/handler"
)

const maxPacketSize = 65535

// udpService handles datagrams sequentially on one socket, so replies keep
// receive order and scripts never run concurrently.
type udpService struct {
	conf    Config
	handler handler.Handler
	log     *slog.Logger
	store   *captureStore
}

func (s *udpService) Name() string { return s.conf.Name }

func (s *udpService) Stop(_ context.Context) error { return nil }

func (s *udpService) Start(ctx context.Context) error {
	pc, err := net.ListenPacket("udp", s.conf.Addr)
	if err != nil {
		return err
	}
	defer pc.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = pc.Close()
		case <-done:
		}
	}()

	s.log.Info("listening", "on", s.conf.Addr, "handler", s.conf.HandlerSpec, "net", "udp")
	if err := s.readLoop(ctx, pc); err != nil && !errors.Is(err, net.ErrClosed) && ctx.Err() == nil {
		return err
	}
	s.store.closeAll()
	return nil
}

func (s *udpService) readLoop(ctx context.Context, pc net.PacketConn) error {
	buf := make([]byte, maxPacketSize)
	for {
		n, remote, err := pc.ReadFrom(buf)
		if err != nil {
			return err
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		w, err := s.store.writer(remote.String())
		if err != nil {
			s.log.Warn("capture unavailable", "remote", remote.String(), "err", err)
		}
		env := handler.Env{Logger: s.log, Capture: w}

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
