package restruct

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/go-restruct/restruct/expr"
)

// ErrInvalidSize is returned when sizefrom is used on an invalid type.
var ErrInvalidSize = errors.New("size specified on fixed size type")

// ErrInvalidSizeOf is returned when sizefrom is used on an invalid type.
var ErrInvalidSizeOf = errors.New("sizeof specified on fixed size type")

// ErrInvalidSizeFrom is returned when sizefrom is used on an invalid type.
var ErrInvalidSizeFrom = errors.New("sizefrom specified on fixed size type")

// ErrInvalidBits is returned when bits is used on an invalid type.
var ErrInvalidBits = errors.New("bits specified on non-bitwise type")

// FieldFlags is a type for flags that can be applied to fields individually.
type FieldFlags uint64

const (
	// VariantBoolFlag causes the true value of a boolean to be ~0 instead of
	// just 1 (all bits are set.) This emulates the behavior of VARIANT_BOOL.
	VariantBoolFlag FieldFlags = 1 << iota

	// InvertedBoolFlag causes the true and false states of a boolean to be
	// flipped in binary.
	InvertedBoolFlag

	// RootFlag is set when the field points to the root struct.
	RootFlag

	// ParentFlag is set when the field points to the parent struct.
	ParentFlag

	// DefaultFlag is set when the field is designated as a switch case default.
	DefaultFlag
)

// Sizer is a type which has a defined size in binary. The SizeOf function
// returns how many bytes the type will consume in memory. This is used during
// encoding for allocation and therefore must equal the exact number of bytes
// the encoded form needs. You may use a pointer receiver even if the type is
// used by value.
type Sizer interface {
	SizeOf() int
}

// BitSizer is an interface for types that need to specify their own size in
// bit-level granularity. It has the same effect as Sizer.
type BitSizer interface {
	BitSize() int
}

// field represents a structure field, similar to reflect.StructField.
type field struct {
	Name       string
	Index      int
	BinaryType reflect.Type
	NativeType reflect.Type
	Order      binary.ByteOrder
	SIndex     int // Index of size field for a slice/string.
	TIndex     int // Index of target of sizeof field.
	Skip       int
	Trivial    bool
	BitSize    uint8
	Flags      FieldFlags
	IsRoot     bool
	IsParent   bool

	IfExpr     *expr.Program
	SizeExpr   *expr.Program
	BitsExpr   *expr.Program
	InExpr     *expr.Program
	OutExpr    *expr.Program
	WhileExpr  *expr.Program
	SwitchExpr *expr.Program
	CaseExpr   *expr.Program
}

// fields represents a structure.
type fields []field

var fieldCache = map[reflect.Type][]field{}
var cacheMutex = sync.RWMutex{}

// Elem constructs a transient field representing an element of an array, slice,
// or pointer.
func (f *field) Elem() field {
	// Special cases for string types, grumble grumble.
	t := f.BinaryType
	if t.Kind() == reflect.String {
		t = reflect.TypeOf([]byte{})
	}

	dt := f.NativeType
	if dt.Kind() == reflect.String {
		dt = reflect.TypeOf([]byte{})
	}

	return field{
		Name:       "*" + f.Name,
		Index:      -1,
		BinaryType: t.Elem(),
		NativeType: dt.Elem(),
		Order:      f.Order,
		TIndex:     -1,
		SIndex:     -1,
		Skip:       0,
		Trivial:    isTypeTrivial(t.Elem()),
	}
}

// fieldFromType returns a field from a reflected type.
func fieldFromType(typ reflect.Type) field {
	return field{
		Index:      -1,
		BinaryType: typ,
		NativeType: typ,
		Order:      nil,
		TIndex:     -1,
		SIndex:     -1,
		Skip:       0,
		Trivial:    isTypeTrivial(typ),
	}
}

func validBitType(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Float32,
		reflect.Complex64, reflect.Complex128:
		return true
	default:
		return false
	}
}

func validSizeType(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Slice, reflect.String:
		return true
	default:
		return false
	}
}

func parseExpr(sources ...string) *expr.Program {
	for _, s := range sources {
		if s != "" {
			return expr.ParseString(s)
		}
	}
	return nil
}

