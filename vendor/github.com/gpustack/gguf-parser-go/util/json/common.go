package json

import (
	stdjson "encoding/json"
	"fmt"
)

type RawMessage = stdjson.RawMessage

var (
	MarshalIndent = stdjson.MarshalIndent
	Indent        = stdjson.Indent
	NewEncoder    = stdjson.NewEncoder
	Valid         = stdjson.Valid
)

// MustMarshal is similar to Marshal,
// but panics if found error.
func MustMarshal(v any) []byte {
	bs, err := Marshal(v)
	if err != nil {
		panic(fmt.Errorf("error marshaling json: %w", err))
	}

	return bs
}

// MustUnmarshal is similar to Unmarshal,
// but panics if found error.
func MustUnmarshal(data []byte, v any) {
	err := Unmarshal(data, v)
	if err != nil {
		panic(fmt.Errorf("error unmarshaling json: %w", err))
	}
}

// MustMarshalIndent is similar to MarshalIndent,
// but panics if found error.
func MustMarshalIndent(v any, prefix, indent string) []byte {
	bs, err := MarshalIndent(v, prefix, indent)
	if err != nil {
		panic(fmt.Errorf("error marshaling indent json: %w", err))
	}

	return bs
}

// ShouldMarshal is similar to Marshal,
// but never return error.
func ShouldMarshal(v any) []byte {
	bs, _ := Marshal(v)
	return bs
}

// ShouldUnmarshal is similar to Unmarshal,
// but never return error.
func ShouldUnmarshal(data []byte, v any) {
	_ = Unmarshal(data, v)
}

// ShouldMarshalIndent is similar to MarshalIndent,
// but never return error.
func ShouldMarshalIndent(v any, prefix, indent string) []byte {
	bs, _ := MarshalIndent(v, prefix, indent)
	return bs
}
