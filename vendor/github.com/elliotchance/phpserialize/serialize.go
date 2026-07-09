package phpserialize

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// MarshalOptions must be provided when invoking Marshal(). Use
// DefaultMarshalOptions() for sensible defaults.
type MarshalOptions struct {
	// If this is true, then all struct names will be stripped from objects
	// and "stdClass" will be used instead. The default value is false.
	OnlyStdClass bool
}

// DefaultMarshalOptions will create a new instance of MarshalOptions with
// sensible defaults. See MarshalOptions for a full description of options.
func DefaultMarshalOptions() *MarshalOptions {
	options := new(MarshalOptions)
	options.OnlyStdClass = false

	return options
}

// MarshalBool returns the bytes to represent a PHP serialized bool value. This
// would be the equivalent to running:
//
//     echo serialize(false);
//     // b:0;
//
// The same result would be returned by marshalling a boolean value:
//
//     Marshal(true)
func MarshalBool(value bool) []byte {
	if value {
		return []byte("b:1;")
	}

	return []byte("b:0;")
}

// MarshalInt returns the bytes to represent a PHP serialized integer value.
// This would be the equivalent to running:
//
//     echo serialize(123);
//     // i:123;
//
// The same result would be returned by marshalling an integer value:
//
//     Marshal(123)
func MarshalInt(value int64) []byte {
	return []byte("i:" + strconv.FormatInt(value, 10) + ";")
}

// MarshalUint is provided for compatibility with unsigned types in Go. It works
// the same way as MarshalInt.
func MarshalUint(value uint64) []byte {
	return []byte("i:" + strconv.FormatUint(value, 10) + ";")
}

// MarshalFloat returns the bytes to represent a PHP serialized floating-point
// value. This would be the equivalent to running:
//
//     echo serialize(1.23);
//     // d:1.23;
//
// The bitSize should represent the size of the float. This makes conversion to
// a string value more accurate, for example:
//
//     // float64 is implicit for literals
//     MarshalFloat(1.23, 64)
//
//     // If the original value was cast from a float32
//     f := float32(1.23)
//     MarshalFloat(float64(f), 32)
//
// The same result would be returned by marshalling a floating-point value:
//
//     Marshal(1.23)
func MarshalFloat(value float64, bitSize int) []byte {
	return []byte("d:" + strconv.FormatFloat(value, 'f', -1, bitSize) + ";")
}

// MarshalString returns the bytes to represent a PHP serialized string value.
// This would be the equivalent to running:
//
//     echo serialize('Hello world');
//     // s:11:"Hello world";
//
// The same result would be returned by marshalling a string value:
//
//     Marshal('Hello world')
//
// One important distinction is that PHP stores binary data in strings. See
// MarshalBytes for more information.
func MarshalString(value string) []byte {
	// As far as I can tell only the single-quote is escaped. Not even the
	// backslash itself is escaped. Weird. See escapeTests for more information.
	value = strings.Replace(value, "'", "\\'", -1)

	return []byte(fmt.Sprintf("s:%d:\"%s\";", len(value), value))
}

// MarshalBytes returns the bytes to represent a PHP serialized string value
// that contains binary data. This is because PHP does not have a distinct type
// for binary data.
//
// This can cause some confusion when decoding the value as it will want to
// unmarshal as a string type. The Unmarshal() function will be sensitive to
// this condition and allow either a string or []byte when unserializing a PHP
// string.
func MarshalBytes(value []byte) []byte {
	var buffer bytes.Buffer
	for _, c := range value {
		buffer.WriteString(fmt.Sprintf("\\x%02x", c))
	}

	return []byte(fmt.Sprintf("s:%d:\"%s\";", len(value), buffer.String()))
}

// MarshalNil returns the bytes to represent a PHP serialized null value.
// This would be the equivalent to running:
//
//     echo serialize(null);
//     // N;
//
// Unlike the other specific Marshal functions it does not take an argument
// because the output is a constant value.
func MarshalNil() []byte {
	return []byte("N;")
}

