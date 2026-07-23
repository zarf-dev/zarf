package expr

import (
	"fmt"
	"reflect"
)

// InvalidOpError is returned when an attempt is made to perform an operation
// on a type that is not supported.
type InvalidOpError struct {
	Op string
	V  Value
}

func (e InvalidOpError) Error() string {
	return fmt.Sprintf("invalid operation: operator %s not defined for %v (%s)", e.Op, e.V.Value(), e.V.Type())
}

// ConversionError is returned when an invalid type conversion is attempted.
type ConversionError struct {
	From Type
	To   Type
}

func (e ConversionError) Error() string {
	return fmt.Sprintf("cannot convert %s to %s", e.From, e.To)
}

// ReferenceError is returned when it is not possible to take the address of a
// value.
type ReferenceError struct{}

func (e ReferenceError) Error() string {
	return "could not take reference of value"
}

// Value represents a value at runtime.
type Value interface {
	Type() Type
	Value() reflect.Value
	RawValue() interface{}

	Negate() Value
	Not() Value
	BitNot() Value
	Deref() Value
	Ref() Value
	Dot(ident string) Value

	LogicalOr(rhs Value) Value
	LogicalAnd(rhs Value) Value
	Equal(rhs Value) Value
	NotEqual(rhs Value) Value
	Lesser(rhs Value) Value
	LesserEqual(rhs Value) Value
	Greater(rhs Value) Value
	GreaterEqual(rhs Value) Value
	Add(rhs Value) Value
	Sub(rhs Value) Value
	Or(rhs Value) Value
	Xor(rhs Value) Value
	Mul(rhs Value) Value
	Div(rhs Value) Value
	Rem(rhs Value) Value
	Lsh(rhs Value) Value
	Rsh(rhs Value) Value
	And(rhs Value) Value
	AndNot(rhs Value) Value
	Index(rhs Value) Value
	Call(in []Value) Value
}

type val struct {
	v reflect.Value
	t Type
}

func promote(from Value, to Type) Value {
	if TypeEqual(from.Type(), to) {
		return from
	}

	ftype := from.Type()
	switch ftype.Kind() {
	case UntypedBool:
		switch to.Kind() {
		case Bool:
			return val{from.Value(), to}
		default:
			panic(ConversionError{From: ftype, To: to})
		}
	case UntypedFloat:
		switch to.Kind() {
		case Float32:
			return val{reflect.ValueOf(float32(reflect.ValueOf(from.Value).Float())), to}
		case Float64:
			return val{reflect.ValueOf(float64(reflect.ValueOf(from.Value).Float())), to}
		default:
			panic(ConversionError{From: ftype, To: to})
		}
	case UntypedInt:
		ival, uval, fval := int64(0), uint64(0), float64(0)
		switch n := from.RawValue().(type) {
		case int64:
			ival = int64(n)
			uval = uint64(n)
			fval = float64(n)
		case uint64:
			ival = int64(n)
			uval = uint64(n)
			fval = float64(n)
		}
		switch to.Kind() {
		case Int:
			return val{reflect.ValueOf(int(ival)), to}
		case Int8:
			return val{reflect.ValueOf(int8(ival)), to}
		case Int16:
			return val{reflect.ValueOf(int16(ival)), to}
		case Int32:
			return val{reflect.ValueOf(int32(ival)), to}
		case Int64:
			return val{reflect.ValueOf(int64(ival)), to}
		case Uint:
			return val{reflect.ValueOf(uint(uval)), to}
		case Uint8:
			return val{reflect.ValueOf(uint8(uval)), to}
		case Uint16:
			return val{reflect.ValueOf(uint16(uval)), to}
		case Uint32:
			return val{reflect.ValueOf(uint32(uval)), to}
		case Uint64:
			return val{reflect.ValueOf(uint64(uval)), to}
		case Uintptr:
			return val{reflect.ValueOf(uintptr(uval)), to}
		case Float32:
			return val{reflect.ValueOf(float32(fval)), to}
		case Float64:
			return val{reflect.ValueOf(float64(fval)), to}
		default:
			panic(ConversionError{From: ftype, To: to})
		}
	case UntypedNil:
		return val{reflect.Zero(toreflecttype(to)), to}
	default:
		panic(ConversionError{From: ftype, To: to})
	}
}

