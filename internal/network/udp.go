package network

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/pcap"
)

const maxPacketSize = 65535

// UDPHandler processes one datagram and returns an optional reply. ses is the
// per-peer capture session, or nil when capture is disabled.
type UDPHandler func(ctx context.Context, data []byte, from net.Addr, ses *pcap.Session) ([]byte, error)

// ServeUDP handles datagrams sequentially on one socket so replies keep
// receive order and handlers never run concurrently.
func ServeUDP(ctx context.Context, name, addr string, run *pcap.Run, idle time.Duration, log *slog.Logger, h UDPHandler) error {
	iface, err := run.NewInterface("gonetsim " + name + " udp")
	if err != nil {
		return err
	}

	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = pc.Close() }()

	var rec *pcap.PacketConn
	if run != nil {
		rec = pcap.NewPacketConn(pc, run, iface, idle)
		pc = rec
	}
	if rec != nil {
		defer rec.CloseAll()
	}

	stop := closeOnCancel(ctx, pc)
	defer stop()

	log.Info("listening on " + addr + " (udp)")

	buf := make([]byte, maxPacketSize)
	for {
		n, remote, err := pc.ReadFrom(buf)
		if err != nil {
			if expectedClose(err, ctx) {
				return nil
			}
			return err
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		var ses *pcap.Session
		if rec != nil {
			ses = rec.SessionFor(remote)
		}

		reply, err := h(ctx, data, remote, ses)
		if err != nil {
			log.Info("packet from " + remote.String() + " failed: " + err.Error())
			continue
		}
		if reply != nil {
			if _, err := pc.WriteTo(reply, remote); err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
				log.Debug("reply to " + remote.String() + " failed: " + err.Error())
			}
		}
	}
}
