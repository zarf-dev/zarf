package logger

import (
	"fmt"
	"time"
)

type Field interface {
	Key() string
	String() string
}

type Fields []Field

func (f *Fields) Add(fields ...Field) {
	*f = append(*f, fields...)
}

func (f *Fields) Get(key string) []Field {
	fields := []Field{}
	for _, field := range *f {
		if field.Key() == key {
			fields = append(fields, field)
		}
	}
	return fields
}

type GenericField struct {
	key    string
	value  any
	format string
}

func (f GenericField) Key() string {
	return f.key
}

func (f GenericField) String() string {
	return fmt.Sprintf(f.format, f.value)
}

func StringField(key, value string) Field {
	return GenericField{
		key:    key,
		value:  value,
		format: "%s",
	}
}

func IntField(key string, value int) Field {
	return GenericField{
		key:    key,
		value:  value,
		format: "%d",
	}
}

func DurationField(key string, value time.Duration) Field {
	return GenericField{
		key:    key,
		value:  value,
		format: "%v",
	}
}
