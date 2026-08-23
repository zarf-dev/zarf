package restruct

import (
	"fmt"
	"reflect"

	"github.com/go-restruct/restruct/expr"
)

type switchcase struct {
	f field
	v reflect.Value
}

type structstack struct {
	buf       []byte
	stack     []reflect.Value
	allowexpr bool
}

func (s *structstack) Resolve(ident string) expr.Value {
	switch ident {
	case "_eof":
		return expr.ValueOf(len(s.buf) == 0)
	default:
		if t := stdLibResolver.Resolve(ident); t != nil {
			return t
		}
		if len(s.stack) > 0 {
			if sv := s.stack[len(s.stack)-1].FieldByName(ident); sv.IsValid() {
				return expr.ValueOf(sv.Interface())
			}
		}
		return nil
	}
}

func (s *structstack) evalBits(f field) int {
	bits := 0
	if f.BitSize != 0 {
		bits = int(f.BitSize)
	}
	if f.BitsExpr != nil {
		bits = reflect.ValueOf(s.evalExpr(f.BitsExpr)).Convert(reflect.TypeOf(int(0))).Interface().(int)
	}
	return bits
}

func (s *structstack) evalSize(f field) int {
	size := 0
	if f.SizeExpr != nil {
		size = reflect.ValueOf(s.evalExpr(f.SizeExpr)).Convert(reflect.TypeOf(int(0))).Interface().(int)
	}
	return size
}

func (s *structstack) evalIf(f field) bool {
	if f.IfExpr == nil {
		return true
	}
	if b, ok := s.evalExpr(f.IfExpr).(bool); ok {
		return b
	}
	panic("expected bool value for if expr")
}

func (s *structstack) evalWhile(f field) bool {
	if b, ok := s.evalExpr(f.WhileExpr).(bool); ok {
		return b
	}
	panic("expected bool value for while expr")
}

func (s *structstack) switcbits(f field, v reflect.Value, on interface{}) (size int) {
	var def *switchcase

	if v.Kind() != reflect.Struct {
		panic(fmt.Errorf("%s: only switches on structs are valid", f.Name))
	}

	sfields := cachedFieldsFromStruct(f.BinaryType)
	l := len(sfields)

	for i := 0; i < l; i++ {
		f := sfields[i]
		v := v.Field(f.Index)

		if f.Flags&DefaultFlag != 0 {
			if def != nil {
				panic(fmt.Errorf("%s: only one default case is allowed", f.Name))
			}
			def = &switchcase{f, v}
			continue
		}

		if f.CaseExpr == nil {
			panic(fmt.Errorf("%s: only cases are valid inside switches", f.Name))
		}

		if s.evalExpr(f.CaseExpr) == on {
			return s.fieldbits(f, v)
		}
	}

	if def != nil {
		return s.fieldbits(def.f, def.v)
	}

	return 0
}

