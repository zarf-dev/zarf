package converter

import (
	"fmt"
	"reflect"
	"strconv"
)

type conversion struct {
	errors []error
	chain  *funcChain
}

func (c *conversion) err(err error) {
	c.errors = append(c.errors, err)
}

func (c *conversion) errf(format string, args ...any) {
	c.err(fmt.Errorf(format, args...))
}

// Convert takes two objects, e.g. v2_1.Document and &v2_2.Document{} and attempts to map all the properties from one
// to the other. After the automatic mapping, if an explicit conversion function is provided, this will be called to
// perform any additional conversion logic necessary.
func (c *conversion) convert(fromValue reflect.Value, toValuePtr reflect.Value) {
	toTypePtr := toValuePtr.Type()

	if !isPtr(toTypePtr) {
		c.errf("TO value provided was not a pointer, unable to set value: %+v", toValuePtr)
		return
	}

	toValue := c.getValue(fromValue, toTypePtr)

	// don't set nil values
	if toValue == nilValue {
		return
	}

	// toValuePtr is the passed-in pointer, toValue is also the same type of pointer
	toValuePtr.Elem().Set(toValue.Elem())
}

func (c *conversion) getValue(fromValue reflect.Value, targetType reflect.Type) reflect.Value {
	var err error

	fromType := fromValue.Type()

	var toValue reflect.Value

	// handle incoming pointer Types
	if isPtr(fromType) {
		if fromValue.IsNil() {
			return nilValue
		}
		fromValue = fromValue.Elem()
		if !fromValue.IsValid() || fromValue.IsZero() {
			return nilValue
		}
		fromType = fromValue.Type()
	}

	baseTargetType := targetType
	if isPtr(targetType) {
		baseTargetType = targetType.Elem()
	}

	switch {
	case isInterface(baseTargetType):
		satisfyingType := c.findConvertableType(fromType, baseTargetType)
		if satisfyingType != nil {
			return c.getValue(fromValue, satisfyingType)
		}
	case isStruct(fromType) && isStruct(baseTargetType):
		// this always creates a pointer type
		toValue = reflect.New(baseTargetType)
		toValue = toValue.Elem()

		for i := 0; i < fromType.NumField(); i++ {
			fromField := fromType.Field(i)
			fromFieldValue := fromValue.Field(i)

			toField, exists := baseTargetType.FieldByName(fromField.Name)
			if !exists {
				continue
			}
			toFieldType := toField.Type

			toFieldValue := toValue.FieldByName(toField.Name)

			newValue := c.getValue(fromFieldValue, toFieldType)
			if newValue == nilValue {
				continue
			}

			toFieldValue.Set(newValue)
		}

		// check for custom convert functions from previous/next version struct

		value, done := c.callConversionFunc(fromValue, fromType, baseTargetType, err, toValue)
		if done {
			return value
		}
	case isSlice(fromType) && isSlice(baseTargetType):
		if fromValue.IsNil() {
			return nilValue
		}

		length := fromValue.Len()
		targetElementType := baseTargetType.Elem()
		toValue = reflect.MakeSlice(baseTargetType, length, length)
		for i := 0; i < length; i++ {
			v := c.getValue(fromValue.Index(i), targetElementType)
			if v.IsValid() {
				toValue.Index(i).Set(v)
			}
		}
	case isMap(fromType) && isMap(baseTargetType):
		if fromValue.IsNil() {
			return nilValue
		}

		keyType := baseTargetType.Key()
		elementType := baseTargetType.Elem()
		toValue = reflect.MakeMap(baseTargetType)
		for _, fromKey := range fromValue.MapKeys() {
			fromVal := fromValue.MapIndex(fromKey)
			k := c.getValue(fromKey, keyType)
			v := c.getValue(fromVal, elementType)
			if k == nilValue || v == nilValue {
				continue
			}
			if v == nilValue {
				continue
			}
			if k.IsValid() && v.IsValid() {
				toValue.SetMapIndex(k, v)
			}
		}
	default:
		toValue = fromValue
	}

	if !toValue.IsValid() {
		return nilValue
	}

	// handle non-pointer returns -- the reflect.New earlier always creates a pointer
	if !isPtr(baseTargetType) {
		toValue = fromPtr(toValue)
	}

	toValue = c.convertValueTypes(toValue, baseTargetType)

	// handle elements which are now pointers
	if isPtr(targetType) {
		toValue = toPtr(toValue)
	}

	return toValue
}

