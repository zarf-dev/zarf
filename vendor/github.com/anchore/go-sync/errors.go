package sync

import "fmt"

type PanicError struct {
	Value any
	Stack string
}

func (p PanicError) Error() string {
	return fmt.Sprintf("panic: %v at:\n%s", p.Value, p.Stack)
}

func (p PanicError) Unwrap() error {
	if e, ok := p.Value.(error); ok {
		return e
	}
	return nil
}

var _ error = (*PanicError)(nil)
