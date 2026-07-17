package taskprogress

import (
	"errors"

	"github.com/wagoodman/go-progress"
)

type Title struct {
	Default string
	Running string
	Success string
	Failed  string
}

func (t Title) Title(p progress.Progress) string {
	isFailed := p.Complete() && (p.Error() != nil && !errors.Is(p.Error(), progress.ErrCompleted))
	isSuccessful := p.Complete() && (p.Error() == nil || errors.Is(p.Error(), progress.ErrCompleted))
	// isRunning := p.Current() > 0 && !p.Complete()
	isRunning := !p.Complete()

	switch {
	case isRunning:
		if t.Running != "" {
			return t.Running
		}
	case isFailed:
		if t.Failed != "" {
			return t.Failed
		}
	case isSuccessful:
		if t.Success != "" {
			return t.Success
		}
	}
	return t.Default
}