// fieldsFromStruct returns a slice of fields for binary packing and unpacking.
func fieldsFromStruct(typ reflect.Type) (result fields) {
	if typ.Kind() != reflect.Struct {
		panic(fmt.Errorf("tried to get fields from non-struct type %s", typ.Kind().String()))
	}

	count := typ.NumField()

	sizeOfMap := map[string]int{}

	for i := 0; i < count; i++ {
		val := typ.Field(i)

		// Skip unexported names (except _)
		if val.PkgPath != "" && val.Name != "_" {
			continue
		}

		// Parse struct tag
		opts := mustParseTag(val.Tag.Get("struct"))
		if opts.Ignore {
			continue
		}

		if opts.RootFlag {
			result = append(result, field{
				Name:  val.Name,
				Index: i,
				Flags: RootFlag,
			})
			continue
		}

		if opts.ParentFlag {
			result = append(result, field{
				Name:  val.Name,
				Index: i,
				Flags: ParentFlag,
			})
			continue
		}

		// Derive type
		ftyp := val.Type
		if opts.Type != nil {
			ftyp = opts.Type
		}

		// SizeOf
		sindex := -1
		tindex := -1
		if j, ok := sizeOfMap[val.Name]; ok {
			if !validSizeType(val.Type) {
				panic(ErrInvalidSizeOf)
			}
			sindex = j
			result[sindex].TIndex = i
			delete(sizeOfMap, val.Name)
		} else if opts.SizeOf != "" {
			sizeOfMap[opts.SizeOf] = i
		}

		// SizeFrom
		if opts.SizeFrom != "" {
			if !validSizeType(val.Type) {
				panic(ErrInvalidSizeFrom)
			}
			for j := 0; j < i; j++ {
				val := result[j]
				if opts.SizeFrom == val.Name {
					sindex = j
					result[sindex].TIndex = i
				}
			}
			if sindex == -1 {
				panic(fmt.Errorf("couldn't find SizeFrom field %s", opts.SizeFrom))
			}
		}

		// Expr
		ifExpr := parseExpr(opts.IfExpr, val.Tag.Get("struct-if"))
		sizeExpr := parseExpr(opts.SizeExpr, val.Tag.Get("struct-size"))
		bitsExpr := parseExpr(opts.BitsExpr, val.Tag.Get("struct-bits"))
		inExpr := parseExpr(opts.InExpr, val.Tag.Get("struct-in"))
		outExpr := parseExpr(opts.OutExpr, val.Tag.Get("struct-out"))
		whileExpr := parseExpr(opts.WhileExpr, val.Tag.Get("struct-while"))
		switchExpr := parseExpr(opts.SwitchExpr, val.Tag.Get("struct-switch"))
		caseExpr := parseExpr(opts.CaseExpr, val.Tag.Get("struct-case"))
		if sizeExpr != nil && !validSizeType(val.Type) {
			panic(ErrInvalidSize)
		}
		if bitsExpr != nil && !validBitType(ftyp) {
			panic(ErrInvalidBits)
		}

		// Flags
		flags := FieldFlags(0)
		if opts.VariantBoolFlag {
			flags |= VariantBoolFlag
		}
		if opts.InvertedBoolFlag {
			flags |= InvertedBoolFlag
		}
		if opts.DefaultFlag {
			flags |= DefaultFlag
		}

		result = append(result, field{
			Name:       val.Name,
			Index:      i,
			BinaryType: ftyp,
			NativeType: val.Type,
			Order:      opts.Order,
			SIndex:     sindex,
			TIndex:     tindex,
			Skip:       opts.Skip,
			Trivial:    isTypeTrivial(ftyp),
			BitSize:    opts.BitSize,
			Flags:      flags,
			IfExpr:     ifExpr,
			SizeExpr:   sizeExpr,
			BitsExpr:   bitsExpr,
			InExpr:     inExpr,
			OutExpr:    outExpr,
			WhileExpr:  whileExpr,
			SwitchExpr: switchExpr,
			CaseExpr:   caseExpr,
		})
	}

	for fieldName := range sizeOfMap {
		panic(fmt.Errorf("couldn't find SizeOf field %s", fieldName))
	}

	return
}

func cachedFieldsFromStruct(typ reflect.Type) (result fields) {
	cacheMutex.RLock()
	result, ok := fieldCache[typ]
	cacheMutex.RUnlock()

	if ok {
		return
	}

	result = fieldsFromStruct(typ)

	cacheMutex.Lock()
	fieldCache[typ] = result
	cacheMutex.Unlock()

	return
}

// isTypeTrivial determines if a given type is constant-size.
func isTypeTrivial(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Kind() {
	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128:
		return true
	case reflect.Array, reflect.Ptr:
		return isTypeTrivial(typ.Elem())
	case reflect.Struct:
		for _, field := range cachedFieldsFromStruct(typ) {
			if !isTypeTrivial(field.BinaryType) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (f *field) sizer(v reflect.Value) (Sizer, bool) {
	if s, ok := v.Interface().(Sizer); ok {
		return s, true
	}

	if !v.CanAddr() {
		return nil, false
	}

	if s, ok := v.Addr().Interface().(Sizer); ok {
		return s, true
	}

	return nil, false
}

func (f *field) bitSizer(v reflect.Value) (BitSizer, bool) {
	if s, ok := v.Interface().(BitSizer); ok {
		return s, true
	}

	if !v.CanAddr() {
		return nil, false
	}

	if s, ok := v.Addr().Interface().(BitSizer); ok {
		return s, true
	}

	return nil, false
}

func (f *field) bitSizeUsingInterface(val reflect.Value) (int, bool) {
	if s, ok := f.bitSizer(val); ok {
		return s.BitSize(), true
	}

	if s, ok := f.sizer(val); ok {
		return s.SizeOf() * 8, true
	}

	return 0, false
}
