package ld

import "reflect"

// RefCount returns the reference count of the value in the container map[string]any
func RefCount(find any, container any) int {
	findV, ok := find.(reflect.Value)
	if !ok {
		findV = reflect.ValueOf(find)
	}
	if !findV.IsValid() {
		return 0
	}

	containerV, ok := container.(reflect.Value)
	if !ok {
		containerV = reflect.ValueOf(container)
	}

	return refCountR(findV, map[reflect.Value]struct{}{}, containerV)
}

// refCountR recursively searches for the value, find, in the value v
func refCountR(find reflect.Value, visited map[reflect.Value]struct{}, v reflect.Value) int {
	if !v.IsValid() {
		return 0
	}
	if _, ok := visited[v]; ok {
		return 0
	}
	visited[v] = struct{}{}
	switch v.Kind() {
	case reflect.Interface:
		return refCountR(find, visited, v.Elem())
	case reflect.Pointer:
		if v.IsNil() {
			return 0
		}
		count := refCountR(find, visited, v.Elem())
		if find.Equal(v) {
			return count + 1
		}
		return count
	case reflect.Struct:
		count := 0
		for i := 0; i < v.NumField(); i++ {
			count += refCountR(find, visited, v.Field(i))
		}
		return count
	case reflect.Slice:
		count := 0
		for i := 0; i < v.Len(); i++ {
			count += refCountR(find, visited, v.Index(i))
		}
		return count
	default:
		return 0
	}
}
