package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/buildkite/go-pipeline/ordered"
	"gopkg.in/yaml.v3"
)

var (
	_ interface {
		json.Marshaler
		ordered.Unmarshaler
		yaml.Marshaler
		selfInterpolater
	} = (*Matrix)(nil)

	_ interface {
		json.Marshaler
		selfInterpolater
	} = (*MatrixAdjustment)(nil)

	_ = []interface {
		json.Marshaler
		ordered.Unmarshaler
		yaml.Marshaler
	}{
		(*MatrixSetup)(nil),
		(*MatrixAdjustmentWith)(nil),
	}
)

var (
	errNilMatrix                   = errors.New("non-empty permutation but matrix is nil")
	errPermutationLengthMismatch   = errors.New("permutation has wrong length")
	errPermutationUnknownDimension = errors.New("permutation has unknown dimension")
	errAdjustmentLengthMismatch    = errors.New("adjustment has wrong length")
	errAdjustmentUnknownDimension  = errors.New("adjustment has unknown dimension")
	errPermutationSkipped          = errors.New("permutation is skipped by adjustment")
	errPermutationNoMatch          = errors.New("permutation is neither a valid matrix combination nor an adjustment")
)

// Matrix models the matrix specification for command steps.
type Matrix struct {
	Setup       MatrixSetup       `yaml:"setup"`
	Adjustments MatrixAdjustments `yaml:"adjustments,omitempty"`

	RemainingFields map[string]any `yaml:",inline"`
}

// IsEmpty reports whether the matrix is empty (is nil, or has no setup,
// no adjustments, and no other data within it).
func (m *Matrix) IsEmpty() bool {
	return m == nil || (len(m.Setup) == 0 && len(m.Adjustments) == 0 && len(m.RemainingFields) == 0)
}

// UnmarshalOrdererd unmarshals from either []any or *ordered.MapSA.
func (m *Matrix) UnmarshalOrdered(o any) error {
	switch src := o.(type) {
	case []any:
		// Single anonymous dimension matrix, no adjustments.
		//
		// matrix:
		//   - apple
		//   - 47
		s := make([]string, 0, len(src))
		if err := ordered.Unmarshal(src, &s); err != nil {
			return err
		}
		m.Setup = MatrixSetup{"": s}

	case *ordered.MapSA:
		// Single anonymous dimension, or multiple named dimensions, with or
		// without adjustments.
		// Unmarshal into this secret wrapper type to avoid infinite recursion.
		type wrappedMatrix Matrix
		if err := ordered.Unmarshal(o, (*wrappedMatrix)(m)); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported src type for Matrix: %T", o)
	}
	return nil
}

// Reports if the matrix is a single anonymous dimension matrix with no
// adjustments or any other fields. (It's a list of items.)
func (m *Matrix) isSimple() bool {
	return len(m.Setup) == 1 && len(m.Setup[""]) != 0 && len(m.Adjustments) == 0 && len(m.RemainingFields) == 0
}

// MarshalJSON is needed to use inlineFriendlyMarshalJSON, and reduces the
// representation to a single list if the matrix is simple.
func (m *Matrix) MarshalJSON() ([]byte, error) {
	if m.isSimple() {
		return json.Marshal(m.Setup[""])
	}
	return inlineFriendlyMarshalJSON(m)
}

// MarshalYAML is needed to reduce the representation to a single slice if
// the matrix is simple.
func (m *Matrix) MarshalYAML() (any, error) {
	if m.isSimple() {
		return m.Setup[""], nil
	}
	// Just in case the YAML marshaler tries to call MarshalYAML on the output,
	// wrap m in a type without a MarshalYAML method.
	type wrappedMatrix Matrix
	return (*wrappedMatrix)(m), nil
}

func (m *Matrix) interpolate(tf stringTransformer) error {
	if m == nil {
		return nil
	}
	if _, is := tf.(matrixInterpolator); is {
		// Don't interpolate matrixes into matrixes.
		return nil
	}
	if err := interpolateMap(tf, m.Setup); err != nil {
		return err
	}
	if err := interpolateSlice(tf, m.Adjustments); err != nil {
		return err
	}
	return interpolateMap(tf, m.RemainingFields)
}

