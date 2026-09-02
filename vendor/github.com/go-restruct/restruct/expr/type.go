package expr

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	primType = map[reflect.Kind]Type{
		reflect.Bool:    NewPrimitiveType(Bool),
		reflect.Int:     NewPrimitiveType(Int),
		reflect.Int8:    NewPrimitiveType(Int8),
		reflect.Int16:   NewPrimitiveType(Int16),
		reflect.Int32:   NewPrimitiveType(Int32),
		reflect.Int64:   NewPrimitiveType(Int64),
		reflect.Uint:    NewPrimitiveType(Uint),
		reflect.Uint8:   NewPrimitiveType(Uint8),
		reflect.Uint16:  NewPrimitiveType(Uint16),
		reflect.Uint32:  NewPrimitiveType(Uint32),
		reflect.Uint64:  NewPrimitiveType(Uint64),
		reflect.Uintptr: NewPrimitiveType(Uintptr),
		reflect.Float32: NewPrimitiveType(Float32),
		reflect.Float64: NewPrimitiveType(Float64),
		reflect.String:  NewPrimitiveType(String),
	}

	primRType = map[Kind]reflect.Type{
		Bool:    reflect.TypeOf(bool(false)),
		Int:     reflect.TypeOf(int(0)),
		Int8:    reflect.TypeOf(int8(0)),
		Int16:   reflect.TypeOf(int16(0)),
		Int32:   reflect.TypeOf(int32(0)),
		Int64:   reflect.TypeOf(int64(0)),
		Uint:    reflect.TypeOf(uint(0)),
		Uint8:   reflect.TypeOf(uint8(0)),
		Uint16:  reflect.TypeOf(uint16(0)),
		Uint32:  reflect.TypeOf(uint32(0)),
		Uint64:  reflect.TypeOf(uint64(0)),
		Uintptr: reflect.TypeOf(uintptr(0)),
		Float32: reflect.TypeOf(float32(0)),
		Float64: reflect.TypeOf(float64(0)),
		String:  reflect.TypeOf(string("")),
	}

	// ErrInvalidKind occurs when you call an inappropriate method for a given kind.
	ErrInvalidKind = errors.New("invalid kind")

	// ErrNotRepresentable occurs when a type is encountered that is not supported by the language.
	ErrNotRepresentable = errors.New("type cannot be represented")

	// ErrUntypedNil occurs when an untyped nil is used inappropriately.
	ErrUntypedNil = errors.New("untyped nil value")
)

// NoSuchFieldError is returned when an unknown field is accessed.
type NoSuchFieldError struct {
	field string
}

func (err NoSuchFieldError) Error() string {
	return fmt.Sprintf("no such field: %s", err.field)
}

// Kind is the most basic type descriptor.
type Kind int

// Enumeration of valid kinds of types.
const (
	Invalid Kind = iota

	// Primitives
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	String

	// Untyped constants
	UntypedBool
	UntypedInt
	UntypedFloat
	UntypedNil

	// Composite types
	Array
	Slice
	Struct
	Map
	Ptr
	Func
	Pkg
)

// Type is the representation of an expr type.
type Type interface {
	Kind() Kind
	String() string
}

// PrimitiveType is the type of primitives.
type PrimitiveType struct {
	kind Kind
}

// NewPrimitiveType returns a new primitive type.
func NewPrimitiveType(k Kind) Type {
	if k < Bool || k > String {
		panic("not a primitive kind")
	}
	return &PrimitiveType{kind: k}
}

// String implements Type.
func (t PrimitiveType) String() string {
	switch t.kind {
	case Bool:
		return "bool"
	case Int:
		return "int"
	case Int8:
		return "int8"
	case Int16:
		return "int16"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	case Uint:
		return "uint"
	case Uint8:
		return "uint8"
	case Uint16:
		return "uint16"
	case Uint32:
		return "uint32"
	case Uint64:
		return "uint64"
	case Uintptr:
		return "uintptr"
	case Float32:
		return "float32"
	case Float64:
		return "float64"
	case String:
		return "string"
	default:
		return ""
	}
}

// Kind implements Type.
func (t PrimitiveType) Kind() Kind {
	return t.kind
}

// littype is the type of literals.
type littype struct {
	kind Kind
}

// NewLiteralType returns a new primitive type.
func NewLiteralType(k Kind) Type {
	if k < UntypedBool || k > UntypedNil {
		panic("not a primitive kind")
	}
	return &littype{kind: k}
}

// String implements Type.
func (t littype) String() string {
	switch t.kind {
	case UntypedBool:
		return "untyped bool constant"
	case UntypedInt:
		return "untyped int constant"
	case UntypedFloat:
		return "untyped float constant"
	case UntypedNil:
		return "untyped nil value"
	default:
		return ""
	}
}

