package v3_0

import "github.com/spdx/tools-golang/spdx/v3/internal/ld"

// collectAllElements collects all elements referenced by the element collection, except the collection itself
func collectAllElements(d AnyElementCollection) []AnyElement {
	all := make(map[AnyElement]struct{}, 1024)
	_ = ld.VisitObjectGraph(d, func(path []any, elem AnyElement) error {
		if _, ok := all[elem]; !ok {
			if elem == d {
				return nil
			}
			all[elem] = struct{}{}
		}
		return nil
	})
	return mapKeys(all)
}

func mapKeys[T comparable, V any](all map[T]V) []T {
	out := make([]T, len(all))
	i := 0
	nilCount := 0
	for e := range all {
		if isNil(e) {
			nilCount++
			continue
		}
		out[i] = e
		i++
	}
	if nilCount > 0 {
		return out[0 : len(out)-nilCount]
	}
	return out
}

func notNil[T any, ListType ~[]T](values ListType) ListType {
	var out ListType
	for i, v := range values {
		if isNil(v) {
			if out == nil {
				out = make(ListType, i, len(values)-1)
				copy(out, values[0:i])
			}
			continue
		}
		if out != nil {
			out = append(out, v)
		}
	}
	if out == nil {
		return values
	}
	return out
}
