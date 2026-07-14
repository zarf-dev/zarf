package progress

import "sync/atomic"

var _ Stager = (*AtomicStage)(nil)

type AtomicStage struct {
	current atomic.Value
}

func (s *AtomicStage) getCurrent() string {
	return s.current.Load().(string)
}

func (s *AtomicStage) setCurrent(new string) {
	s.current.Store(new)
}

func (s *AtomicStage) Stage() string {
	return s.getCurrent()
}

func (s *AtomicStage) Set(new string) {
	s.setCurrent(new)
}

func NewAtomicStage(current string) *AtomicStage {
	result := AtomicStage{}
	result.current.Store(current)
	return &result
}
