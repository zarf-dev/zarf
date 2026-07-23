package ld

import (
	"cmp"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
)

var (
	Debug      = os.Getenv("DEBUG_SPDX_TOOLS_GOLANG") == "true"
	emptyValue reflect.Value
	anyType    = reflect.TypeOf((*any)(nil)).Elem()
)

// baseType returns the base type if this is a pointer or interface
func baseType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Pointer:
		return baseType(t.Elem())
	default:
		return t
	}
}

func isFunc(o any) bool {
	return reflect.TypeOf(o).Kind() == reflect.Func
}

// isBlankNodeID indicates this is a blank node ID, e.g. _:CreationInfo-1
func isBlankNodeID(id string) bool {
	return strings.HasPrefix(id, "_:")
}

func typeName(t reflect.Type) string {
	switch {
	case isPointer(t):
		return "*" + typeName(t.Elem())
	case isSlice(t):
		return "[]" + typeName(t.Elem())
	case isMap(t):
		return "map[" + typeName(t.Key()) + "]" + typeName(t.Elem())
	case isPrimitive(t):
		return t.Name()
	}
	return path.Base(t.PkgPath()) + "." + t.Name()
}

func isSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice
}

func isMap(t reflect.Type) bool {
	return t.Kind() == reflect.Map
}

func isPointer(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer
}

func isPrimitive(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64,
		reflect.Bool:
		return true
	default:
		return false
	}
}

// FieldByType returns a field defined on type StructType matching the provided type, t
func FieldByType[StructType any](t reflect.Type) (reflect.StructField, bool) {
	var v StructType
	typ := reflect.TypeOf(v)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == typ {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// skipField indicates whether the field should be skipped
func skipField(field reflect.StructField) bool {
	return field.Type.Size() == 0
}

// merge returns a new map with all map values merged together
func merge[K comparable, V any](maps ...map[K]V) map[K]V {
	out := map[K]V{}
	for _, m := range maps {
		if m == nil {
			continue
		}
		for k, v := range m {
			if in1, ok := out[k]; ok {
				// existing value, recursively merge nested maps
				map1, ok1 := any(in1).(map[K]V)
				map2, ok2 := any(v).(map[K]V)
				if ok1 && ok2 {
					out[k] = any(merge[K, V](map1, map2)).(V)
					continue
				}
				panic("Context key already defined: " + stringify(k))
			}
			out[k] = v
		}
	}
	return out
}

func firstKey[K cmp.Ordered, V any](m map[K]V) K {
	return sortedKeys(m)[0]
}

func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func elemImplements[T any](v reflect.Value) bool {
	switch v.Type().Kind() {
	case reflect.Pointer, reflect.Interface:
		e := v.Elem()
		if !e.IsValid() {
			return false
		}
		if !e.CanInterface() {
			return false
		}
		_, ok := e.Interface().(T)
		return ok
	default:
		return false
	}
}

func trimCommonPrefixes(values []string) (prefix string, trimmed []string) {
	out := values[:]
	slices.Sort(out)
	last := len(out) - 1
	common := 0
	for ; common < len(out[0]); common++ {
		if out[0][common] != out[last][common] {
			break
		}
	}
	if common < 0 {
		return "", values
	}
	prefix = values[0][:common]
	for i := range out {
		out[i] = out[i][common:]
	}
	return prefix, out
}