// MarshalStruct returns the bytes that represent a PHP encoded class from a
// struct or pointer to a struct.
//
// Fields that are not exported (starting with a lowercase letter) will not be
// present in the output. All fields that appear in the output will have their
// first letter converted to lowercase. Any other uppercase letters in the field
// name are maintained. At the moment there is no way to change this behaviour,
// unlike other marshallers that use a tag on the field.
func MarshalStruct(input interface{}, options *MarshalOptions) ([]byte, error) {
	value := reflect.ValueOf(input)
	typeOfValue := value.Type()

	// Some of the fields in the struct may not be visible (unexported). We
	// need to make sure we count all the visible ones for the final result.
	visibleFieldCount := 0

	var buffer bytes.Buffer
	for i := 0; i < value.NumField(); i++ {
		f := value.Field(i)

		if !f.CanInterface() {
			// This is an unexported field, we cannot read it.
			continue
		}

		visibleFieldCount++

		fieldName, fieldOptions := parseTag(typeOfValue.Field(i).Tag.Get("php"))

		if fieldOptions.Contains("omitnilptr") {
			if f.Kind() == reflect.Ptr && f.IsNil() {
				visibleFieldCount--
				continue
			}
		}

		if fieldName == "-" {
			visibleFieldCount--
			continue
		} else if fieldName == "" {
			fieldName = lowerCaseFirstLetter(typeOfValue.Field(i).Name)
		}
		buffer.Write(MarshalString(fieldName))

		m, err := Marshal(f.Interface(), options)
		if err != nil {
			return nil, err
		}

		buffer.Write(m)
	}

	className := reflect.ValueOf(input).Type().Name()
	if options.OnlyStdClass {
		className = "stdClass"
	}

	return []byte(fmt.Sprintf("O:%d:\"%s\":%d:{%s}", len(className),
		className, visibleFieldCount, buffer.String())), nil
}

// Marshal is the canonical way to perform the equivalent of serialize() in PHP.
// It can handle encoding scalar types, slices and maps.
func Marshal(input interface{}, options *MarshalOptions) ([]byte, error) {

	if options == nil {
		options = DefaultMarshalOptions()
	}

	// []byte is a special case because all strings (binary and otherwise)
	// are handled as strings in PHP.
	if bytesToEncode, ok := input.([]byte); ok {
		return MarshalBytes(bytesToEncode), nil
	}

	// Nil is another special case because it is typeless and must be
	// handled before trying to determine the type.
	if input == nil {
		return MarshalNil(), nil
	}

	// Otherwise we need to decide if it is a scalar value, map or slice.
	value := reflect.ValueOf(input)
	switch value.Kind() {
	case reflect.Bool:
		return MarshalBool(value.Bool()), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64:
		return MarshalInt(value.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return MarshalUint(value.Uint()), nil

	case reflect.Float32:
		return MarshalFloat(value.Float(), 32), nil

	case reflect.Float64:
		return MarshalFloat(value.Float(), 64), nil

	case reflect.String:
		return MarshalString(value.String()), nil

	case reflect.Slice:
		return marshalSlice(value.Interface(), options)

	case reflect.Map:
		return marshalMap(value.Interface(), options)

	case reflect.Struct:
		return MarshalStruct(input, options)

	case reflect.Ptr:
		if value.IsNil() {
			return MarshalNil(), nil
		}
		return Marshal(value.Elem().Interface(), options)

	default:
		return nil, fmt.Errorf("can not encode: %T", input)
	}
}

func marshalSlice(input interface{}, options *MarshalOptions) ([]byte, error) {
	s := reflect.ValueOf(input)

	var buffer bytes.Buffer
	for i := 0; i < s.Len(); i++ {
		m, err := Marshal(i, options)
		if err != nil {
			return nil, err
		}

		buffer.Write(m)

		m, err = Marshal(s.Index(i).Interface(), options)
		if err != nil {
			return nil, err
		}

		buffer.Write(m)
	}

	return []byte(fmt.Sprintf("a:%d:{%s}", s.Len(), buffer.String())), nil
}

func marshalMap(input interface{}, options *MarshalOptions) ([]byte, error) {
	s := reflect.ValueOf(input)

	// Go randomises maps. To be able to test this we need to make sure the
	// map keys always come out in the same order. So we sort them first.
	mapKeys := s.MapKeys()
	sort.Slice(mapKeys, func(i, j int) bool {
		return lessValue(mapKeys[i], mapKeys[j])
	})

	var buffer bytes.Buffer
	for _, mapKey := range mapKeys {
		m, err := Marshal(mapKey.Interface(), options)
		if err != nil {
			return nil, err
		}

		buffer.Write(m)

		m, err = Marshal(s.MapIndex(mapKey).Interface(), options)
		if err != nil {
			return nil, err
		}

		buffer.Write(m)
	}

	return []byte(fmt.Sprintf("a:%d:{%s}", s.Len(), buffer.String())), nil
}

func lowerCaseFirstLetter(s string) string {
	return strings.ToLower(s[0:1]) + s[1:]
}
