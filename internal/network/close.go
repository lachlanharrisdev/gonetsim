package network

import (
	"context"
	"errors"
	"io"
	"net"
)

// closeOnCancel closes c when ctx is done. Call the returned stop func when
// the caller is shutting down anyway.
func closeOnCancel(ctx context.Context, c io.Closer) (stop func()) {
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-stopped:
		}
	}()
	return func() { close(stopped) }
}

// expectedClose reports whether err is the normal outcome of a listener being
// closed (shutdown) rather than a real failure.
func expectedClose(err error, ctx context.Context) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return true
	}
	return ctx.Err() != nil
}
