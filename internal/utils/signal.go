package utils

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
