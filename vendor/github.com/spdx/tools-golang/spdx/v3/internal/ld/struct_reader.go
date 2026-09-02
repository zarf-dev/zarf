package ld

import (
	"fmt"
	"reflect"
	"sync"
)

type reader[T any] func(reflect.Value) (T, error)

var idReaders = map[reflect.Type]reader[string]{}
var idReaderLock = sync.RWMutex{}

func idReader(t reflect.Type) (reader[string], error) {
	var err error
	idReaderLock.RLock()
	r := idReaders[t]
	idReaderLock.RUnlock()
	if r != nil {
		return r, nil
	}
	idReaderLock.Lock()
	r = idReaders[t]
	if r != nil {
		idReaderLock.Unlock()
		return r, nil
	}
	switch t.Kind() {
	case reflect.String:
		r = func(value reflect.Value) (string, error) {
			return value.String(), nil
		}
	case reflect.Struct:
		r, err = structIdFunc(t)
	case reflect.Map:
		// only map[string] supported
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("unsupported map key type: %v", stringify(t.Key()))
		}
		r = func(value reflect.Value) (string, error) {
			v := value.MapIndex(reflect.ValueOf(JsonIdProp))
			if v.IsValid() && v.Type().Kind() == reflect.String {
				return v.String(), nil
			}
			return "", fmt.Errorf("unable to find @id")
		}
	default:
		err = fmt.Errorf("unable to create ID reader, unsupported type: %v", stringify(t))
	}
	idReaders[t] = r
	idReaderLock.Unlock()
	return r, err
}

func structIdFunc(t reflect.Type) (reader[string], error) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && hasIDField(f.Type) {
			getter, err := structIdFunc(f.Type)
			if err != nil {
				return nil, fmt.Errorf("field: %v :%w", f.Name, err)
			}
			return func(value reflect.Value) (string, error) {
				v := value.Field(i)
				return getter(v)
			}, nil
		}
		if f.Tag.Get(GoIriTagName) == JsonIdProp {
			if f.Type.Kind() != reflect.String {
				return nil, fmt.Errorf("invalid @id type for field: %v in %v", f.Name, stringify(t))
			}
			return func(value reflect.Value) (string, error) {
				return value.Field(i).String(), nil
			}, nil
		}
	}
	// this struct type does not have an id
	return nil, fmt.Errorf("unable to find ID field in %v", stringify(t))
}

func hasIDField(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Tag.Get(GoIriTagName) == JsonIdProp {
			return true
		}
		if f.Anonymous {
			if hasIDField(f.Type) {
				return true
			}
		}
	}
	return false
}

func getID(v reflect.Value) (string, error) {
	if !v.IsValid() {
		return "", fmt.Errorf("invalid value")
	}
	switch v.Type().Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Pointer:
		return getID(v.Elem())
	case reflect.Struct, reflect.Map:
		r, err := idReader(v.Type())
		if err != nil {
			return "", err
		}
		return r(v)
	default:
		return "", fmt.Errorf("unsupported type: %v", stringify(v.Type()))
	}
}

func GetID(v any) (string, error) {
	if v == nil {
		return "", fmt.Errorf("value is nil")
	}
	if v, ok := v.(reflect.Value); ok {
		return getID(v)
	}
	return getID(reflect.ValueOf(v))
}
