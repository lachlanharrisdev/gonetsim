package listener

import (
	"fmt"
	"log/slog"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/handler"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

func NewService(conf Config, global *state.Store, logger *slog.Logger, run *capture.Run) (service.Service, error) {
	if global == nil {
		global = state.NewStore(nil)
	}
	if err := conf.Validate(); err != nil {
		return nil, fmt.Errorf("listener %s: %w", conf.Name, err)
	}
	h, err := handler.New(conf.HandlerSpec, conf.BaseDir, global.Budget())
	if err != nil {
		return nil, fmt.Errorf("listener %s handler: %w", conf.Name, err)
	}

	log := service.NewPrefixedLogger(logger, conf.Name)
	if !conf.Capture {
		run = nil
	}
	if conf.Network == "udp" {
		return &udpService{conf: conf, handler: h, log: log, run: run, global: global}, nil
	}
	return &tcpService{conf: conf, handler: h, log: log, run: run, global: global}, nil
}