// validatePermutation checks that the permutation is a valid selection of
// dimension values, including any non-skipped adjustments.
func (m *Matrix) validatePermutation(p MatrixPermutation) error {
	if m == nil {
		if len(p) > 0 {
			return errNilMatrix
		}
		// An empty permutation from a nil matrix...seems fine to me?
		return nil
	}
	if len(p) != len(m.Setup) {
		return fmt.Errorf("%w: %d != %d", errPermutationLengthMismatch, len(p), len(m.Setup))
	}

	// Check that the dimensions in the permutation are unique and defined in
	// the matrix setup.
	for dim := range p {
		// An empty but non-nil setup dimension is valid (all values may be
		// given by adjustment tuples).
		if m.Setup[dim] == nil {
			return fmt.Errorf("%w: %q", errPermutationUnknownDimension, dim)
		}
	}

	// Check that the permutation values are in the matrix setup (a basic
	// permutation). Whether they are or are not, we still check adjustments.
	valid := true
	for dim, val := range p {
		if !slices.Contains(m.Setup[dim], val) {
			// Not a basic permutation. It could still be an adjustment though.
			valid = false
			break
		}
	}

	// Check if the permutation matches any adjustment.
	for _, adj := range m.Adjustments {
		// Ensure adj.With has the same size and dimension names as m.Setup.
		// adj.With is a map so no need to check for repetition.
		// Because adjustments can introduce new dimension values, only the
		// names of dimensions are checked.
		if len(adj.With) != len(m.Setup) {
			return fmt.Errorf("%w: %d != %d", errAdjustmentLengthMismatch, len(adj.With), len(m.Setup))
		}
		for dim := range adj.With {
			// An empty but non-nil setup dimension is valid (all values may be
			// given by adjustment tuples).
			if m.Setup[dim] == nil {
				return fmt.Errorf("%w: %q", errAdjustmentUnknownDimension, dim)
			}
		}

		// Now we can test whether p == adj.With.
		match := true
		for dim, val := range p {
			if val != adj.With[dim] {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		if adj.ShouldSkip() {
			return errPermutationSkipped
		}
		// Not skipped, but is an adjustment, so it's valid.
		// If multiple adjustments have the same permutation, and any of them
		// have "skip: true", then that applies, so we can't bail early.
		valid = true
	}

	if !valid {
		return errPermutationNoMatch
	}
	return nil
}

// MatrixPermutation represents a possible permutation of a matrix.
type MatrixPermutation map[string]string

// MatrixSetup is the main setup of a matrix - one or more dimensions. The cross
// product of the dimensions in the setup produces the base combinations of
// matrix values.
type MatrixSetup map[string][]string

// MarshalJSON returns either a list (if the setup is a single anonymous
// dimension) or an object (if it contains one or more (named) dimensions).
func (ms MatrixSetup) MarshalJSON() ([]byte, error) {
	// Note that MarshalYAML (below) always returns nil error.
	o, _ := ms.MarshalYAML()
	return json.Marshal(o)
}

// MarshalYAML returns either a Scalars (if the setup is a single anonymous
// dimension) or a map (if it contains one or more (named) dimensions).
func (ms MatrixSetup) MarshalYAML() (any, error) {
	if len(ms) == 1 && len(ms[""]) > 0 {
		return ms[""], nil
	}
	return map[string][]string(ms), nil
}

// UnmarshalOrdered unmarshals from either []any or *ordered.MapSA.
func (ms *MatrixSetup) UnmarshalOrdered(o any) error {
	if *ms == nil {
		*ms = make(MatrixSetup)
	}
	switch src := o.(type) {
	case []any:
		// Single anonymous dimension, but we only get here if its under a setup
		// key. (Maybe the user wants adjustments for their single dimension.)
		//
		// matrix:
		//   setup:
		//     - apple
		//     - 47
		s := make([]string, 0, len(src))
		if err := ordered.Unmarshal(src, &s); err != nil {
			return err
		}
		(*ms)[""] = s

	case *ordered.MapSA:
		// One or more (named) dimensions.
		// Unmarshal into the underlying type to avoid infinite recursion.
		if err := ordered.Unmarshal(src, (*map[string][]string)(ms)); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported src type for MatrixSetup: %T", o)
	}
	return nil
}

// MatrixAdjustments is a set of adjustments.
type MatrixAdjustments []*MatrixAdjustment

// MatrixAdjustment models an adjustment - a combination of (possibly new)
// matrix values, and skip/soft fail configuration.
type MatrixAdjustment struct {
	With MatrixAdjustmentWith `yaml:"with"`
	Skip any                  `yaml:"skip,omitempty"`

	RemainingFields map[string]any `yaml:",inline"` // NB: soft_fail is in the remaining fields
}

func (ma *MatrixAdjustment) ShouldSkip() bool {
	switch s := ma.Skip.(type) {
	case bool:
		return s

	case nil:
		return false

	default:
		return true
	}
}

// MarshalJSON is needed to use inlineFriendlyMarshalJSON.
func (ma *MatrixAdjustment) MarshalJSON() ([]byte, error) {
	return inlineFriendlyMarshalJSON(ma)
}

func (ma *MatrixAdjustment) interpolate(tf stringTransformer) error {
	if ma == nil {
		return nil
	}
	if err := interpolateMap(tf, ma.With); err != nil {
		return err
	}
	return interpolateMap(tf, ma.RemainingFields)
}

// MatrixAdjustmentWith is either a map of dimension key -> dimension value,
// or a single value (for single anonymous dimension matrices).
type MatrixAdjustmentWith map[string]string

// MarshalJSON returns either a single scalar or an object.
func (maw MatrixAdjustmentWith) MarshalJSON() ([]byte, error) {
	// Note that MarshalYAML (below) always returns nil error.
	o, _ := maw.MarshalYAML()
	return json.Marshal(o)
}

// MarshalYAML returns either a single scalar or a map.
func (maw MatrixAdjustmentWith) MarshalYAML() (any, error) {
	if _, has := maw[""]; has && len(maw) == 1 {
		return maw[""], nil
	}
	return map[string]string(maw), nil
}

// UnmarshalOrdered unmarshals from either a scalar value (string, bool, or int)
// or *ordered.MapSA.
func (maw *MatrixAdjustmentWith) UnmarshalOrdered(o any) error {
	if *maw == nil {
		*maw = make(MatrixAdjustmentWith)
	}

	switch src := o.(type) {
	case bool, int, string:
		// A single scalar.
		// (This is how you can do adjustments on a single anonymous dimension.)
		//
		// matrix:
		//   setup:
		//     - apple
		//     - 47
		//   adjustments:
		//     - with: banana
		//       soft_fail: true
		(*maw)[""] = fmt.Sprint(src)

	case *ordered.MapSA:
		// A map of dimension key -> dimension value. (Tuple of dimension value
		// selections.)
		return src.Range(func(k string, v any) error {
			switch vt := v.(type) {
			case bool, int, string:
				(*maw)[k] = fmt.Sprint(vt)

			default:
				return fmt.Errorf("unsupported value type %T in key %q for MatrixAdjustmentsWith", v, k)
			}
			return nil
		})

	default:
		return fmt.Errorf("unsupported src type for MatrixAdjustmentsWith: %T", o)
	}
	return nil
}
