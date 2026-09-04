package listener

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

type Config struct {
	Name        string
	Network     string // "tcp" or "udp"
	Addr        string
	HandlerSpec string
	ReadTimeout time.Duration // idle timeout applied to each connection
	TLS         *tlsprovider.Config
	Capture     bool
	// BaseDir is the directory relative handler script paths resolve against.
	BaseDir string
	// CaptureDir overrides the base directory for capture files.
	// When empty, capture.DefaultDir is used.
	CaptureDir string
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("name is required")
	}
	if c.Network == "" {
		return errors.New("network is required")
	}
	switch c.Network {
	case "tcp", "udp":
		// ok
	default:
		return errors.New("network must be one of: tcp, udp")
	}
	if c.Addr == "" {
		return errors.New("listen addr is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", c.Addr); err != nil {
		return fmt.Errorf("invalid listen addr %q (expected host:port): %w", c.Addr, err)
	}
	if strings.TrimSpace(c.HandlerSpec) == "" {
		return errors.New("handler is required")
	}
	if c.ReadTimeout <= 0 {
		return errors.New("read_timeout must be > 0")
	}
	if c.TLS != nil && c.Network != "tcp" {
		return errors.New("tls is only supported on tcp listeners")
	}
	return nil
}
