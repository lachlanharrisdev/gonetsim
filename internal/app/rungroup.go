// Package app runs a set of blocking server functions together: it starts them
// all, cancels the rest when one fails or the parent context ends, and waits
// for every goroutine to finish before returning.
package app

import (
	"context"
	"errors"
	"sync"
)

// RunAll runs fns concurrently until every fn returns, the first error occurs
// (the rest are cancelled), or ctx is done. The first non-cancellation error
// is returned.
func RunAll(ctx context.Context, fns ...func(context.Context) error) error {
	if len(fns) == 0 {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Buffered so each goroutine exits without waiting on the main select.
	errc := make(chan error, len(fns))
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Add(1)
		go func(f func(context.Context) error) {
			defer wg.Done()
			errc <- f(runCtx)
		}(fn)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var firstErr error
	for {
		select {
		case <-ctx.Done():
			cancel()
			<-done
			return nil
		case err := <-errc:
			if err != nil && !errors.Is(err, context.Canceled) && firstErr == nil {
				firstErr = err
				cancel()
			}
		case <-done:
			// All goroutines have finished sending; the channel is full to
			// capacity. Drain whatever the main loop hasn't consumed yet.
			close(errc)
			for err := range errc {
				if err != nil && !errors.Is(err, context.Canceled) && firstErr == nil {
					firstErr = err
				}
			}
			return firstErr
		}
	}
}
