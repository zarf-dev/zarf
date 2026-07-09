package progress

import (
	"sync"
	"sync/atomic"
)

type Manual struct {
	n        int64
	total    int64
	err      error
	errMutex sync.Mutex
}

func NewManual(size int64) *Manual {
	return &Manual{
		total: size,
	}
}

func (p *Manual) Current() int64 {
	return atomic.LoadInt64(&p.n)
}

func (p *Manual) Size() int64 {
	return atomic.LoadInt64(&p.total)
}

func (p *Manual) Error() error {
	p.errMutex.Lock()
	defer p.errMutex.Unlock()
	return p.err
}

func (p *Manual) SetError(err error) {
	p.errMutex.Lock()
	defer p.errMutex.Unlock()
	p.err = err
}

func (p *Manual) Progress() Progress {
	return Progress{
		current: p.Current(),
		size:    p.Size(),
		err:     p.Error(),
	}
}

func (p *Manual) Add(n int64) {
	atomic.AddInt64(&p.n, n)
}

func (p *Manual) Increment() {
	atomic.AddInt64(&p.n, 1)
}

func (p *Manual) Set(n int64) {
	atomic.StoreInt64(&p.n, n)
}

func (p *Manual) SetTotal(total int64) {
	atomic.StoreInt64(&p.total, total)
}

func (p *Manual) SetCompleted() {
	p.SetError(ErrCompleted)
	if p.Current() > 0 && p.Size() <= 0 {
		p.SetTotal(p.Current())
		return
	}
}
