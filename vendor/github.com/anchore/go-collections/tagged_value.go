package collections

import (
	"fmt"
	"reflect"
	"slices"
)

// TaggedValue holds an arbitrary value with associated tags
type TaggedValue[T any] struct {
	Value T
	Tags  []string
}

// HasTag indicates the TaggedValue has a tag matching one or more of the provided arguments
func (t TaggedValue[T]) HasTag(tags ...string) bool {
	for _, tag := range tags {
		if slices.Contains(t.Tags, tag) {
			return true
		}
	}
	return false
}

// NewTaggedValue returns a tagged value, that can be added, for example, to a TaggedValueSet collection
func NewTaggedValue[T any](value T, tags ...string) TaggedValue[T] {
	return TaggedValue[T]{
		Value: value,
		Tags:  tags,
	}
}

// TaggedValueSet is a utility to handle a sorted set of tagged items including basic filtering
type TaggedValueSet[T comparable] []TaggedValue[T]

// Values returns a slice containing the values in the set
func (t TaggedValueSet[T]) Values() []T {
	out := make([]T, len(t))
	for i, v := range t {
		out[i] = v.Value
	}
	return out
}

// HasTag indicates this set contains one or more items matching any of the provided tags
func (t TaggedValueSet[T]) HasTag(tags ...string) bool {
	for _, tagged := range t {
		if tagged.HasTag(tags...) {
			return true
		}
	}
	return false
}

// HasValue indicates any of the provided values is present in this set
func (t TaggedValueSet[T]) HasValue(value ...T) bool {
	for _, tagged := range t {
		for _, v := range value {
			if isEqual(tagged.Value, v) {
				return true
			}
		}
	}
	return false
}

// Select returns a new set of values matching any of the provided tags, ordered by the provided tags
func (t TaggedValueSet[T]) Select(tags ...string) TaggedValueSet[T] {
	if len(tags) == 0 {
		return TaggedValueSet[T]{}
	}
	out := make(TaggedValueSet[T], 0, len(t))
	for _, tag := range tags {
		for _, existing := range t {
			if existing.HasTag(tag) {
				if out.HasValue(existing.Value) {
					continue
				}
				out = append(out, existing)
			}
		}
	}
	return out
}

// Remove returns a new set of values, excluding those with any of the provided tags
func (t TaggedValueSet[T]) Remove(tags ...string) TaggedValueSet[T] {
	if len(tags) == 0 {
		return t
	}
	out := make(TaggedValueSet[T], 0, len(t))
	for _, tagged := range t {
		if !tagged.HasTag(tags...) {
			out = append(out, tagged)
		}
	}
	return out
}

// Join returns a new set of values, combining this set and the provided values, omitting duplicates
func (t TaggedValueSet[T]) Join(taggedValues ...TaggedValue[T]) TaggedValueSet[T] {
	if len(taggedValues) == 0 {
		return t
	}
	out := make(TaggedValueSet[T], 0, len(t)+len(taggedValues))
	out = append(out, t...)
	for _, value := range taggedValues {
		if t.HasValue(value.Value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

// Tags returns the unique set of tags from all entries
func (t TaggedValueSet[T]) Tags() (tags []string) {
	for _, entry := range t {
		for _, tag := range entry.Tags {
			if !slices.Contains(tags, tag) {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// since T in TaggedValueSet[T] is comparable, we can use == but this can panic if T is an interface and specific values are
// used which implement the interface but are not comparable. to avoid panics, we check if both values are comparable.
// additionally, if we wanted to use function references directly e.g. tagged.New(&package.Func), this function implements
// a workaround to properly compare these types
func isEqual[T any](a, b T) bool {
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	if va.Type().Comparable() && vb.Type().Comparable() {
		return va.Equal(vb)
	}
	if va.Type().Kind() == reflect.Func {
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
	return reflect.DeepEqual(a, b)
}
