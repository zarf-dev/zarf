package ordered

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/buildkite/go-pipeline/warning"

	"gopkg.in/yaml.v3"
)

// Errors that can be returned by Unmarshal
// (typically wrapped - use errors.Is).
var (
	ErrIntoNonPointer       = errors.New("cannot unmarshal into non-pointer")
	ErrIntoNil              = errors.New("cannot unmarshal into nil")
	ErrNotSettable          = errors.New("target value not settable")
	ErrIncompatibleTypes    = errors.New("incompatible types")
	ErrUnsupportedSrc       = errors.New("cannot unmarshal from src")
	ErrMultipleInlineFields = errors.New(`multiple fields tagged with yaml:",inline"`)
)

// Unmarshaler is an interface that types can use to override the default
// unmarshaling behaviour.
type Unmarshaler interface {
	// UnmarshalOrdered should unmarshal src into the implementing value. src
	// will generally be one of *Map[string, any], []any, or a "scalar" built-in
	// type.
	// If UnmarshalOrdered returns a non-nil error that is not a warning, the
	// whole unmarshaling process may halt at that point and report that error
	// (wrapped).
	// Unlike other errors, returning a warning lets unmarshalling continue
	// so that all warnings can be printed together at the end.
	UnmarshalOrdered(src any) error
}

// Unmarshal recursively unmarshals src into dst. src and dst can be a variety
// of types under the hood, but some combinations don't work. Good luck!
//
//   - If dst is nil, then src must be nil.
//   - If src is yaml.Node or *yaml.Node, then DecodeYAML is called to translate
//     the node into another type.
//   - If dst is a pointer and src is nil, then the value dst points to is set
//     to zero.
//   - If dst is a pointer to a pointer, Unmarshal recursively calls Unmarshal
//     on the inner pointer, creating a new value of the type being pointed to
//     as needed.
//   - If dst implements Unmarshaler, Unmarshal returns
//     dst.UnmarshalOrdered(src).
//   - If dst is *any, Unmarshal copies src directly into *dst.
//
// Otherwise, it acts a lot like yaml.Unmarshal, except that the type S of src
// and type D of dst can be one of the following:
//
//   - S = *Map[string, any] (recursively containing values with types from this
//     list); D must be one of: a pointer to a struct with yaml tags,
//     or a map or a pointer to a map (either *Map or map) with string keys.
//     yaml tags includes ",inline". Inline fields must themselves be a type
//     that Unmarshal can unmarshal *Map[string, any] into - another struct or
//     Map or map with string keys.
//     Struct targets can also have `aliases` tags of the form
//     `aliases:"apple,banana,citron"`
//     If the field name or yaml tag key doesn't match, Unmarshal looks through
//     the aliases list to see if any are present, and uses the value for the
//     first.
//   - S = []any (also recursively containing values with types from this list),
//     which is recursively unmarshaled elementwise; D is *[]any or
//     *[]somethingElse.
//   - S ∊ {string, float64, int, bool}; D must be *S (value copied directly),
//     *[]S or *[]any (value appended), *string (value formatted through
//     fmt.Sprint) or *[]string (formatted value appended).
func Unmarshal(src, dst any) error {
	if dst == nil {
		// This is interface nil (not typed nil, which has to be tested after
		// figuring out the types).
		if src == nil {
			// Unmarshal nil into nil? Seems legit
			return nil
		}
		return ErrIntoNil
	}

	// Apply DecodeYAML to yaml.Node or *yaml.Node first.
	switch n := src.(type) {
	case yaml.Node:
		o, err := DecodeYAML(&n)
		if err != nil {
			return err
		}
		src = o

	case *yaml.Node:
		o, err := DecodeYAML(n)
		if err != nil {
			return err
		}
		src = o
	}

	if um, ok := dst.(Unmarshaler); ok {
		return um.UnmarshalOrdered(src)
	}

	// Handle typed nil pointers, pointers to nil, and pointers to pointers.
	// Note that vdst could still be a map.
	vdst := reflect.ValueOf(dst)

	// First, handle src == nil. dst must be a pointer to something or nil.
	if src == nil {
		if vdst.Kind() != reflect.Pointer {
			return fmt.Errorf("%w (%T)", ErrIntoNonPointer, dst)
		}
		if vdst.IsNil() {
			// Unmarshaling nil into nil... seems legit.
			return nil
		}
		// Zero out the value pointed to by dst.
		vdst.Elem().SetZero()
		return nil
	}

	// src is not nil. dst is usually a pointer - is it nil? pointer to pointer?
	if vdst.Kind() == reflect.Pointer {
		// Unmarshaling into typed nil value?
		if vdst.IsNil() {
			return ErrIntoNil
		}

		// Non-nil pointer to something. Another pointer?
		if edst := vdst.Elem(); edst.Kind() == reflect.Pointer {
			// The type of the value being double-pointed to.
			innerType := edst.Type().Elem()
			if edst.IsNil() {
				// Create a new value of the inner type.
				edst.Set(reflect.New(innerType))
			}

			// Handle double pointers by recursing on the inner layer.
			return Unmarshal(src, edst.Interface())
		}
	}

	if tdst, ok := dst.(*any); ok {
		*tdst = src
		return nil
	}

	switch tsrc := src.(type) {
	case *Map[string, any]:
		return tsrc.decodeInto(dst)

	case []any:
		switch tdst := dst.(type) {
		case *[]any:
			*tdst = append(*tdst, tsrc...)

		default:
			if vdst.Kind() != reflect.Pointer {
				return fmt.Errorf("%w (%T)", ErrIntoNonPointer, dst)
			}
			sdst := vdst.Elem() // The slice we append to, reflectively
			if sdst.Kind() != reflect.Slice {
				return fmt.Errorf("%w: cannot unmarshal []any into %T", ErrIncompatibleTypes, dst)
			}
			stype := sdst.Type()  // stype = []E = the type of the slice
			etype := stype.Elem() // etype = E = Type of the slice's elements
			if sdst.IsNil() {
				// src isn't nil, so the output slice shouldn't be either.
				// Use MakeSlice to preallocate the exact size required.
				sdst = reflect.MakeSlice(stype, 0, len(tsrc))
			}
			var warns []error
			for i, a := range tsrc {
				x := reflect.New(etype) // x := new(E) (type *E)
				err := Unmarshal(a, x.Interface())
				if w := warning.As(err); w != nil {
					warns = append(warns, w.Wrapf("while unmarshaling item at index %d of %d", i, len(tsrc)))
				} else if err != nil {
					return fmt.Errorf("unmarshaling item at index %d of %d: %w", i, len(tsrc), err)
				}
				sdst = reflect.Append(sdst, x.Elem())
			}
			vdst.Elem().Set(sdst)
			return warning.Wrap(warns...)
		}

	case string:
		return unmarshalScalar(tsrc, dst)

	case float64:
		return unmarshalScalar(tsrc, dst)

	case int:
		return unmarshalScalar(tsrc, dst)

	case bool:
		return unmarshalScalar(tsrc, dst)

	default:
		return fmt.Errorf("%w %T", ErrUnsupportedSrc, src)
	}

	return nil
}

