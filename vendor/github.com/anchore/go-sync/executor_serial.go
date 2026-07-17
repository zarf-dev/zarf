package sync

import "context"

// serialExecutor is an Executor that executes serially, without any goroutines
type serialExecutor struct{}

func (u serialExecutor) Go(fn func()) {
	fn()
}

func (u serialExecutor) Wait(_ context.Context) {
}

var _ Executor = (*serialExecutor)(nil)
