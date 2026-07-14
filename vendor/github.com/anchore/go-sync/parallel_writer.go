package sync

import (
	"context"
	"errors"
	"io"
	"sync"
)

type parallelWriter struct {
	ctx      context.Context
	executor Executor
	writers  []io.Writer
}

// ParallelWriter returns a writer that writes the contents of each write call in parallel
// to all provided writers
func ParallelWriter(ctx context.Context, executorName string, writers ...io.Writer) io.Writer {
	executor := ContextExecutor(&ctx, executorName)
	return &parallelWriter{
		ctx:      ctx,
		executor: executor,
		writers:  writers,
	}
}

func (w *parallelWriter) Write(p []byte) (int, error) {
	errs := List[error]{}
	wg := sync.WaitGroup{}
	wg.Add(len(w.writers))
	for _, writer := range w.writers {
		w.executor.Go(func() {
			defer wg.Done()
			_, err := writer.Write(p)
			if err != nil {
				errs.Append(err)
			}
		})
	}
	wg.Wait()
	if errs.Len() > 0 {
		return 0, errors.Join(errs.Values()...)
	}
	return len(p), nil
}

var _ io.Writer = (*parallelWriter)(nil)
