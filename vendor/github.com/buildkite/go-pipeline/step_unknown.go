package pipeline

import (
	"encoding/json"

	"github.com/buildkite/go-pipeline/ordered"
)

// Compile-time check that *UnknownStep satisfies necessary interfaces
var _ interface {
	Step
	ordered.Unmarshaler
} = (*UnknownStep)(nil)

// UnknownStep models any step we don't know how to represent in this version.
// When future step types are added, they should be parsed with more specific
// types. UnknownStep is present to allow older parsers to preserve newer
// pipelines.
type UnknownStep struct {
	Contents any
}

// MarshalJSON marshals the contents of the step.
func (u *UnknownStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.Contents)
}

// MarshalYAML returns the contents of the step.
func (u *UnknownStep) MarshalYAML() (any, error) {
	return u.Contents, nil
}

// UnmarshalOrdered unmarshals an unknown step.
func (u *UnknownStep) UnmarshalOrdered(src any) error {
	u.Contents = src
	return nil
}

func (u *UnknownStep) interpolate(tf stringTransformer) error {
	c, err := interpolateAny(tf, u.Contents)
	if err != nil {
		return err
	}
	u.Contents = c
	return nil
}

func (UnknownStep) stepTag() {}
