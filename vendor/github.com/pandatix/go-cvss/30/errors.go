package gocvss30

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidCVSSHeader  = errors.New("invalid CVSS v3.0 header")
	ErrTooShortVector     = errors.New("too short vector")
	ErrInvalidMetricValue = errors.New("invalid metric value")
	ErrOutOfBoundsScore   = errors.New("out of bounds score")
)

// ErrMissing is an error returned by ParseVector when the
// given vector have missing base score attributes.
type ErrMissing struct {
	Abv string
}

func (err ErrMissing) Error() string {
	return fmt.Sprintf("base metric %s is not defined", err.Abv)
}

var _ error = (*ErrMissing)(nil)

// ErrDefinedN is an error returned by ParseVector when the
// given vector has a metric abbreviation defined multiple times.
type ErrDefinedN struct {
	Abv string
}

func (err ErrDefinedN) Error() string {
	return fmt.Sprintf("given CVSS v3.0 vector has %s metric abbreviation defined multiple times", err.Abv)
}

var _ error = (*ErrDefinedN)(nil)

// ErrInvalidMetric is an error returned when a given
// metric does not exist.
type ErrInvalidMetric struct {
	Abv string
}

func (err ErrInvalidMetric) Error() string {
	return fmt.Sprintf("invalid metric abbreviation : %s", err.Abv)
}

var _ error = (*ErrInvalidMetric)(nil)
