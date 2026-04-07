package service

import "context"

type Service interface {
	// unique identifier for the service, used in logging
	Name() string

	// start runs the service blocking until the service stops
	Start(ctx context.Context) error

	// stop gracefully shuts down the service within the given context deadline
	Stop(ctx context.Context) error
}
