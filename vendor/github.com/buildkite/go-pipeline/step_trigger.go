package pipeline

import (
	"encoding/json"
)

// TriggerStep models a trigger step.
//
// Standard caveats apply - see the package comment.
type TriggerStep struct {
	Contents map[string]any `yaml:",inline"`
}

// MarshalJSON marshals the contents of the step.
func (t TriggerStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Contents)
}

func (s TriggerStep) interpolate(tf stringTransformer) error {
	return interpolateMap(tf, s.Contents)
}

func (TriggerStep) stepTag() {}
