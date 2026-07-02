package ld

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
)

var (
	StopTraversing      = fmt.Errorf("stop-traversing-graph")
	defaultVisitorDepth = 32 // make some reasonable visit defaults to avoid _always_ reallocating
	defaultVisitorSize  = 1024
)

// VisitObjectGraph traverses the object graph, taking into account cycles, calling the visitor function for each
// applicable type along the traversal, including field properties, pointer and subsequent struct values, elements in
// slices and values of maps, as well as some context such as the path to the field within a struct.
// NOTE: visitFunc will be called with a mutable path, if you need to use it, make a copy or process in the visitor
func VisitObjectGraph[T any](graph any, visitFunc visitFunc[T]) error {
	v := reflect.ValueOf(graph)
	if !v.IsValid() {
		return fmt.Errorf("error: invalid reflect.Value: %v", graph)
	}
	if rv, ok := graph.(reflect.Value); ok {
		v = rv
	}
	path := make([]any, 1, defaultVisitorDepth)
	path[0] = baseType(v.Type())
	o := visitor[T]{
		path:      path,
		visited:   make(map[key]struct{}, defaultVisitorSize),
		visitFunc: visitFunc,
	}
	err := o.visit(v, false)
	if errors.Is(err, StopTraversing) {
		return nil
	}
	return err
}

type visitFunc[T any] func(path []any, value T) error

type visitor[T any] struct {
	path      []any
	visited   map[key]struct{}
	visitFunc visitFunc[T]
}

type key struct {
	typ reflect.Type
	ptr uintptr
}

func (o *visitor[T]) visit(v reflect.Value, calledWithPointer bool) error {
	if !v.IsValid() {
		return nil
	}

	if v.Kind() == reflect.Interface {
		return o.visit(v.Elem(), false)
	}

	k, ok := makeKey(v)
	if ok {
		if _, ok = o.visited[k]; ok {
			return nil
		}
		o.visited[k] = struct{}{}
	}

	if v.CanInterface() {
		if Debug {
			debugLog("visiting:", o.path, v)
		}
		val, ok := v.Interface().(T)
		if ok {
			if !calledWithPointer && !isNil(v) {
				err := o.visitFunc(o.path, val)
				if err != nil {
					return err
				}
				if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
					calledWithPointer = true
				}
			}
		}
	}

	t := v.Type()

	switch t.Kind() {
	case reflect.Pointer:
		return o.visit(v.Elem(), calledWithPointer)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if !f.Anonymous {
				o.path = append(o.path, f)
			}
			fv := v.Field(i)
			if fv.Kind() == reflect.Struct && v.CanAddr() { // go allows calling pointer methods on struct fields of structs
				fv = fv.Addr()
			}
			err := o.visit(fv, false)
			if !f.Anonymous {
				o.path = o.path[:len(o.path)-1]
			}
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		if iter == nil {
			return nil
		}
		for iter.Next() {
			o.path = append(o.path, iter.Key())
			iv := iter.Value()
			// maps _cannot_ treat structs as pointers, so these do not .Addr() the same way slices and structs do
			err := o.visit(iv, false)
			o.path = o.path[:len(o.path)-1]
			if err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			o.path = append(o.path, i)
			iv := v.Index(i)
			if iv.Kind() == reflect.Struct && iv.CanAddr() { // go allows calling pointer methods on slices of structs
				iv = iv.Addr()
			}
			err := o.visit(iv, false)
			o.path = o.path[:len(o.path)-1]
			if err != nil {
				return err
			}
		}
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
	default:
		// for debugging: panic(fmt.Errorf("unexpected type: %v %#v", typeName(t), v))
	}
	return nil
}

func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Chan, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Interface:
		return v.IsNil() || isNil(v.Elem())
	default:
	}
	return !v.IsValid()
}

func makeKey(v reflect.Value) (key, bool) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Chan, reflect.Map, reflect.Slice:
		if v.IsNil() {
			return key{}, false
		}
		return key{v.Type(), v.Pointer()}, true
	default:
		return key{}, false
	}
}

func debugLog(args ...any) {
	_, file, line, _ := runtime.Caller(1)
	_, _ = fmt.Fprint(os.Stderr, file)
	_, _ = fmt.Fprint(os.Stderr, "@")
	_, _ = fmt.Fprintf(os.Stderr, "%d", line)
	_, _ = fmt.Fprint(os.Stderr, " ")
	for _, arg := range args {
		if v, ok := arg.(reflect.Value); ok {
			if v.CanInterface() {
				arg = fmt.Sprintf("%v %v", typeName(v.Type()), v.Interface())
			}
		}
		_, _ = fmt.Fprintf(os.Stderr, "%v", arg)
		_, _ = fmt.Fprint(os.Stderr, " ")
	}
	_, _ = fmt.Fprintln(os.Stderr)
}
