package pipeline

import "encoding/json"

// See the comment in step_scalar.go.

// WaitStep models a wait step.
//
// Standard caveats apply - see the package comment.
type WaitStep struct {
	Scalar   string         `yaml:"-"`
	Contents map[string]any `yaml:",inline"`
}

// MarshalJSON marshals a wait step as "wait" if the step is empty, or as the
// s.Scalar if it is not empty, or as s.Contents.
func (s *WaitStep) MarshalJSON() ([]byte, error) {
	o, err := s.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.Marshal(o)
}

// MarshalYAML returns a wait step as "wait" if the step is empty, or as the
// s.Scalar if it is not empty, or as s.Contents.
func (s *WaitStep) MarshalYAML() (any, error) {
	if s.Scalar != "" {
		return s.Scalar, nil
	}
	if len(s.Contents) == 0 {
		return "wait", nil
	}
	return s.Contents, nil
}

func (s *WaitStep) interpolate(tf stringTransformer) error {
	return interpolateMap(tf, s.Contents)
}

func (*WaitStep) stepTag() {}