func (c *conversion) callConversionFunc(fromValue reflect.Value, fromType reflect.Type, baseTargetType reflect.Type, _ error, toValue reflect.Value) (reflect.Value, bool) {
	if c.chain.funcs[fromType] != nil && c.chain.funcs[fromType][baseTargetType] != nil {
		convertFunc := c.chain.funcs[fromType][baseTargetType]
		err := convertFunc(fromValue, toValue.Addr())
		if err != nil {
			c.errf("an error occurred calling %s.%s: %v", baseTargetType.Name(), convertFromName, err)
			return nilValue, true
		}
	}
	return reflect.Value{}, false
}

// convertValueTypes takes a value and a target type, and attempts to convert
// between the Types - e.g. string -> int. when this function is called the value
func (c *conversion) convertValueTypes(value reflect.Value, targetType reflect.Type) reflect.Value {
	typ := value.Type()
	switch {
	// if the Types are the same, just return the value
	case typ == targetType:
		return value
	case typ.Kind() == targetType.Kind() && typ.ConvertibleTo(targetType):
		return value.Convert(targetType)
	case value.IsZero() && isPrimitive(targetType):
		// do nothing, will return nilValue
	case isPrimitive(typ) && isPrimitive(targetType):
		// get a string representation of the value
		str := fmt.Sprintf("%v", value.Interface()) // TODO is there a better way to get a string representation?
		var err error
		var out any
		switch {
		case isString(targetType):
			out = str
		case isBool(targetType):
			out, err = strconv.ParseBool(str)
		case isInt(targetType):
			out, err = strconv.Atoi(str)
		case isUint(targetType):
			out, err = strconv.ParseUint(str, 10, 64)
		case isFloat(targetType):
			out, err = strconv.ParseFloat(str, 64)
		}

		if err != nil {
			c.err(err)
			return nilValue
		}

		v := reflect.ValueOf(out)

		v = v.Convert(targetType)

		return v
	case isSlice(typ) && isSlice(targetType):
		// this should already be handled in getValue
	case isSlice(typ):
		// this may be lossy
		if value.Len() > 0 {
			v := value.Index(0)
			return c.convertValueTypes(v, targetType)
		}
		return c.convertValueTypes(nilValue, targetType)
	case isSlice(targetType):
		elementType := targetType.Elem()
		v := c.convertValueTypes(value, elementType)
		if v == nilValue {
			return v
		}
		slice := reflect.MakeSlice(targetType, 1, 1)
		slice.Index(0).Set(v)
		return slice
	}

	c.errf("unable to convert from: %v to %v", value.Interface(), targetType.Name())
	return nilValue
}

func (c *conversion) findConvertableType(fromType reflect.Type, targetType reflect.Type) reflect.Type {
	converters := c.chain.funcs[fromType]
	if converters == nil {
		return nil
	}
	if v, ok := converters[targetType]; ok {
		if v == nil {
			return nil
		}
		return targetType
	}
	var found *reflect.Type
	for target := range converters {
		if target.AssignableTo(targetType) {
			if found == nil {
				found = &target
			} else {
				// found multiple
				found = nil
				break
			}
		}
	}

	if found == nil {
		// if we didn't find exactly 1, don't check again
		converters[targetType] = nil
		return nil
	}
	converters[targetType] = converters[*found]
	return *found
}

func isPtr(typ reflect.Type) bool {
	return typ.Kind() == reflect.Ptr
}

func isPrimitive(typ reflect.Type) bool {
	return isString(typ) || isBool(typ) || isInt(typ) || isUint(typ) || isFloat(typ)
}

func isString(typ reflect.Type) bool {
	return typ.Kind() == reflect.String
}

func isBool(typ reflect.Type) bool {
	return typ.Kind() == reflect.Bool
}

func isInt(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return true
	default:
		return false
	}
}

func isUint(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return true
	default:
		return false
	}
}

func isFloat(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Float32,
		reflect.Float64:
		return true
	default:
		return false
	}
}

func isStruct(typ reflect.Type) bool {
	return typ.Kind() == reflect.Struct
}

func isSlice(typ reflect.Type) bool {
	return typ.Kind() == reflect.Slice
}

func isMap(typ reflect.Type) bool {
	return typ.Kind() == reflect.Map
}

func isInterface(targetType reflect.Type) bool {
	return targetType.Kind() == reflect.Interface
}

func toPtr(val reflect.Value) reflect.Value {
	typ := val.Type()
	if !isPtr(typ) {
		// this creates a pointer type inherently
		ptrVal := reflect.New(typ)
		ptrVal.Elem().Set(val)
		val = ptrVal
	}
	return val
}

func fromPtr(val reflect.Value) reflect.Value {
	if isPtr(val.Type()) {
		val = val.Elem()
	}
	return val
}

// convertFromName constant to find the ConvertFrom method
const convertFromName = "ConvertFrom"

var (
	// nilValue is returned in a number of cases when a value should not be set
	nilValue = reflect.ValueOf(nil)
)
