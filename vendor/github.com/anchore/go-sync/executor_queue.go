package sync

import (
	"context"
	"sync"
	"sync/atomic"
)

// queuedExecutor is an Executor that accepts units of work to execute asynchronously, queuing them rather than blocking
type queuedExecutor struct {
	canceled       atomic.Bool
	maxConcurrency int
	executing      atomic.Int32
	queue          List[*func()]
	wg             sync.WaitGroup
	childLock      sync.RWMutex
	childExecutor  *errGroupExecutor
}

var _ Executor = (*queuedExecutor)(nil)

func (e *queuedExecutor) Go(f func()) {
	if e.canceled.Load() {
		return
	}
	e.wg.Add(1)
	fn := func() {
		defer e.wg.Done()
		if e.canceled.Load() {
			return
		}
		f()
	}
	e.queue.Enqueue(&fn)
	if int(e.executing.Load()) < e.maxConcurrency {
		go e.exec()
	}
}

func (e *queuedExecutor) Wait(ctx context.Context) {
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

func (e *queuedExecutor) ChildExecutor() Executor {
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

func (e *queuedExecutor) exec() {
	e.executing.Add(1)
	defer e.executing.Add(-1)
	if int(e.executing.Load()) > e.maxConcurrency {
		return
	}
	for {
		f, ok := e.queue.Dequeue()
		if !ok {
			return
		}
		if f != nil {
			(*f)()
		}
	}
}