func coerce1(v Value) Value {
	if v.Type().Kind() == UntypedInt {
		switch n := v.RawValue().(type) {
		case uint64:
			return val{reflect.ValueOf(int(n)), NewPrimitiveType(Int)}
		case int64:
			return val{reflect.ValueOf(int(n)), NewPrimitiveType(Int)}
		}
	}
	return v
}

func coerce(lhs Value, rhs Value) (Value, Value) {
	if TypeEqual(lhs.Type(), rhs.Type()) {
		return coerce1(lhs), coerce1(rhs)
	} else if assignable(lhs.Type(), rhs.Type()) {
		return promote(lhs, rhs.Type()), rhs
	} else if assignable(rhs.Type(), lhs.Type()) {
		return lhs, promote(rhs, lhs.Type())
	}
	panic(ConversionError{From: lhs.Type(), To: rhs.Type()})
}

func (v val) Type() Type {
	return v.t
}

func (v val) Value() reflect.Value {
	return v.v
}

func (v val) RawValue() interface{} {
	return v.v.Interface()
}

func (v val) Negate() Value {
	switch n := v.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(-n), v.t}
	case int8:
		return val{reflect.ValueOf(-n), v.t}
	case int16:
		return val{reflect.ValueOf(-n), v.t}
	case int32:
		return val{reflect.ValueOf(-n), v.t}
	case int64:
		return val{reflect.ValueOf(-n), v.t}
	case uint:
		return val{reflect.ValueOf(-n), v.t}
	case uint8:
		return val{reflect.ValueOf(-n), v.t}
	case uint16:
		return val{reflect.ValueOf(-n), v.t}
	case uint32:
		return val{reflect.ValueOf(-n), v.t}
	case uint64:
		return val{reflect.ValueOf(-n), v.t}
	case uintptr:
		return val{reflect.ValueOf(-n), v.t}
	case float32:
		return val{reflect.ValueOf(-n), v.t}
	case float64:
		return val{reflect.ValueOf(-n), v.t}
	default:
		panic(InvalidOpError{Op: "-", V: v})
	}
}

func (v val) Not() Value {
	switch n := v.RawValue().(type) {
	case bool:
		return val{reflect.ValueOf(!n), v.t}
	default:
		panic(InvalidOpError{Op: "!", V: v})
	}
}

func (v val) BitNot() Value {
	switch n := v.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(^n), v.t}
	case int8:
		return val{reflect.ValueOf(^n), v.t}
	case int16:
		return val{reflect.ValueOf(^n), v.t}
	case int32:
		return val{reflect.ValueOf(^n), v.t}
	case int64:
		return val{reflect.ValueOf(^n), v.t}
	case uint:
		return val{reflect.ValueOf(^n), v.t}
	case uint8:
		return val{reflect.ValueOf(^n), v.t}
	case uint16:
		return val{reflect.ValueOf(^n), v.t}
	case uint32:
		return val{reflect.ValueOf(^n), v.t}
	case uint64:
		return val{reflect.ValueOf(^n), v.t}
	case uintptr:
		return val{reflect.ValueOf(^n), v.t}
	default:
		panic(InvalidOpError{Op: "^", V: v})
	}
}

func (v val) Deref() Value {
	ptrtype, ok := v.t.(*PtrType)
	if !ok {
		panic(InvalidOpError{Op: "*", V: v})
	}
	return val{v.v.Elem(), ptrtype.Elem()}
}

func (v val) Ref() Value {
	if !v.v.CanAddr() {
		panic(ReferenceError{})
	}
	return val{v.v.Addr(), NewPtrType(v.t)}
}

func (v val) Dot(ident string) Value {
	switch v.t.(type) {
	case *PackageType:
		if sv := v.RawValue().(Package).Symbol(ident); sv != nil {
			return sv
		}
	case *StructType:
		if sv := v.v.FieldByName(ident); sv.IsValid() {
			return val{sv, TypeOf(sv.Interface())}
		}
	case *PtrType:
		return v.Deref().Dot(ident)
	}
	panic(InvalidOpError{Op: ".", V: v})
}

