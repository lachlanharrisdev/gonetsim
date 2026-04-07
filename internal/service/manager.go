package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type Manager struct {
	services        []Service
	shutdownTimeout time.Duration
}

func NewManager(shutdownTimeout time.Duration) *Manager {
	return &Manager{shutdownTimeout: shutdownTimeout}
}

func (m *Manager) Add(s Service) {
	m.services = append(m.services, s)
}

type serviceExit struct {
	svc Service
	err error
}

func (m *Manager) RunAll(ctx context.Context) error {
	return runServices(ctx, m.shutdownTimeout, m.services)
}

func (m *Manager) RunSingleService(ctx context.Context, s Service) error {
	return runServices(ctx, m.shutdownTimeout, []Service{s})
}

func runServices(ctx context.Context, shutdownTimeout time.Duration, services []Service) error {
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
		go runService(&wg, runCtx, service, exitCh)
	}

	runErr := waitForStopOrExit(ctx, exitCh, cancel)
	stopAll(shutdownTimeout, services)
	wg.Wait()

	if runErr != nil {
		return runErr
	}
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func waitForStopOrExit(ctx context.Context, exitCh <-chan serviceExit, cancel context.CancelFunc) error {
	select {
	case <-ctx.Done():
		return nil
	case ex := <-exitCh:
		cancel()
		return exitAsError(ex)
	}
}

func exitAsError(ex serviceExit) error {
	if ex.err == nil || errors.Is(ex.err, context.Canceled) {
		return fmt.Errorf("[%s] exited unexpectedly", ex.svc.Name())
	}
	return fmt.Errorf("[%s] %w", ex.svc.Name(), ex.err)
}

func stopAll(shutdownTimeout time.Duration, services []Service) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var stopWG sync.WaitGroup
	stopWG.Add(len(services))
	for _, svc := range services {
		service := svc
		go func() {
			defer stopWG.Done()
			if err := service.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("[%s] stop error: %v", service.Name(), err)
			}
		}()
	}
	stopWG.Wait()
}

func runService(wg *sync.WaitGroup, ctx context.Context, service Service, exitCh chan<- serviceExit) {
	defer wg.Done()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] recovered from panic: %v", service.Name(), r)
			exitCh <- serviceExit{svc: service, err: fmt.Errorf("panic: %v", r)}
		}
	}()

	log.Printf("[%s] starting...", service.Name())
	err := service.Start(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("[%s] stopped with error: %v", service.Name(), err)
	} else {
		log.Printf("[%s] stopped", service.Name())
	}
	exitCh <- serviceExit{svc: service, err: err}
}
