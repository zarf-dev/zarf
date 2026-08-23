package phpserialize

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"strings"
)

// findByte will return the first position at or after offset of the specified
// byte. -1 is returned if the byte is not found.
func findByte(data []byte, lookingFor byte, offset int) int {
	for ; offset < len(data); offset++ {
		if data[offset] == lookingFor {
			return offset
		}
	}

	return -1
}

// DecodePHPString converts a string of ASCII bytes (like "Bj\xc3\xb6rk") back
// into a UTF8 string ("Björk", in that case).
func DecodePHPString(data []byte) string {
	var buffer bytes.Buffer
	for i := 0; i < len(data); i++ {
		if data[i] == '\\' {
			if i+1 <= len(data)-1 {
				switch data[i+1] {
				case 'x':
					b, _ := strconv.ParseInt(string(data[i+2:i+4]), 16, 32)
					buffer.WriteByte(byte(b))
					i += 3

				case 'n':
					buffer.WriteByte('\n')
					i++

				case '\'':
					buffer.WriteByte(data[i+1])
					i++

				default:
					// It's a bit annoying but a backlash itself is not escaped. So
					// if it was not followed by a known character we have to assume
					// this.
					buffer.WriteByte('\\')
				}
			} else {
				buffer.WriteByte('\\')
			}
		} else {
			buffer.WriteByte(data[i])
		}
	}

	return buffer.String()
}

func UnmarshalFloat(data []byte) (float64, error) {
	i, _, err := consumeFloat(data, 0)
	return i, err
}

func UnmarshalString(data []byte) (string, error) {
	i, _, err := consumeString(data, 0)
	return i, err
}

func UnmarshalBytes(data []byte) ([]byte, error) {
	v, err := UnmarshalString(data)

	return []byte(v), err
}

func UnmarshalInt(data []byte) (int64, error) {
	i, _, err := consumeInt(data, 0)
	return i, err
}

func UnmarshalUint(data []byte) (uint64, error) {
	v, err := UnmarshalInt(data)
	return uint64(v), err
}

func UnmarshalNil(data []byte) error {
	_, _, err := consumeNil(data, 0)
	return err
}

func UnmarshalBool(data []byte) (bool, error) {
	v, _, err := consumeBool(data, 0)
	return v, err
}

func checkType(data []byte, typeCharacter byte, offset int) bool {
	return len(data) > offset && data[offset] == typeCharacter
}

func UnmarshalIndexedArray(data []byte) ([]interface{}, error) {
	v, _, err := consumeIndexedArray(data, 0)

	return v, err
}

func UnmarshalAssociativeArray(data []byte) (map[interface{}]interface{}, error) {
	// We may be unmarshalling an object into a map.
	if checkType(data, 'O', 0) {
		result, _, err := consumeObjectAsMap(data, 0)

		return result, err
	}

	result, _, err := consumeAssociativeArray(data, 0)

	return result, err
}

func UnmarshalObject(data []byte, v reflect.Value) error {
	_, err := consumeObject(data, 0, v)
	return err
}

func Unmarshal(data []byte, v interface{}) error {
	value := reflect.ValueOf(v).Elem()

	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := UnmarshalInt(data)
		if err != nil {
			return err
		}

		value.SetInt(v)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := UnmarshalUint(data)
		if err != nil {
			return err
		}

		value.SetUint(v)

	case reflect.Float32, reflect.Float64:
		v, err := UnmarshalFloat(data)
		if err != nil {
			return err
		}

		value.SetFloat(v)

	case reflect.Bool:
		v, err := UnmarshalBool(data)
		if err != nil {
			return err
		}

		value.SetBool(v)

	case reflect.String:
		v, err := UnmarshalString(data)
		if err != nil {
			return err
		}

		value.SetString(v)

	case reflect.Slice:
		// uint8 is an alias for byte. This means we are trying to pull
		// a binary string out.
		if value.Type().Elem().Kind() == reflect.Uint8 {
			v, err := UnmarshalBytes(data)
			if err != nil {
				return err
			}

			value.SetBytes(v)
			return nil
		}

		// Otherwise this must be a slice (array)
		v, err := UnmarshalIndexedArray(data)
		if err != nil {
			return err
		}

		value.Set(reflect.ValueOf(v))
		return nil

	case reflect.Map:
		v, err := UnmarshalAssociativeArray(data)
		if err != nil {
			return err
		}

		value.Set(reflect.ValueOf(v))
		return nil

	case reflect.Struct:
		err := UnmarshalObject(data, value)
		if err != nil {
			return err
		}

		return nil

	default:
		return errors.New("can not unmarshal type: " + value.Kind().String())
	}

	return nil
}

func upperCaseFirstLetter(s string) string {
	return strings.ToUpper(s[0:1]) + s[1:]
}
