package pipeline

import (
	"fmt"

	"github.com/buildkite/go-pipeline/ordered"
)

// GroupStep models a group step.
//
// Standard caveats apply - see the package comment.
type GroupStep struct {
	// Fields common to various step types
	Key string `yaml:"key,omitempty" aliases:"id,identifier"`

	// Group must always exist in a group step (so that we know it is a group).
	// If it has a value, it is treated as equivalent to the label or name.
	Group *string `yaml:"group" aliases:"label,name"`

	Steps Steps `yaml:"steps"`

	// RemainingFields stores any other top-level mapping items so they at least
	// survive an unmarshal-marshal round-trip.
	RemainingFields map[string]any `yaml:",inline"`
}

// UnmarshalOrdered unmarshals a group step from an ordered map.
func (g *GroupStep) UnmarshalOrdered(src any) error {
	type wrappedGroup GroupStep
	if err := ordered.Unmarshal(src, (*wrappedGroup)(g)); err != nil {
		return fmt.Errorf("unmarshalling GroupStep: %w", err)
	}

	// Ensure Steps is never nil. Server side expects a sequence.
	if g.Steps == nil {
		g.Steps = Steps{}
	}
	return nil
}

func (g *GroupStep) interpolate(tf stringTransformer) error {
	if err := interpolateString(tf, &g.Key); err != nil {
		return err
	}
	if err := interpolateString(tf, g.Group); err != nil {
		return err
	}
	if err := g.Steps.interpolate(tf); err != nil {
		return err
	}
	return interpolateMap(tf, g.RemainingFields)
}

func (GroupStep) stepTag() {}

// MarshalJSON marshals the step to JSON. Special handling is needed because
// yaml.v3 has "inline" but encoding/json has no concept of it.
func (g *GroupStep) MarshalJSON() ([]byte, error) {
	return inlineFriendlyMarshalJSON(g)
}
