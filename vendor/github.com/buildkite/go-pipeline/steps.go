package pipeline

import (
	"errors"
	"fmt"

	"github.com/buildkite/go-pipeline/ordered"
	"github.com/buildkite/go-pipeline/warning"
)

// Sentinel errors that can appear when falling back to UnknownStep.
var (
	ErrStepTypeInference = errors.New("cannot infer step type")
	ErrUnknownStepType   = errors.New("unknown step type")
)

// Compile-time check that *Steps is an ordered.Unmarshaler.
var _ ordered.Unmarshaler = (*Steps)(nil)

// Steps contains multiple steps. It is useful for unmarshaling step sequences,
// since it has custom logic for determining the correct step type.
type Steps []Step

// UnmarshalOrdered unmarshals a slice ([]any) into a slice of steps.
func (s *Steps) UnmarshalOrdered(o any) error {
	if o == nil {
		if *s == nil {
			// `steps: null` is normalised to an empty slice.
			*s = Steps{}
		}
		return nil
	}
	sl, ok := o.([]any)
	if !ok {
		return fmt.Errorf("unmarshaling steps: got %T, want a slice ([]any)", sl)
	}
	// Preallocate slice if not already allocated
	if *s == nil {
		*s = make(Steps, 0, len(sl))
	}

	var warns []error
	for i, st := range sl {
		step, err := unmarshalStep(st)
		if w := warning.As(err); w != nil {
			warns = append(warns, w.Wrapf("while unmarshaling step %d of %d", i+1, len(sl)))
		} else if err != nil {
			return err
		}
		*s = append(*s, step)
	}
	return warning.Wrap(warns...)
}

func (s Steps) interpolate(tf stringTransformer) error {
	return interpolateSlice(tf, s)
}

// unmarshalStep unmarshals into the right kind of Step.
func unmarshalStep(o any) (Step, error) {
	switch o := o.(type) {
	case string:
		return NewScalarStep(o)

	case *ordered.MapSA:
		return stepFromMap(o)

	default:
		return nil, fmt.Errorf("unmarshaling step: unsupported type %T", o)
	}
}

// stepFromMap parses a step (that was originally a YAML mapping).
func stepFromMap(o *ordered.MapSA) (Step, error) {
	sType, hasType := o.Get("type")

	var warns []error
	var step Step
	var err error

	if hasType {
		sTypeStr, ok := sType.(string)
		if !ok {
			return nil, fmt.Errorf("unmarshaling step: step's `type` key was %T (value %v), want string", sType, sType)
		}
		step, err = stepByType(sTypeStr)
	} else {
		step, err = stepByKeyInference(o)
	}

	if err != nil {
		step = new(UnknownStep)
		warns = append(warns, err)
	}

	// Decode the step (into the right step type).
	err = ordered.Unmarshal(o, step)
	if w := warning.As(err); w != nil {
		warns = append(warns, w)
	} else if err != nil {
		// Hmm, maybe we picked the wrong kind of step?
		// Downgrade this error to a warning.
		step = &UnknownStep{Contents: o}
		warns = append(warns, warning.Wrapf(err, "fell back using unknown type of step due to an unmarshaling error"))
	}
	return step, warning.Wrap(warns...)
}

// stepByType returns a new empty step with a type corresponding to the "type"
// field. Unrecognised type values result in an UnknownStep containing an
// error wrapping ErrUnknownStepType.
func stepByType(sType string) (Step, error) {
	switch sType {
	case "command", "script":
		return new(CommandStep), nil

	case "wait", "waiter":
		return &WaitStep{Contents: map[string]any{}}, nil

	case "block", "input", "manual":
		return &InputStep{Contents: map[string]any{}}, nil

	case "trigger":
		return new(TriggerStep), nil

	case "group": // as far as i know this doesn't happen, but it's here for completeness
		return new(GroupStep), nil

	default:
		return nil, fmt.Errorf("%w %q", ErrUnknownStepType, sType)
	}
}

// stepByKeyInference returns a new empty step with a type based on some heuristic rules

// (first rule wins):
//
// - command, commands, plugins -> CommandStep
// - wait, waiter -> WaitStep
// - block, input, manual -> InputStep
// - trigger: TriggerStep
// - group: GroupStep.
//
// Failure to infer a step type results in an UnknownStep containing an
// error wrapping ErrStepTypeInference.
func stepByKeyInference(o *ordered.MapSA) (Step, error) {
	switch {
	case o.Contains("command") || o.Contains("commands") || o.Contains("plugins"):
		// NB: Some "command" step are commandless containers that exist
		// just to run plugins!
		return new(CommandStep), nil

	case o.Contains("wait") || o.Contains("waiter"):
		return new(WaitStep), nil

	case o.Contains("block") || o.Contains("input") || o.Contains("manual"):
		return new(InputStep), nil

	case o.Contains("trigger"):
		return new(TriggerStep), nil

	case o.Contains("group"):
		return new(GroupStep), nil

	default:
		inferrableKeys := []string{
			"command", "commands", "plugins", "wait", "waiter", "block", "input", "manual", "trigger", "group",
		}
		return nil, fmt.Errorf("%w: need one of %v", ErrStepTypeInference, inferrableKeys)
	}
}