func (t littype) Kind() Kind { return t.kind }

// PackageType is the type of a package.
type PackageType struct {
	symbols map[string]Type
}

// NewPackageType returns a new package with the given symbols.
func NewPackageType(symbols map[string]Type) *PackageType {
	return &PackageType{symbols}
}

// String implements Type.
func (PackageType) String() string {
	return "package"
}

// Kind implements Type.
func (PackageType) Kind() Kind {
	return Pkg
}

// Symbol returns a symbol by the given name, or nil if none could be found.
func (t PackageType) Symbol(ident string) Type {
	if s, ok := t.symbols[ident]; ok {
		return s
	}
	return nil
}

// ArrayType is the type of array-like values.
type ArrayType struct {
	count int
	elem  Type
}

// NewArrayType returns a new array type.
func NewArrayType(count int, elem Type) *ArrayType {
	return &ArrayType{count: count, elem: elem}
}

// String implements Type.
func (t ArrayType) String() string {
	return fmt.Sprintf("[%d]%s", t.count, t.elem.String())
}

// Kind implements Type.
func (ArrayType) Kind() Kind {
	return Array
}

// Elem is the type of element in the array.
func (t ArrayType) Elem() Type {
	return t.elem
}

// Len is the length of the array.
func (t ArrayType) Len() int {
	return t.count
}

// SliceType is the type of array-like values.
type SliceType struct {
	elem Type
}

// NewSliceType returns a new array type.
func NewSliceType(elem Type) *SliceType {
	return &SliceType{elem: elem}
}

// String implements Type.
func (t SliceType) String() string {
	return "[]" + t.elem.String()
}

// Kind implements Type.
func (SliceType) Kind() Kind {
	return Slice
}

// Elem is the type of element in the slice.
func (t SliceType) Elem() Type {
	return t.elem
}

// MapType is the type of maps.
type MapType struct {
	key, val Type
}

// NewMapType returns a new map type.
func NewMapType(key Type, val Type) Type {
	return MapType{key: key, val: val}
}

// String implements Type.
func (t MapType) String() string {
	return "map[" + t.key.String() + "]" + t.val.String()
}

// Kind implements Type.
func (MapType) Kind() Kind {
	return Map
}

// Key is the type of the map's keys.
func (t MapType) Key() Type {
	return t.key
}

// Value is the type of the map's values.
func (t MapType) Value() Type {
	return t.val
}

// Field represents a struct field.
type Field struct {
	Name string
	Type Type
}

// StructType is the type of struct values.
type StructType struct {
	fields   []Field
	fieldMap map[string]Field
}

// NewStructType returns a new struct type.
func NewStructType(fields []Field) *StructType {
	fieldMap := map[string]Field{}
	for _, field := range fields {
		fieldMap[field.Name] = field
	}
	return &StructType{fields: fields, fieldMap: fieldMap}
}

// String implements Type.
func (t StructType) String() string {
	return "struct"
}

// Kind implements Type.
func (StructType) Kind() Kind {
	return Struct
}

// NumFields returns the number of fields in the struct.
func (t StructType) NumFields() int {
	return len(t.fields)
}

// Field returns the nth field in the struct.
func (t StructType) Field(i int) Field {
	return t.fields[i]
}

// FieldByName returns the field with the given name.
func (t StructType) FieldByName(name string) (Field, bool) {
	f, ok := t.fieldMap[name]
	return f, ok
}

// PtrType is the type of pointers.
type PtrType struct {
	elem Type
}

// NewPtrType returns a new pointer type.
func NewPtrType(elem Type) *PtrType {
	return &PtrType{elem: elem}
}

// String implements Type.
func (t PtrType) String() string {
	return "*" + t.elem.String()
}

// Kind implements Type.
func (PtrType) Kind() Kind {
	return Ptr
}

// Elem returns the element being pointed to by the pointer.
func (t PtrType) Elem() Type {
	return t.elem
}

// FuncType is the type of function values.
type FuncType struct {
	in       []Type
	out      []Type
	variadic bool
}

// NewFuncType returns a new function type.
func NewFuncType(in []Type, out []Type, variadic bool) *FuncType {
	return &FuncType{in: in, out: out, variadic: variadic}
}

// String implements Type.
func (t FuncType) String() string {
	return "func"
}

// Kind implements Type.
func (FuncType) Kind() Kind {
	return Func
}

// NumIn returns the number of input parameters.
func (t FuncType) NumIn() int {
	return len(t.in)
}

// In gets the nth input parameter.
func (t FuncType) In(i int) Type {
	return t.in[i]
}

// IsVariadic returns true for variadic functions.
func (t FuncType) IsVariadic() bool {
	return t.variadic
}

