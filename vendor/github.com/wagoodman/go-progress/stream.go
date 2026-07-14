package progress

import (
	"context"
	"time"
)

func Stream(ctx context.Context, progressable Progressable, interval time.Duration) <-chan Progress {
	results := make(chan Progress)

	generator := NewGenerator(progressable, progressable)

	go func() {
		defer close(results)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				progress := generator.Progress()
				results <- progress
				if progress.Complete() {
					return
				}
			}
		}
	}()
	return results
}

func StreamMonitor(ctx context.Context, monitor Monitorable, interval time.Duration) <-chan int64 {
	results := make(chan int64)

	go func() {
		defer close(results)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				completed := IsErrCompleted(monitor.Error())
				results <- monitor.Current()
				if completed {
					return
				}
			}
		}
	}()
	return results
}

func StreamMonitors(ctx context.Context, monitors []Monitorable, interval time.Duration) <-chan []int64 {
	results := make(chan []int64)

	go func() {
		defer close(results)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				res := make([]int64, len(monitors))
				completedMonitors := 0
				for _, monitor := range monitors {
					if IsErrCompleted(monitor.Error()) {
						completedMonitors++
					}
					res = append(res, monitor.Current())
				}
				results <- res
				if completedMonitors == len(monitors) {
					return
				}
			}
		}
	}()
	return results
}
