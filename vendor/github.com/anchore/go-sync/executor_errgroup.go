package sync

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// errGroupExecutor is an Executor that executes units of work, blocking when Go is called once the maxConcurrency
// is reached, only continuing subsequent Go calls when the nuber of executing functions drops below maxConcurrency
type errGroupExecutor struct {
	maxConcurrency int
	canceled       atomic.Bool
	g              errgroup.Group
	wg             sync.WaitGroup
	childLock      sync.RWMutex
	childExecutor  *errGroupExecutor
}

func newErrGroupExecutor(maxConcurrency int) *errGroupExecutor {
	e := &errGroupExecutor{
		maxConcurrency: maxConcurrency,
	}
	e.g.SetLimit(maxConcurrency)
	return e
}

func (e *errGroupExecutor) Go(f func()) {
	e.wg.Add(1)
	fn := func() error {
		defer e.wg.Done()
		if e.canceled.Load() {
			return nil
		}
		f()
		return nil
	}
	e.g.Go(fn)
}

func (e *errGroupExecutor) Wait(ctx context.Context) {
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

func (e *errGroupExecutor) ChildExecutor() Executor {
	e.childLock.RLock()
	child := e.childExecutor
	e.childLock.RUnlock()
	if child != nil {
		return child
	}
	e.childLock.Lock()
	defer e.childLock.Unlock()
	if e.childExecutor == nil {
		// create child executor with same bound
		e.childExecutor = newErrGroupExecutor(e.maxConcurrency)
	}
	return e.childExecutor
}

var _ Executor = (*errGroupExecutor)(nil)
