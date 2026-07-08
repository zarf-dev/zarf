package phpserialize

import (
	"errors"
	"reflect"
	"strconv"
)

// The internal consume functions work as the parser/lexer when reading
// individual items off the serialized stream.

// consumeStringUntilByte will return a string that includes all characters
// after the given offset, but only up until (and not including) a found byte.
//
// This function will only work with a plain, non-encoded series of bytes. It
// should not be used to capture anything other that ASCII data that is
// terminated by a single byte.
func consumeStringUntilByte(data []byte, lookingFor byte, offset int) (s string, newOffset int) {
	newOffset = findByte(data, lookingFor, offset)
	if newOffset < 0 {
		return "", -1
	}

	s = string(data[offset:newOffset])
	return
}

func consumeInt(data []byte, offset int) (int64, int, error) {
	if !checkType(data, 'i', offset) {
		return 0, -1, errors.New("not an integer")
	}

	alphaNumber, newOffset := consumeStringUntilByte(data, ';', offset+2)
	i, err := strconv.Atoi(alphaNumber)
	if err != nil {
		return 0, -1, err
	}

	// The +1 is to skip over the final ';'
	return int64(i), newOffset + 1, nil
}

func consumeFloat(data []byte, offset int) (float64, int, error) {
	if !checkType(data, 'd', offset) {
		return 0, -1, errors.New("not a float")
	}

	alphaNumber, newOffset := consumeStringUntilByte(data, ';', offset+2)
	v, err := strconv.ParseFloat(alphaNumber, 64)
	if err != nil {
		return 0, -1, err
	}

	return v, newOffset + 1, nil
}

func consumeString(data []byte, offset int) (string, int, error) {
	if !checkType(data, 's', offset) {
		return "", -1, errors.New("not a string")
	}

	return consumeStringRealPart(data, offset+2)
}

// consumeIntPart will consume an integer followed by and including a colon.
// This is used in many places to describe the number of elements or an upcoming
// length.
func consumeIntPart(data []byte, offset int) (int, int, error) {
	rawValue, newOffset := consumeStringUntilByte(data, ':', offset)
	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return 0, -1, err
	}

	// The +1 is to skip over the ':'
	return value, newOffset + 1, nil
}

func consumeStringRealPart(data []byte, offset int) (string, int, error) {
	length, newOffset, err := consumeIntPart(data, offset)
	if err != nil {
		return "", -1, err
	}

	// Skip over the '"' at the start of the string. I'm not sure why they
	// decided to wrap the string in double quotes since it's totally
	// redundant.
	offset = newOffset + 1

	s := DecodePHPString(data[offset : length+offset])

	// The +2 is to skip over the final '";'
	return s, offset + length + 2, nil
}

func consumeNil(data []byte, offset int) (interface{}, int, error) {
	if !checkType(data, 'N', offset) {
		return nil, -1, errors.New("not null")
	}

	return nil, offset + 2, nil
}

func consumeBool(data []byte, offset int) (bool, int, error) {
	if !checkType(data, 'b', offset) {
		return false, -1, errors.New("not a boolean")
	}

	return data[offset+2] == '1', offset + 4, nil
}

func consumeObjectAsMap(data []byte, offset int) (
	map[interface{}]interface{}, int, error) {
	result := map[interface{}]interface{}{}

	// Read the class name. The class name follows the same format as a
	// string. We could just ignore the length and hope that no class name
	// ever had a non-ascii characters in it, but this is safer - and
	// probably easier.
	_, offset, err := consumeStringRealPart(data, offset+2)
	if err != nil {
		return nil, -1, err
	}

	// Read the number of elements in the object.
	length, offset, err := consumeIntPart(data, offset)
	if err != nil {
		return nil, -1, err
	}

	// Skip over the '{'
	offset++

	// Read the elements
	for i := 0; i < length; i++ {
		var key string
		var value interface{}

		// The key should always be a string. I am not completely sure
		// about this.
		key, offset, err = consumeString(data, offset)
		if err != nil {
			return nil, -1, err
		}

		// If the next item is an object we can't simply consume it,
		// rather we send the reflect.Value back through consumeObject
		// so the recursion can be handled correctly.
		if data[offset] == 'O' {
			var subMap interface{}

			subMap, offset, err = consumeObjectAsMap(data, offset)
			if err != nil {
				return nil, -1, err
			}

			result[key] = subMap
		} else {
			value, offset, err = consumeNext(data, offset)
			if err != nil {
				return nil, -1, err
			}

			result[key] = value
		}
	}

	// The +1 is for the final '}'
	return result, offset + 1, nil
}