// fieldbits determines the encoded size of a field in bits.
func (s *structstack) fieldbits(f field, val reflect.Value) (size int) {
	skipBits := f.Skip * 8

	if f.Flags&RootFlag == RootFlag {
		s.setancestor(f, val, s.root())
		return 0
	}

	if f.Flags&ParentFlag == ParentFlag {
		for i := 1; i < len(s.stack); i++ {
			if s.setancestor(f, val, s.ancestor(i)) {
				break
			}
		}
		return 0
	}

	if f.SwitchExpr != nil {
		return s.switcbits(f, val, s.evalExpr(f.SwitchExpr))
	}

	if f.Name != "_" {
		if s, ok := f.bitSizeUsingInterface(val); ok {
			return s
		}
	} else {
		// Non-trivial, unnamed fields do not make sense. You can't set a field
		// with no name, so the elements can't possibly differ.
		// N.B.: Though skip will still work, use struct{} instead for skip.
		if !isTypeTrivial(val.Type()) {
			return skipBits
		}
	}

	if !s.evalIf(f) {
		return 0
	}

	if b := s.evalBits(f); b != 0 {
		return b
	}

	alen := 1
	switch f.BinaryType.Kind() {
	case reflect.Int8, reflect.Uint8, reflect.Bool:
		return 8 + skipBits
	case reflect.Int16, reflect.Uint16:
		return 16 + skipBits
	case reflect.Int, reflect.Int32,
		reflect.Uint, reflect.Uint32,
		reflect.Float32:
		return 32 + skipBits
	case reflect.Int64, reflect.Uint64,
		reflect.Float64, reflect.Complex64:
		return 64 + skipBits
	case reflect.Complex128:
		return 128 + skipBits
	case reflect.Slice, reflect.String:
		switch f.NativeType.Kind() {
		case reflect.Slice, reflect.String, reflect.Array, reflect.Ptr:
			alen = val.Len()
		default:
			return 0
		}
		fallthrough
	case reflect.Array, reflect.Ptr:
		size += skipBits

		// If array type, get length from type.
		if f.BinaryType.Kind() == reflect.Array {
			alen = f.BinaryType.Len()
		}

		// Optimization: if the array/slice is empty, bail now.
		if alen == 0 {
			return size
		}

		switch f.NativeType.Kind() {
		case reflect.Ptr:
			return s.fieldbits(f.Elem(), val.Elem())
		case reflect.Slice, reflect.String, reflect.Array:
			// Optimization: if the element type is trivial, we can derive the
			// length from a single element.
			elem := f.Elem()
			if elem.Trivial {
				size += s.fieldbits(elem, reflect.Zero(elem.BinaryType)) * alen
			} else {
				for i := 0; i < alen; i++ {
					size += s.fieldbits(elem, val.Index(i))
				}
			}
		}
		return size
	case reflect.Struct:
		size += skipBits
		s.push(val)
		for _, field := range cachedFieldsFromStruct(f.BinaryType) {
			if field.BitSize != 0 {
				size += int(field.BitSize)
			} else {
				size += s.fieldbits(field, val.Field(field.Index))
			}
		}
		s.pop(val)
		return size
	default:
		return 0
	}
}

// fieldbytes returns the effective size in bytes, for the few cases where
// byte sizes are needed.
func (s *structstack) fieldbytes(f field, val reflect.Value) (size int) {
	return (s.fieldbits(f, val) + 7) / 8
}

func (s *structstack) fieldsbits(fields fields, val reflect.Value) (size int) {
	for _, field := range fields {
		size += s.fieldbits(field, val.Field(field.Index))
	}
	return
}

func (s *structstack) evalExpr(program *expr.Program) interface{} {
	if !s.allowexpr {
		panic("call restruct.EnableExprBeta() to eanble expressions beta")
	}
	v, err := expr.EvalProgram(s, program)
	if err != nil {
		panic(err)
	}
	return v
}

func (s *structstack) push(v reflect.Value) {
	s.stack = append(s.stack, v)
}

func (s *structstack) pop(v reflect.Value) {
	var p reflect.Value
	s.stack, p = s.stack[:len(s.stack)-1], s.stack[len(s.stack)-1]
	if p != v {
		panic("struct stack misaligned")
	}
}

func (s *structstack) setancestor(f field, v reflect.Value, ancestor reflect.Value) bool {
	if ancestor.Kind() != reflect.Ptr {
		if !ancestor.CanAddr() {
			return false
		}
		ancestor = ancestor.Addr()
	}
	if ancestor.Type().AssignableTo(v.Type()) {
		v.Set(ancestor)
		return true
	}
	return false
}

func (s *structstack) root() reflect.Value {
	if len(s.stack) > 0 {
		return s.stack[0]
	}
	return reflect.ValueOf(nil)
}

func (s *structstack) ancestor(generation int) reflect.Value {
	if len(s.stack) > generation {
		return s.stack[len(s.stack)-generation-1]
	}
	return reflect.ValueOf(nil)
}
