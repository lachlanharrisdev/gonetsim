package listener

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/handler"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

func NewService(conf Config, global *state.Store, logger *slog.Logger) (service.Service, error) {
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
	store := &captureStore{}
	if conf.Capture {
		baseDir := conf.CaptureDir
		if baseDir == "" {
			baseDir = capture.DefaultDir
		}
		cs, err := capture.NewStore(baseDir, conf.Name)
		if err != nil {
			return nil, fmt.Errorf("listener %s: %w", conf.Name, err)
		}
		store.store = cs
	}
	if conf.Network == "udp" {
		store.idle = conf.ReadTimeout
		return &udpService{conf: conf, handler: h, log: log, store: store, global: global}, nil
	}
	return &tcpService{conf: conf, handler: h, log: log, store: store, global: global}, nil
}

type captureStore struct {
	store *capture.Store
	idle  time.Duration

	mu      sync.Mutex
	entries map[string]*captureEntry
}

type captureEntry struct {
	w    *capture.Writer
	last time.Time
}

func (cs *captureStore) writer(key string) (*capture.Writer, error) {
	if cs.store == nil {
		return nil, nil
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now()
	if cs.idle > 0 {
		for k, e := range cs.entries {
			if now.Sub(e.last) > cs.idle {
				_ = e.w.Close()
				delete(cs.entries, k)
			}
		}
	}
	if e, ok := cs.entries[key]; ok {
		e.last = now
		return e.w, nil
	}
	w, err := cs.store.Conn(key, now)
	if err != nil {
		return nil, err
	}
	if cs.entries == nil {
		cs.entries = make(map[string]*captureEntry)
	}
	cs.entries[key] = &captureEntry{w: w, last: now}
	return w, nil
}

func (cs *captureStore) closeAll() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, e := range cs.entries {
		_ = e.w.Close()
	}
	cs.entries = nil
}

func (cs *captureStore) release(key string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if e, ok := cs.entries[key]; ok {
		delete(cs.entries, key)
		_ = e.w.Close()
	}
}