func (v val) LogicalOr(rhs Value) Value {
	lv, ok := v.RawValue().(bool)
	if !ok {
		panic(InvalidOpError{Op: "||", V: v})
	}

	rv, ok := rhs.RawValue().(bool)
	if !ok {
		panic(InvalidOpError{Op: "||", V: rhs})
	}

	return val{reflect.ValueOf(lv || rv), NewPrimitiveType(Bool)}
}

func (v val) LogicalAnd(rhs Value) Value {
	lv, ok := v.RawValue().(bool)
	if !ok {
		panic(InvalidOpError{Op: "&&", V: v})
	}

	rv, ok := rhs.RawValue().(bool)
	if !ok {
		panic(InvalidOpError{Op: "&&", V: rhs})
	}

	return val{reflect.ValueOf(lv && rv), NewPrimitiveType(Bool)}
}

func (v val) Equal(rhs Value) Value {
	l, r := coerce(v, rhs)
	return val{reflect.ValueOf(l.RawValue() == r.RawValue()), NewPrimitiveType(Bool)}
}

func (v val) NotEqual(rhs Value) Value {
	l, r := coerce(v, rhs)
	return val{reflect.ValueOf(l.RawValue() != r.RawValue()), NewPrimitiveType(Bool)}
}

func (v val) Lesser(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv < r.RawValue().(int)), NewPrimitiveType(Bool)}
	case int8:
		return val{reflect.ValueOf(lv < r.RawValue().(int8)), NewPrimitiveType(Bool)}
	case int16:
		return val{reflect.ValueOf(lv < r.RawValue().(int16)), NewPrimitiveType(Bool)}
	case int32:
		return val{reflect.ValueOf(lv < r.RawValue().(int32)), NewPrimitiveType(Bool)}
	case int64:
		return val{reflect.ValueOf(lv < r.RawValue().(int64)), NewPrimitiveType(Bool)}
	case uint:
		return val{reflect.ValueOf(lv < r.RawValue().(uint)), NewPrimitiveType(Bool)}
	case uint8:
		return val{reflect.ValueOf(lv < r.RawValue().(uint8)), NewPrimitiveType(Bool)}
	case uint16:
		return val{reflect.ValueOf(lv < r.RawValue().(uint16)), NewPrimitiveType(Bool)}
	case uint32:
		return val{reflect.ValueOf(lv < r.RawValue().(uint32)), NewPrimitiveType(Bool)}
	case uint64:
		return val{reflect.ValueOf(lv < r.RawValue().(uint64)), NewPrimitiveType(Bool)}
	case uintptr:
		return val{reflect.ValueOf(lv < r.RawValue().(uintptr)), NewPrimitiveType(Bool)}
	case float32:
		return val{reflect.ValueOf(lv < r.RawValue().(float32)), NewPrimitiveType(Bool)}
	case float64:
		return val{reflect.ValueOf(lv < r.RawValue().(float64)), NewPrimitiveType(Bool)}
	default:
		panic(InvalidOpError{Op: "<", V: l})
	}
}

func (v val) LesserEqual(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv <= r.RawValue().(int)), NewPrimitiveType(Bool)}
	case int8:
		return val{reflect.ValueOf(lv <= r.RawValue().(int8)), NewPrimitiveType(Bool)}
	case int16:
		return val{reflect.ValueOf(lv <= r.RawValue().(int16)), NewPrimitiveType(Bool)}
	case int32:
		return val{reflect.ValueOf(lv <= r.RawValue().(int32)), NewPrimitiveType(Bool)}
	case int64:
		return val{reflect.ValueOf(lv <= r.RawValue().(int64)), NewPrimitiveType(Bool)}
	case uint:
		return val{reflect.ValueOf(lv <= r.RawValue().(uint)), NewPrimitiveType(Bool)}
	case uint8:
		return val{reflect.ValueOf(lv <= r.RawValue().(uint8)), NewPrimitiveType(Bool)}
	case uint16:
		return val{reflect.ValueOf(lv <= r.RawValue().(uint16)), NewPrimitiveType(Bool)}
	case uint32:
		return val{reflect.ValueOf(lv <= r.RawValue().(uint32)), NewPrimitiveType(Bool)}
	case uint64:
		return val{reflect.ValueOf(lv <= r.RawValue().(uint64)), NewPrimitiveType(Bool)}
	case uintptr:
		return val{reflect.ValueOf(lv <= r.RawValue().(uintptr)), NewPrimitiveType(Bool)}
	case float32:
		return val{reflect.ValueOf(lv <= r.RawValue().(float32)), NewPrimitiveType(Bool)}
	case float64:
		return val{reflect.ValueOf(lv <= r.RawValue().(float64)), NewPrimitiveType(Bool)}
	default:
		panic(InvalidOpError{Op: "<=", V: l})
	}
}

