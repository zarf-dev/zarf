package pipeline

import (
	"github.com/buildkite/go-pipeline/warning"
)

// In the buildkite pipeline yaml, some step types (broadly, wait steps, input steps and block steps) can be represented
// either by a scalar string (ie "wait") or by a mapping with keys and values and such
//
// This behaviour is difficult to cleanly model in go, which leads to the somewhat odd structure of the structs defined
// in this file - each type (WaitStep, InputStep) has a Scalar field which is set to the scalar value if the step was
// if, during pipeline parsing, the step was represented as a scalar, and is left empty if the step was represented as
// a mapping. In essence, if one of the fields of these structs is filled, the other should be empty.
//
// On the unmarshalling side, the differing types is handled by the steps parser - see the unmarshalStep() function in
// ./steps.go - it infers the underlying type of the thing it's unmarshalling, and if it's a string, calls NewScalarStep()
// (below) to create the appropriate struct. If it's a mapping, it creates the appropriate struct directly.
//
// On the marshalling side, the MarshalJSON() function on each struct handles the different cases. In general, if the
// Scalar field is set, it marshals that, otherwise it marshals the other fields.
//
// In reading this file, you may have noticed that I mentioned that there are three types of step that can be represented
// as a scalar, but there are only two structs defined here. This is because the third type, block steps, are represented
// in exactly the same way as input steps, so they can share the same struct. This is liable to change in the future,
// as conceptually they're different types, and it makes sense to have them as different types in go as well.
//
// Also also! The implementations for WaitStep and InputStep **almost**, but not quite identical. This is due to the behaviour
// of marshalling an empty struct into into JSON. For WaitStep, it makes sense that the empty &WaitStep{} struct marshals
// to "wait", but with InputStep, there's no way to tell whether it should be marshalled to "input" or "block", which
// have very different behaviour on the backend.

var validStepScalars = []string{"wait", "waiter", "block", "input", "manual"}

// NewScalarStep returns a Step that can be represented as a single string.
// Currently these are "wait", "block", "input", and some deprecated variations
// ("waiter", "manual"). If it is any other string, NewScalarStep returns an
// UnknownStep containing an error wrapping ErrUnknownStepType.
func NewScalarStep(s string) (Step, error) {
	switch s {
	case "wait", "waiter":
		return &WaitStep{Scalar: s}, nil

	case "block", "input", "manual":
		return &InputStep{Scalar: s}, nil

	default:
		return &UnknownStep{Contents: s}, warning.Newf("%w %q, want one of %v", ErrUnknownStepType, s, validStepScalars)
	}
}
