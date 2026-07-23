package pipeline

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/oleiade/reflections"
)

// inlineFriendlyMarshalJSON marshals the given object to JSON, but with special handling given to fields tagged with ",inline".
// This is needed because yaml.v3 has "inline" but encoding/json has no concept of it.
func inlineFriendlyMarshalJSON(q any) ([]byte, error) {
	fieldNames, err := reflections.Fields(q)
	if err != nil {
		return nil, fmt.Errorf("could not get fields of %T: %w", q, err)
	}

	var inlineFields map[string]any // no need to pre-allocate, we directly set it if we find inline fields
	outlineFields := make(map[string]any, len(fieldNames))

	for _, fieldName := range fieldNames {
		tag, err := reflections.GetFieldTag(q, fieldName, "yaml")
		if err != nil {
			return nil, fmt.Errorf("could not get yaml tag of %T.%s: %w", q, fieldName, err)
		}

		switch tag {
		case "-":
			continue

		case ",inline":
			inlineFieldsValue, err := reflections.GetField(q, fieldName)
			if err != nil {
				return nil, fmt.Errorf("could not get inline fields value of %T.%s: %w", q, fieldName, err)
			}

			if inf, ok := inlineFieldsValue.(map[string]any); ok {
				inlineFields = inf
			} else {
				return nil, fmt.Errorf("inline fields value of %T.%s must be a map[string]any, was %T instead", q, fieldName, inlineFieldsValue)
			}

		default:
			fieldValue, err := reflections.GetField(q, fieldName)
			if err != nil {
				return nil, fmt.Errorf("could not get value of %T.%s: %w", q, fieldName, err)
			}

			tags := strings.Split(tag, ",")
			keyName := tags[0] // e.g. "foo,omitempty" -> "foo"
			if len(tags) > 1 && tags[1] == "omitempty" && isEmptyValue(fieldValue) {
				continue
			}

			outlineFields[keyName] = fieldValue
		}
	}

	allFields := make(map[string]any, len(outlineFields)+len(inlineFields))

	for k, v := range inlineFields {
		allFields[k] = v
	}

	// "outline" (non-inline) fields should take precedence over inline fields
	for k, v := range outlineFields {
		allFields[k] = v
	}

	return json.Marshal(allFields)
}

// stolen from encoding/json
func isEmptyValue(q any) bool {
	if q == nil { // not stolen from encoding/json, but oddly missing from it?
		return true
	}

	v := reflect.ValueOf(q)

	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}
