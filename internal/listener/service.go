package listener

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/handler"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
)

// NewService resolves the handler spec (compiling Lua scripts) so
// misconfiguration fails at startup rather than per connection.
func NewService(conf Config, logger *slog.Logger) (service.Service, error) {
	if err := conf.Validate(); err != nil {
		return nil, fmt.Errorf("listener %s: %w", conf.Name, err)
	}
	h, err := handler.New(conf.HandlerSpec, conf.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("listener %s handler: %w", conf.Name, err)
	}

	log := service.NewPrefixedLogger(logger, conf.Name)
	store := &captureStore{store: nil}
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
		return &udpService{conf: conf, handler: h, log: log, store: store}, nil
	}
	return &tcpService{conf: conf, handler: h, log: log, store: store}, nil
}

type captureStore struct {
	store *capture.Store

	mu      sync.Mutex
	writers map[string]*capture.Writer
}

func (cs *captureStore) writer(key string) (*capture.Writer, error) {
	if cs.store == nil {
		return nil, nil
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if w, ok := cs.writers[key]; ok {
		return w, nil
	}
	w, err := cs.store.Conn(key, time.Now())
	if err != nil {
		return nil, err
	}
	if cs.writers == nil {
		cs.writers = make(map[string]*capture.Writer)
	}
	cs.writers[key] = w
	return w, nil
}

func (cs *captureStore) closeAll() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, w := range cs.writers {
		_ = w.Close()
	}
	cs.writers = nil
}

func (cs *captureStore) release(key string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if w, ok := cs.writers[key]; ok {
		delete(cs.writers, key)
		_ = w.Close()
	}
}
