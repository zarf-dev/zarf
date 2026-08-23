package ordered

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

// DecodeJSON decodes JSON bytes into the same generic types that DecodeYAML
// produces: *Map[string, any] for objects, []any for arrays, and string,
// int, float64, bool, or nil for scalars.
//
// Unlike DecodeYAML, this uses encoding/json directly and therefore correctly
// handles all JSON-valid characters (including control characters that yaml.v3
// rejects).
func DecodeJSON(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return decodeJSONValue(dec)
}

func decodeJSONValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return decodeJSONObject(dec)
		case '[':
			return decodeJSONArray(dec)
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter %q", t)
		}

	case json.Number:
		// Try int first, then float, matching yaml.v3 behaviour.
		if i, err := t.Int64(); err == nil {
			return int(i), nil
		}
		f, err := t.Float64()
		if err != nil {
			return nil, fmt.Errorf("decoding JSON number %q: %w", t, err)
		}
		// If the float is a whole number that fits in an int, return int.
		if f == math.Trunc(f) && !math.IsInf(f, 0) && f >= math.MinInt && f <= math.MaxInt {
			return int(int64(f)), nil
		}
		return f, nil

	case string:
		return t, nil

	case bool:
		return t, nil

	case nil:
		return nil, nil

	default:
		return nil, fmt.Errorf("unexpected JSON token type %T", tok)
	}
}

func decodeJSONObject(dec *json.Decoder) (*Map[string, any], error) {
	m := NewMap[string, any](0)
	for dec.More() {
		// Read the key.
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key in JSON object, got %T", tok)
		}

		// Read the value.
		val, err := decodeJSONValue(dec)
		if err != nil {
			return nil, fmt.Errorf("decoding value for key %q: %w", key, err)
		}
		m.Set(key, val)
	}

	// Consume the closing '}'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return m, nil
}

func decodeJSONArray(dec *json.Decoder) ([]any, error) {
	var arr []any
	for dec.More() {
		val, err := decodeJSONValue(dec)
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)
	}

	// Consume the closing ']'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return arr, nil
}
