package service

import (
	"context"
	"log"
	"sync"
	"time"
)

type Manager struct {
	services        []Service
	wg              sync.WaitGroup
	shutdownTimeout time.Duration
}

func NewManager(shutdownTimeout time.Duration) *Manager {
	return &Manager{shutdownTimeout: shutdownTimeout}
}

func (m *Manager) Add(s Service) {
	m.services = append(m.services, s)
}

func (m *Manager) RunAll(ctx context.Context) {
	for _, s := range m.services {
		service := s
		InstantiateService(m, ctx, service)
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	for _, s := range m.services {
		service := s
		go func() {
			if err := service.Stop(shutdownCtx); err != nil && err != context.Canceled {
				log.Printf("[%s] stop error: %v", service.Name(), err)
			}
		}()
	}

	// block until all services have exited
	m.wg.Wait()
}

func (m *Manager) RunSingleService(ctx context.Context, s Service) {
	InstantiateService(m, ctx, s)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	go func() {
		if err := s.Stop(shutdownCtx); err != nil && err != context.Canceled {
			log.Printf("[%s] stop error: %v", s.Name(), err)
		}
	}()

	m.wg.Wait()
}

func InstantiateService(m *Manager, ctx context.Context, service Service) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		defer func() {
			if r := recover(); r != nil {
				log.Printf("[%s] recovered from panic: %v", service.Name(), r)
			}
		}()

		log.Printf("[%s] starting...", service.Name())
		if err := service.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("[%s] stopped with error: %v", service.Name(), err)
		} else {
			log.Printf("[%s] stopped", service.Name())
		}
	}()
}