func setField(structFieldValue reflect.Value, value interface{}) error {
	if !structFieldValue.IsValid() {
		return nil
	}

	val := reflect.ValueOf(value)
	if !val.IsValid() {
		// structFieldValue will be set to default.
		return nil
	}

	switch structFieldValue.Type().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		structFieldValue.SetInt(val.Int())

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		structFieldValue.SetUint(val.Uint())

	case reflect.Float32, reflect.Float64:
		structFieldValue.SetFloat(val.Float())

	case reflect.Struct:
		m := val.Interface().(map[interface{}]interface{})
		fillStruct(structFieldValue, m)

	case reflect.Slice:
		l := val.Len()
		arrayOfObjects := reflect.MakeSlice(structFieldValue.Type(), l, l)

		for i := 0; i < l; i++ {
			if m, ok := val.Index(i).Interface().(map[interface{}]interface{}); ok {
				obj := arrayOfObjects.Index(i)
				fillStruct(obj, m)
			} else {
				switch arrayOfObjects.Index(i).Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					arrayOfObjects.Index(i).SetInt(val.Index(i).Elem().Int())
				case reflect.Float32, reflect.Float64:
					arrayOfObjects.Index(i).SetFloat(val.Index(i).Elem().Float())
				default:
					arrayOfObjects.Index(i).Set(val.Index(i).Elem())
				}

			}
		}

		structFieldValue.Set(arrayOfObjects)
	case reflect.Ptr:
		// Instantiate structFieldValue.
		structFieldValue.Set(reflect.New(structFieldValue.Type().Elem()))
		return setField(structFieldValue.Elem(), value)
	default:
		structFieldValue.Set(val)
	}

	return nil
}

// https://stackoverflow.com/questions/26744873/converting-map-to-struct
func fillStruct(obj reflect.Value, m map[interface{}]interface{}) error {
	tt := obj.Type()
	for i := 0; i < obj.NumField(); i++ {
		field := obj.Field(i)
		if !field.CanSet() {
			continue
		}
		var key string
		if tag := tt.Field(i).Tag.Get("php"); tag == "-" {
			continue
		} else if tag != "" {
			key = tag
		} else {
			key = lowerCaseFirstLetter(tt.Field(i).Name)
		}
		if v, ok := m[key]; ok {
			setField(field, v)
		}
	}

	return nil
}

func consumeObject(data []byte, offset int, v reflect.Value) (int, error) {
	if !checkType(data, 'O', offset) {
		return -1, errors.New("not an object")
	}

	m, offset, err := consumeObjectAsMap(data, offset)
	if err != nil {
		return -1, err
	}

	return offset, fillStruct(v, m)
}

func consumeNext(data []byte, offset int) (interface{}, int, error) {
	if offset >= len(data) {
		return nil, -1, errors.New("corrupt")
	}

	switch data[offset] {
	case 'a':
		return consumeIndexedOrAssociativeArray(data, offset)
	case 'b':
		return consumeBool(data, offset)
	case 'd':
		return consumeFloat(data, offset)
	case 'i':
		return consumeInt(data, offset)
	case 's':
		return consumeString(data, offset)
	case 'N':
		return consumeNil(data, offset)
	case 'O':
		return consumeObjectAsMap(data, offset)
	}

	return nil, -1, errors.New("can not consume type: " +
		string(data[offset:]))
}

func consumeIndexedOrAssociativeArray(data []byte, offset int) (interface{}, int, error) {
	// Sometimes we don't know if the array is going to be indexed or
	// associative until we have already started to consume it.
	originalOffset := offset

	// Try to consume it as an indexed array first.
	arr, offset, err := consumeIndexedArray(data, originalOffset)
	if err == nil {
		return arr, offset, err
	}

	// Fallback to consuming an associative array
	return consumeAssociativeArray(data, originalOffset)
}

func consumeAssociativeArray(data []byte, offset int) (map[interface{}]interface{}, int, error) {
	if !checkType(data, 'a', offset) {
		return map[interface{}]interface{}{}, -1, errors.New("not an array")
	}

	// Skip over the "a:"
	offset += 2

	rawLength, offset := consumeStringUntilByte(data, ':', offset)
	length, err := strconv.Atoi(rawLength)
	if err != nil {
		return map[interface{}]interface{}{}, -1, err
	}

	// Skip over the ":{"
	offset += 2

	result := map[interface{}]interface{}{}
	for i := 0; i < length; i++ {
		var key interface{}

		key, offset, err = consumeNext(data, offset)
		if err != nil {
			return map[interface{}]interface{}{}, -1, err
		}

		result[key], offset, err = consumeNext(data, offset)
		if err != nil {
			return map[interface{}]interface{}{}, -1, err
		}
	}

	return result, offset + 1, nil
}

func consumeIndexedArray(data []byte, offset int) ([]interface{}, int, error) {
	if !checkType(data, 'a', offset) {
		return []interface{}{}, -1, errors.New("not an array")
	}

	rawLength, offset := consumeStringUntilByte(data, ':', offset+2)
	length, err := strconv.Atoi(rawLength)
	if err != nil {
		return []interface{}{}, -1, err
	}

	// Skip over the ":{"
	offset += 2

	result := make([]interface{}, length)
	for i := 0; i < length; i++ {
		// Even non-associative arrays (arrays that are zero-indexed)
		// still have their keys serialized. We need to read these
		// indexes to make sure we are actually decoding a slice and not
		// a map.
		var index int64
		index, offset, err = consumeInt(data, offset)
		if err != nil {
			return []interface{}{}, -1, err
		}

		if index != int64(i) {
			return []interface{}{}, -1,
				errors.New("cannot decode map as slice")
		}

		// Now we consume the value
		result[i], offset, err = consumeNext(data, offset)
		if err != nil {
			return []interface{}{}, -1, err
		}
	}

	// The +1 is for the final '}'
	return result, offset + 1, nil
}
