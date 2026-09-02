package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeService struct {
	name     string
	startErr error
	started  chan struct{}
	stopped  chan struct{}
	block    bool
}

func (f *fakeService) Name() string { return f.name }

func (f *fakeService) Start(ctx context.Context) error {
	close(f.started)
	if f.block {
		<-ctx.Done()
	}
	return f.startErr
}

func (f *fakeService) Stop(ctx context.Context) error {
	close(f.stopped)
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunServices_PropagatesStartError(t *testing.T) {
	svc := &fakeService{
		name:     "boom",
		startErr: errors.New("bind: address already in use"),
		started:  make(chan struct{}),
		stopped:  make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runServices(ctx, discardLogger(), time.Second, []Service{svc})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to name the failing service, got %q", err)
	}
	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected the underlying error to be preserved, got %q", err)
	}
}

func TestRunServices_ReturnsNilOnCleanShutdown(t *testing.T) {
	svc := &fakeService{
		name:    "ok",
		block:   true,
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runServices(ctx, discardLogger(), time.Second, []Service{svc})
	}()

	<-svc.started
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil on clean shutdown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("manager did not return after cancellation")
	}
}
