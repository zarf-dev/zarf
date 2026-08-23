package pipeline

import (
	"encoding/json"
	"errors"
)

// See the comment in step_scalar.go.

// InputStep models a block or input step.
//
// Standard caveats apply - see the package comment.
type InputStep struct {
	Scalar   string         `yaml:"-"`
	Contents map[string]any `yaml:",inline"`
}

// MarshalJSON marshals s.Scalar if it's not empty, otherwise s.Contents if that
// is not empty. If both s.Scalar and s.Contents are empty, it reports an error.
func (s *InputStep) MarshalJSON() ([]byte, error) {
	o, err := s.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.Marshal(o)
}

// MarshalYAML returns s.Scalar if it's not empty, otherwise s.Contents if that
// is not empty. If both s.Scalar and s.Contents are empty, it reports an error.
func (s *InputStep) MarshalYAML() (any, error) {
	if s.Scalar != "" {
		return s.Scalar, nil
	}
	if len(s.Contents) == 0 {
		return nil, errors.New("empty input step")
	}
	return s.Contents, nil
}

func (s InputStep) interpolate(tf stringTransformer) error {
	return interpolateMap(tf, s.Contents)
}

func (*InputStep) stepTag() {}
