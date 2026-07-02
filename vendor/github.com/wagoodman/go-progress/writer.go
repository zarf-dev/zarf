package progress

import "sync/atomic"

// Writer will consume a throw away bytes its given (not a passthrough). This is intended to be used with io.MultiWriter
type Writer struct {
	current atomic.Int64
	size    atomic.Int64
	done    atomic.Bool
}

func NewSizedWriter(size int64) *Writer {
	val := &Writer{}
	val.size.Store(size)
	return val
}

func NewWriter() *Writer {
	return NewSizedWriter(-1)
}

func (w *Writer) SetComplete() {
	w.done.Store(true)
}

func (w *Writer) Write(p []byte) (int, error) {
	n := len(p)
	w.current.Add(int64(n))
	return n, nil
}

func (w *Writer) Current() int64 {
	return w.current.Load()
}

func (w *Writer) Size() int64 {
	return w.size.Load()
}

func (w *Writer) Error() error {
	if w.done.Load() {
		return ErrCompleted
	}
	return nil
}
