package progress

import (
	"errors"
	"io"
)

const (
	DefaultStrategy AggregationStrategy = iota
	NormalizeStrategy
)

type AggregationStrategy int

type Aggregator struct {
	progs    []Progressable
	strategy AggregationStrategy
}

func NewAggregator(strategy AggregationStrategy, p ...Progressable) *Aggregator {
	if p == nil {
		p = make([]Progressable, 0)
	}
	return &Aggregator{
		progs:    p,
		strategy: strategy,
	}
}

func (a *Aggregator) Add(p ...Progressable) {
	a.progs = append(a.progs, p...)
}

func (a *Aggregator) Progress() Progress {
	result := Progress{}
	var completedProgs int
	var errs []error

	for _, p := range a.progs {

		switch a.strategy {
		case NormalizeStrategy:
			if p.Size() < 0 {
				result.current = 0
			} else {
				result.current += int64(100 / (float64(p.Size()) / float64(p.Current())))
			}
			result.size += 100
		default:
			result.current += p.Current()
			s := p.Size()
			if s > 0 {
				result.size += s
			}
		}

		// capture notable errors
		err := p.Error()
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, ErrCompleted) {
			errs = append(errs, err)
		}
		if IsCompleted(p) {
			completedProgs++
		}
	}

	if completedProgs == len(a.progs) {
		errs = append(errs, ErrCompleted)
	}

	result.err = errors.Join(errs...)
	return result
}

func (a Aggregator) Current() int64 {
	return a.Progress().Current()
}

func (a Aggregator) Size() int64 {
	return a.Progress().Size()
}

func (a Aggregator) Error() error {
	return a.Progress().Error()
}