func (v val) Greater(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv > r.RawValue().(int)), NewPrimitiveType(Bool)}
	case int8:
		return val{reflect.ValueOf(lv > r.RawValue().(int8)), NewPrimitiveType(Bool)}
	case int16:
		return val{reflect.ValueOf(lv > r.RawValue().(int16)), NewPrimitiveType(Bool)}
	case int32:
		return val{reflect.ValueOf(lv > r.RawValue().(int32)), NewPrimitiveType(Bool)}
	case int64:
		return val{reflect.ValueOf(lv > r.RawValue().(int64)), NewPrimitiveType(Bool)}
	case uint:
		return val{reflect.ValueOf(lv > r.RawValue().(uint)), NewPrimitiveType(Bool)}
	case uint8:
		return val{reflect.ValueOf(lv > r.RawValue().(uint8)), NewPrimitiveType(Bool)}
	case uint16:
		return val{reflect.ValueOf(lv > r.RawValue().(uint16)), NewPrimitiveType(Bool)}
	case uint32:
		return val{reflect.ValueOf(lv > r.RawValue().(uint32)), NewPrimitiveType(Bool)}
	case uint64:
		return val{reflect.ValueOf(lv > r.RawValue().(uint64)), NewPrimitiveType(Bool)}
	case uintptr:
		return val{reflect.ValueOf(lv > r.RawValue().(uintptr)), NewPrimitiveType(Bool)}
	case float32:
		return val{reflect.ValueOf(lv > r.RawValue().(float32)), NewPrimitiveType(Bool)}
	case float64:
		return val{reflect.ValueOf(lv > r.RawValue().(float64)), NewPrimitiveType(Bool)}
	default:
		panic(InvalidOpError{Op: ">", V: l})
	}
}

func (v val) GreaterEqual(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv >= r.RawValue().(int)), NewPrimitiveType(Bool)}
	case int8:
		return val{reflect.ValueOf(lv >= r.RawValue().(int8)), NewPrimitiveType(Bool)}
	case int16:
		return val{reflect.ValueOf(lv >= r.RawValue().(int16)), NewPrimitiveType(Bool)}
	case int32:
		return val{reflect.ValueOf(lv >= r.RawValue().(int32)), NewPrimitiveType(Bool)}
	case int64:
		return val{reflect.ValueOf(lv >= r.RawValue().(int64)), NewPrimitiveType(Bool)}
	case uint:
		return val{reflect.ValueOf(lv >= r.RawValue().(uint)), NewPrimitiveType(Bool)}
	case uint8:
		return val{reflect.ValueOf(lv >= r.RawValue().(uint8)), NewPrimitiveType(Bool)}
	case uint16:
		return val{reflect.ValueOf(lv >= r.RawValue().(uint16)), NewPrimitiveType(Bool)}
	case uint32:
		return val{reflect.ValueOf(lv >= r.RawValue().(uint32)), NewPrimitiveType(Bool)}
	case uint64:
		return val{reflect.ValueOf(lv >= r.RawValue().(uint64)), NewPrimitiveType(Bool)}
	case uintptr:
		return val{reflect.ValueOf(lv >= r.RawValue().(uintptr)), NewPrimitiveType(Bool)}
	case float32:
		return val{reflect.ValueOf(lv >= r.RawValue().(float32)), NewPrimitiveType(Bool)}
	case float64:
		return val{reflect.ValueOf(lv >= r.RawValue().(float64)), NewPrimitiveType(Bool)}
	default:
		panic(InvalidOpError{Op: ">=", V: l})
	}
}

