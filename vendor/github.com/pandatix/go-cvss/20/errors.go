package gocvss20

import (
	"errors"
	"fmt"
)

var (
	ErrTooShortVector     = errors.New("too short vector")
	ErrInvalidMetricOrder = errors.New("invalid metric order")
	ErrInvalidMetricValue = errors.New("invalid metric value")
)

// ErrInvalidMetric is an error returned when a given
// metric does not exist.
type ErrInvalidMetric struct {
	Abv string
}

func (err ErrInvalidMetric) Error() string {
	return fmt.Sprintf("invalid metric abbreviation : %s", err.Abv)
}

var _ error = (*ErrInvalidMetric)(nil)