// NumOut returns the number of output parameters.
func (t FuncType) NumOut() int {
	return len(t.out)
}

// Out gets the nth output parameter.
func (t FuncType) Out(i int) Type {
	return t.out[i]
}

// TypeEqual returns true if the two types are equal.
func TypeEqual(a, b Type) bool {
	// TODO: this could be a bit more precise.
	return reflect.DeepEqual(a, b)
}

// toreflecttype converts an expr type into a runtime type.
func toreflecttype(t Type) reflect.Type {
	switch t := t.(type) {
	case *PrimitiveType:
		return primRType[t.Kind()]
	case *ArrayType:
		return reflect.ArrayOf(t.Len(), toreflecttype(t.Elem()))
	case *SliceType:
		return reflect.SliceOf(toreflecttype(t.Elem()))
	case *StructType:
		fields := make([]reflect.StructField, 0, t.NumFields())
		for i := 0; i < t.NumFields(); i++ {
			field := t.Field(i)
			fields = append(fields, reflect.StructField{
				Name: field.Name,
				Type: toreflecttype(field.Type),
			})
		}
		return reflect.StructOf(fields)
	case *MapType:
		return reflect.MapOf(toreflecttype(t.Key()), toreflecttype(t.Value()))
	case *PtrType:
		return reflect.PtrTo(toreflecttype(t.Elem()))
	case *FuncType:
		nin := t.NumIn()
		in := make([]reflect.Type, 0, nin)
		for i := 0; i < nin; i++ {
			in = append(in, toreflecttype(t.In(i)))
		}
		nout := t.NumOut()
		out := make([]reflect.Type, 0, nout)
		for i := 0; i < nout; i++ {
			out = append(out, toreflecttype(t.Out(i)))
		}
		return reflect.FuncOf(in, out, t.IsVariadic())
	default:
		panic(ErrNotRepresentable)
	}
}

var typemap = map[reflect.Type]Type{}
var typemutex = sync.Mutex{}

func savetype(reflect reflect.Type, expr Type) Type {
	typemutex.Lock()
	defer typemutex.Unlock()

	typemap[reflect] = expr
	return expr
}

func loadtype(reflect reflect.Type) (Type, bool) {
	typemutex.Lock()
	defer typemutex.Unlock()

	if expr, ok := typemap[reflect]; ok {
		return expr, true
	}
	return nil, false
}

// fromreflecttype converts a runtime type into an expr type.
func fromreflecttype(t reflect.Type) Type {
	if et, ok := loadtype(t); ok {
		return et
	}

	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.String:
		return primType[t.Kind()]
	case reflect.Array:
		return NewArrayType(t.Len(), fromreflecttype(t.Elem()))
	case reflect.Func:
		nin := t.NumIn()
		in := make([]Type, 0, nin)
		for i := 0; i < nin; i++ {
			in = append(in, fromreflecttype(t.In(i)))
		}
		nout := t.NumOut()
		out := make([]Type, 0, nout)
		for i := 0; i < nout; i++ {
			out = append(out, fromreflecttype(t.Out(i)))
		}
		return NewFuncType(in, out, t.IsVariadic())
	case reflect.Map:
		return NewMapType(fromreflecttype(t.Key()), fromreflecttype(t.Elem()))
	case reflect.Ptr:
		et := &PtrType{}
		savetype(t, et)
		*et = *NewPtrType(fromreflecttype(t.Elem()))
		return et
	case reflect.Slice:
		et := &SliceType{}
		savetype(t, et)
		*et = *NewSliceType(fromreflecttype(t.Elem()))
		return et
	case reflect.Struct:
		fields := make([]Field, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			fields = append(fields, Field{
				Name: field.Name,
				Type: fromreflecttype(field.Type),
			})
		}
		return NewStructType(fields)
	default:
		panic(ErrNotRepresentable)
	}
}

func assignable(from Type, to Type) bool {
	if TypeEqual(from, to) {
		return true
	}

	switch from.Kind() {
	case UntypedNil:
		switch to.Kind() {
		case Ptr, Func, Slice, Map:
			return true
		}

	case UntypedBool:
		switch to.Kind() {
		case Bool:
			return true
		}

	case UntypedInt:
		// TODO: Range and overflow checks.
		switch to.Kind() {
		case Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
			return true
		case Float32, Float64:
			return true
		}

	case UntypedFloat:
		switch to.Kind() {
		case Float32, Float64:
			return true
		}
	}
	return false
}

// TypeOf returns the type of a runtime value.
func TypeOf(i interface{}) Type {
	if pkg, ok := i.(Package); ok {
		return pkg.Type()
	}

	return fromreflecttype(reflect.TypeOf(i))
}
