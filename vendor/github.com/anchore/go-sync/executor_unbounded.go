package sync

import (
	"context"
	"sync"
	"sync/atomic"
)

// unboundedExecutor executes all Go calls without any specific bound
type unboundedExecutor struct {
	canceled atomic.Bool
	wg       sync.WaitGroup
}

func (e *unboundedExecutor) Go(f func()) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		if e.canceled.Load() {
			return
		}
		f()
	}()
}

func (e *unboundedExecutor) Wait(ctx context.Context) {
	e.canceled.Store(ctx.Err() != nil)

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		e.canceled.Store(true)
	case <-done:
	}
}
