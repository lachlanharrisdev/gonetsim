package service

import (
	"context"
	"log/slog"
)

type Service interface {
	// unique identifier for the service, used in logging
	Name() string

	// start runs the service blocking until the service stops
	Start(ctx context.Context) error

	// stop gracefully shuts down the service within the given context deadline
	Stop(ctx context.Context) error
}

// optional interface implemented by services that want
// a logger injected by the service manager
type LoggerAware interface {
	SetLogger(logger *slog.Logger)
}