func (v val) Add(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv + r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv + r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv + r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv + r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv + r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv + r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv + r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv + r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv + r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv + r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv + r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	case float32:
		return val{reflect.ValueOf(lv + r.RawValue().(float32)), NewPrimitiveType(Float32)}
	case float64:
		return val{reflect.ValueOf(lv + r.RawValue().(float64)), NewPrimitiveType(Float64)}
	case string:
		return val{reflect.ValueOf(lv + r.RawValue().(string)), NewPrimitiveType(String)}
	default:
		panic(InvalidOpError{Op: "+", V: l})
	}
}

func (v val) Sub(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv - r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv - r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv - r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv - r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv - r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv - r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv - r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv - r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv - r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv - r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv - r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	case float32:
		return val{reflect.ValueOf(lv - r.RawValue().(float32)), NewPrimitiveType(Float32)}
	case float64:
		return val{reflect.ValueOf(lv - r.RawValue().(float64)), NewPrimitiveType(Float64)}
	default:
		panic(InvalidOpError{Op: "-", V: l})
	}
}

func (v val) Or(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv | r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv | r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv | r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv | r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv | r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv | r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv | r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv | r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv | r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv | r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv | r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	default:
		panic(InvalidOpError{Op: "|", V: l})
	}
}

func (v val) Xor(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv ^ r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv ^ r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv ^ r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv ^ r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv ^ r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv ^ r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv ^ r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv ^ r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv ^ r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv ^ r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv ^ r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	default:
		panic(InvalidOpError{Op: "^", V: l})
	}
}

func (v val) Mul(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv * r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv * r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv * r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv * r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv * r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv * r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv * r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv * r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv * r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv * r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv * r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	case float32:
		return val{reflect.ValueOf(lv * r.RawValue().(float32)), NewPrimitiveType(Float32)}
	case float64:
		return val{reflect.ValueOf(lv * r.RawValue().(float64)), NewPrimitiveType(Float64)}
	default:
		panic(InvalidOpError{Op: "*", V: l})
	}
}

func (v val) Div(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv / r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv / r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv / r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv / r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv / r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv / r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv / r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv / r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv / r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv / r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv / r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	case float32:
		return val{reflect.ValueOf(lv / r.RawValue().(float32)), NewPrimitiveType(Float32)}
	case float64:
		return val{reflect.ValueOf(lv / r.RawValue().(float64)), NewPrimitiveType(Float64)}
	default:
		panic(InvalidOpError{Op: "/", V: l})
	}
}

func (v val) Rem(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv % r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv % r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv % r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv % r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv % r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv % r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv % r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv % r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv % r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv % r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv % r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	default:
		panic(InvalidOpError{Op: "%", V: l})
	}
}

func (v val) Lsh(rhs Value) Value {
	l := coerce1(v)
	shift := uint64(0)

	switch rv := rhs.RawValue().(type) {
	case uint:
		shift = uint64(rv)
	case uint8:
		shift = uint64(rv)
	case uint16:
		shift = uint64(rv)
	case uint32:
		shift = uint64(rv)
	case uint64:
		shift = uint64(rv)
	case uintptr:
		shift = uint64(rv)
	default:
		panic(InvalidOpError{Op: "<<", V: rhs})
	}

	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv << shift), NewPrimitiveType(Uintptr)}
	default:
		panic(InvalidOpError{Op: "<<", V: l})
	}
}

func (v val) Rsh(rhs Value) Value {
	l := coerce1(v)
	shift := uint64(0)

	switch rv := rhs.RawValue().(type) {
	case uint:
		shift = uint64(rv)
	case uint8:
		shift = uint64(rv)
	case uint16:
		shift = uint64(rv)
	case uint32:
		shift = uint64(rv)
	case uint64:
		shift = uint64(rv)
	case uintptr:
		shift = uint64(rv)
	default:
		panic(InvalidOpError{Op: ">>", V: rhs})
	}

	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv >> shift), NewPrimitiveType(Uintptr)}
	default:
		panic(InvalidOpError{Op: ">>", V: l})
	}
}

