package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Manager struct {
	services        []Service
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

func NewManager(shutdownTimeout time.Duration, logger *slog.Logger) *Manager {
	return &Manager{shutdownTimeout: shutdownTimeout, logger: logger}
}

func (m *Manager) Add(s Service) {
	m.services = append(m.services, s)
}

type serviceExit struct {
	svc Service
	err error
}

func (m *Manager) RunAll(ctx context.Context) error {
	return runServices(ctx, m.logger, m.shutdownTimeout, m.services)
}

func (m *Manager) RunSingleService(ctx context.Context, s Service) error {
	return runServices(ctx, m.logger, m.shutdownTimeout, []Service{s})
}

func runServices(ctx context.Context, logger *slog.Logger, shutdownTimeout time.Duration, services []Service) error {
	if len(services) == 0 {
		return nil
	}
	if shutdownTimeout <= 0 {
		shutdownTimeout = 5 * time.Second
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	exitCh := make(chan serviceExit, len(services))

	var wg sync.WaitGroup
	wg.Add(len(services))
	for _, svc := range services {
		service := svc
		go runService(&wg, runCtx, logger, service, exitCh)
	}

	logger.Info("starting services", "count", len(services))

	running := len(services)
	exited := make(map[Service]bool)

	for running > 0 {
		select {
		case <-ctx.Done():
			cancel()
			stopRemaining(logger, shutdownTimeout, services, exited)
			wg.Wait()
			return nil
		case ex := <-exitCh:
			running--
			exited[ex.svc] = true
		}
	}

	wg.Wait()
	return nil
}

func stopRemaining(logger *slog.Logger, shutdownTimeout time.Duration, services []Service, exited map[Service]bool) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var stopWG sync.WaitGroup
	for _, svc := range services {
		if exited[svc] {
			continue
		}
		service := svc
		stopWG.Add(1)
		go func() {
			defer stopWG.Done()
			if err := service.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("service stop error", "service", service.Name(), "err", err)
			}
		}()
	}
	stopWG.Wait()
}

func runService(wg *sync.WaitGroup, ctx context.Context, rootLogger *slog.Logger, service Service, exitCh chan<- serviceExit) {
	defer wg.Done()
	logger := NewPrefixedLogger(rootLogger, service.Name())

	defer func() {
		if r := recover(); r != nil {
			logger.Error("recovered from panic", "panic", r)
			exitCh <- serviceExit{svc: service, err: fmt.Errorf("panic: %v", r)}
		}
	}()

	logger.Info("starting")
	err := service.Start(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("stopped with error", "err", err)
	} else {
		logger.Info("stopped")
	}
	exitCh <- serviceExit{svc: service, err: err}
}