func unmarshalScalar[S any](src S, dst any) error {
	switch tdst := dst.(type) {
	case *S:
		*tdst = src

	case *[]S:
		*tdst = append(*tdst, src)

	case *[]any:
		*tdst = append(*tdst, src)

	case *string:
		*tdst = fmt.Sprint(src)

	case *[]string:
		*tdst = append(*tdst, fmt.Sprint(src))

	default:
		return fmt.Errorf("%w: cannot unmarshal %T into %T", ErrIncompatibleTypes, src, dst)
	}
	return nil
}

// decodeInto loads the contents of the map into the target (pointer to struct).
// It behaves sort of like `yaml.Node.Decode`:
//
//   - If target is a map type with string keys, it unmarshals its contents
//     elementwise, with values passed through Unmarshal.
//   - If target is *struct{...}, it matches keys to exported fields either
//     by looking at `yaml` tags, or using lowercased field names.
//   - If a field has a yaml:",inline" tag, it copies any leftover values into
//     that field, which must have type map[string]any or any. (Structs are not
//     supported for inline.)
func (m *Map[K, V]) decodeInto(target any) error {
	tm, ok := any(m).(*Map[string, any])
	if !ok {
		return fmt.Errorf("%w: cannot unmarshal from %T, want K=string, V=any", ErrIncompatibleTypes, m)
	}
	// Note: m, and therefore tm, can be nil at this moment.

	// Work out the kind of target being used.
	// Dereference the target to find the inner value, if needed.
	targetValue := reflect.ValueOf(target)
	switch targetValue.Kind() {
	case reflect.Pointer:
		// Passed a pointer to something.
		if tm == nil {
			if targetValue.IsNil() {
				return nil // nothing to do
			}
			if !targetValue.CanSet() {
				return ErrNotSettable
			}
			targetValue.SetZero() // which is nil
			return nil
		}
		if targetValue.IsNil() {
			return ErrIntoNil
		}
		targetValue = targetValue.Elem()

	case reflect.Map:
		// Continue below.

	default:
		return fmt.Errorf("%w: cannot unmarshal %T into %T, want map or *struct{...}", ErrIncompatibleTypes, m, target)
	}

	switch targetValue.Kind() {
	case reflect.Map:
		// Process the map directly.
		mapType := targetValue.Type()
		// For simplicity, require the key type to be string.
		if keyType := mapType.Key(); keyType.Kind() != reflect.String {
			return fmt.Errorf("%w for map key: cannot unmarshal %T into %T", ErrIncompatibleTypes, m, target)
		}

		// If tm is nil, then set the target to nil.
		if tm == nil {
			if targetValue.IsNil() {
				// Nothing to do.
				return nil
			}
			if !targetValue.CanSet() {
				return ErrNotSettable
			}
			targetValue.SetZero() // which is nil
			return nil
		}
		// Otherwise, if target is a pointer to a nil map (with type), create a new map.
		if targetValue.IsNil() {
			if !targetValue.CanSet() {
				return ErrNotSettable
			}
			targetValue.Set(reflect.MakeMapWithSize(mapType, tm.Len()))
		}

		valueType := mapType.Elem()
		var warns []error
		if err := tm.Range(func(k string, v any) error {
			nv := reflect.New(valueType)
			err := Unmarshal(v, nv.Interface())
			if w := warning.As(err); w != nil {
				warns = append(warns, w.Wrapf("while unmarshaling value for key %q", k))
			} else if err != nil {
				return fmt.Errorf("unmarshaling value for key %q: %w", k, err)
			}

			targetValue.SetMapIndex(reflect.ValueOf(k), nv.Elem())
			return nil
		}); err != nil {
			return err
		}
		return warning.Wrap(warns...)

	case reflect.Struct:
		// The rest of the method is concerned with this.
	default:
		return fmt.Errorf("%w: cannot unmarshal %T into %T", ErrIncompatibleTypes, m, target)
	}

	// These are the (accessible by reflection) fields it has.
	// This includes non-exported fields.
	fields := reflect.VisibleFields(targetValue.Type())

	var inlineField reflect.StructField
	outlineKeys := make(map[string]struct{})

	var warns []error

	for _, field := range fields {
		// Skip non-exported fields. This is conventional *and* correct.
		if !field.IsExported() {
			continue
		}

		// No worries if the tag is not there - apply defaults.
		tag, _ := field.Tag.Lookup("yaml")

		switch tag {
		case "-":
			// Note: if a field is skipped with "-", yaml.v3 still puts it into
			// inline.
			continue

		case ",inline":
			if inlineField.Index != nil {
				return fmt.Errorf("%w %T", ErrMultipleInlineFields, target)
			}
			inlineField = field
			continue
		}

		// default:
		key, _, _ := strings.Cut(tag, ",")
		if key == "" {
			// yaml.v3 convention:
			// "Struct fields ... are unmarshalled using the field name
			// lowercased as the default key."
			key = strings.ToLower(field.Name)
		}

		// Is there a value for this key?
		value, has := tm.Get(key)
		if !has {
			// Look for aliases, and choose the first with a value.
			atag, _ := field.Tag.Lookup("aliases")
			for _, alias := range strings.Split(atag, ",") {
				value, has = tm.Get(alias)
				if has {
					key = alias
					break
				}
			}
		}
		if !has {
			// Couldn't find a value for the key or any aliases, so skip.
			continue
		}

		// key matched a field, so it isn't inline.
		outlineKeys[key] = struct{}{}

		// Now load value into the field recursively.
		// Get a pointer to the field. This works because target is a pointer.
		ptrToField := targetValue.FieldByIndex(field.Index).Addr()
		err := Unmarshal(value, ptrToField.Interface())
		if w := warning.As(err); w != nil {
			warns = append(warns, w.Wrapf("while unmarshaling the value for key %q into struct field %q", key, field.Name))
		} else if err != nil {
			return err
		}
	}

	if inlineField.Index == nil {
		return warning.Wrap(warns...)
	}
	// The rest is handling the ",inline" field.
	// We support any field that Unmarshal can unmarshal tm into.

	inlinePtr := targetValue.FieldByIndex(inlineField.Index).Addr()

	// Copy all values that weren't non-inline fields into a temporary map.
	// This is just to avoid mutating tm.
	temp := NewMap[string, any](tm.Len())
	tm.Range(func(k string, v any) error {
		if _, outline := outlineKeys[k]; outline {
			return nil
		}
		temp.Set(k, v)
		return nil
	})

	// If the inline map contains nothing, then don't bother setting it.
	if temp.Len() == 0 {
		return warning.Wrap(warns...)
	}

	err := Unmarshal(temp, inlinePtr.Interface())
	if w := warning.As(err); w != nil {
		warns = append(warns, w.Wrapf("while unmarshaling the remaining input into an inline field of type %T", inlinePtr.Interface()))
		return warning.Wrap(warns...)
	}
	return err
}

// Compile-time check that *Map[string,any] is an Unmarshaler
var _ Unmarshaler = (*MapSA)(nil)

// UnmarshalOrdered unmarshals a value into this map.
// K must be string, src must be *Map[string, any], and each value in src must
// be unmarshallable into *V.
func (m *Map[K, V]) UnmarshalOrdered(src any) error {
	if m == nil {
		return ErrIntoNil
	}

	tm, ok := any(m).(*Map[string, V])
	if !ok {
		return fmt.Errorf("%w: receiver type %T, want K = string", ErrIncompatibleTypes, m)
	}

	tsrc, ok := src.(*Map[string, any])
	if !ok {
		return fmt.Errorf("%w: src type %T, want *Map[string, any]", ErrIncompatibleTypes, src)
	}

	var warns []error
	if err := tsrc.Range(func(k string, v any) error {
		var dv V
		err := Unmarshal(v, &dv)
		if w := warning.As(err); w != nil {
			warns = append(warns, w.Wrapf("while unmarshaling the value for key %q", k))
		} else if err != nil {
			return fmt.Errorf("unmarshaling value for key %q: %w", k, err)
		}
		tm.Set(k, dv)
		return nil
	}); err != nil {
		return err
	}
	return warning.Wrap(warns...)
}