func (v val) And(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv & r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv & r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv & r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv & r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv & r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv & r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv & r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv & r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv & r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv & r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv & r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	default:
		panic(InvalidOpError{Op: "&", V: l})
	}
}

func (v val) AndNot(rhs Value) Value {
	l, r := coerce(v, rhs)
	switch lv := l.RawValue().(type) {
	case int:
		return val{reflect.ValueOf(lv &^ r.RawValue().(int)), NewPrimitiveType(Int)}
	case int8:
		return val{reflect.ValueOf(lv &^ r.RawValue().(int8)), NewPrimitiveType(Int8)}
	case int16:
		return val{reflect.ValueOf(lv &^ r.RawValue().(int16)), NewPrimitiveType(Int16)}
	case int32:
		return val{reflect.ValueOf(lv &^ r.RawValue().(int32)), NewPrimitiveType(Int32)}
	case int64:
		return val{reflect.ValueOf(lv &^ r.RawValue().(int64)), NewPrimitiveType(Int64)}
	case uint:
		return val{reflect.ValueOf(lv &^ r.RawValue().(uint)), NewPrimitiveType(Uint)}
	case uint8:
		return val{reflect.ValueOf(lv &^ r.RawValue().(uint8)), NewPrimitiveType(Uint8)}
	case uint16:
		return val{reflect.ValueOf(lv &^ r.RawValue().(uint16)), NewPrimitiveType(Uint16)}
	case uint32:
		return val{reflect.ValueOf(lv &^ r.RawValue().(uint32)), NewPrimitiveType(Uint32)}
	case uint64:
		return val{reflect.ValueOf(lv &^ r.RawValue().(uint64)), NewPrimitiveType(Uint64)}
	case uintptr:
		return val{reflect.ValueOf(lv &^ r.RawValue().(uintptr)), NewPrimitiveType(Uintptr)}
	default:
		panic(InvalidOpError{Op: "&^", V: l})
	}
}

func (v val) Index(rhs Value) Value {
	if v.t.Kind() == String {
		index := promote(rhs, NewPrimitiveType(Int)).RawValue().(int)
		return val{reflect.ValueOf(v.RawValue().(string)[index]), NewPrimitiveType(Uint8)}
	}
	switch t := v.t.(type) {
	case *ArrayType:
		return val{v.v.Index(promote(rhs, NewPrimitiveType(Int)).RawValue().(int)), t.Elem()}
	case *SliceType:
		return val{v.v.Index(promote(rhs, NewPrimitiveType(Int)).RawValue().(int)), t.Elem()}
	case *MapType:
		return val{v.v.MapIndex(promote(rhs, t.Key()).Value()), t.Value()}
	default:
		panic(InvalidOpError{Op: "[]", V: v})
	}
}

func (v val) Call(in []Value) Value {
	ft, ok := v.t.(*FuncType)
	if !ok {
		panic("Call invoked on non-function")
	}
	if len(in) != ft.NumIn() {
		panic("invalid number of arguments to function")
	}
	inconv := []reflect.Value{}
	for i, n := range in {
		inconv = append(inconv, promote(n, ft.In(i)).Value())
	}
	out := v.v.Call(inconv)
	if len(out) != 1 {
		panic("only functions returning 1 value are supported")
	}
	return val{out[0], ft.Out(0)}
}

// ValueOf returns a Value for the given runtime value.
func ValueOf(i interface{}) Value {
	return val{reflect.ValueOf(i), TypeOf(i)}
}

func literalboolval(v bool) Value {
	return val{reflect.ValueOf(v), NewLiteralType(UntypedBool)}
}

func literalstrval(v string) Value {
	return val{reflect.ValueOf(v), NewPrimitiveType(String)}
}

func literalintval(v int64) Value {
	return val{reflect.ValueOf(v), NewLiteralType(UntypedInt)}
}

func literaluintval(v uint64) Value {
	return val{reflect.ValueOf(v), NewLiteralType(UntypedInt)}
}

func literalfloatval(v float64) Value {
	return val{reflect.ValueOf(v), NewLiteralType(UntypedFloat)}
}

func literalnilval() Value {
	return val{reflect.ValueOf(interface{}(nil)), NewLiteralType(UntypedNil)}
}
