package anyx

import (
	"encoding/json"
	"fmt"
	"strconv"

	"golang.org/x/exp/constraints"
)

// Number converts any type to the specified number type.
func Number[T constraints.Integer | constraints.Float](v any) T {
	switch vv := v.(type) {
	case int:
		return T(vv)
	case int8:
		return T(vv)
	case int16:
		return T(vv)
	case int32:
		return T(vv)
	case int64:
		return T(vv)
	case uint:
		return T(vv)
	case uint8:
		return T(vv)
	case uint16:
		return T(vv)
	case uint32:
		return T(vv)
	case uint64:
		return T(vv)
	case float32:
		return T(vv)
	case float64:
		return T(vv)
	case bool:
		if vv {
			return T(1)
		}
		return T(0)
	case string:
		x, err := strconv.ParseInt(vv, 10, 64)
		if err != nil {
			y, err := strconv.ParseFloat(vv, 64)
			if err != nil {
				return T(0)
			} else {
				return T(y)
			}
		}
		return T(x)
	case json.Number:
		x, err := vv.Int64()
		if err != nil {
			y, err := vv.Float64()
			if err != nil {
				return T(0)
			} else {
				return T(y)
			}
		}
		return T(x)
	default:
		return T(0)
	}
}

// Bool converts any type to a bool.
func Bool(v any) bool {
	switch vv := v.(type) {
	case bool:
		return vv
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
		return vv != 0
	case float32, float64:
		return vv != 0
	case string:
		return vv != "0"
	case fmt.Stringer:
		return vv.String() != "0"
	default:
		return false
	}
}

// String converts any type to a string.
func String(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case []byte:
		return string(vv)
	case int:
		return strconv.FormatInt(int64(vv), 10)
	case int8:
		return strconv.FormatInt(int64(vv), 10)
	case int16:
		return strconv.FormatInt(int64(vv), 10)
	case int32:
		return strconv.FormatInt(int64(vv), 10)
	case int64:
		return strconv.FormatInt(vv, 10)
	case uint:
		return strconv.FormatUint(uint64(vv), 10)
	case uint8:
		return strconv.FormatUint(uint64(vv), 10)
	case uint16:
		return strconv.FormatUint(uint64(vv), 10)
	case uint32:
		return strconv.FormatUint(uint64(vv), 10)
	case uint64:
		return strconv.FormatUint(vv, 10)
	case float32:
		return strconv.FormatFloat(float64(vv), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(vv, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(vv)
	case fmt.Stringer:
		return vv.String()
	case json.RawMessage:
		return string(vv)
	default:
		return fmt.Sprintf("%v", v)
	}
}
