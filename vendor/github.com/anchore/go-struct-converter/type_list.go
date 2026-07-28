package converter

import (
	"iter"
	"reflect"
	"unsafe"
)

//go:linkname typelinks reflect.typelinks
func typelinks() (sections []unsafe.Pointer, offset [][]int32)

//go:linkname add reflect.add
func add(_ unsafe.Pointer, _ uintptr, _ string) unsafe.Pointer

func listAllBaseTypes() iter.Seq[reflect.Type] {
	return func(yield func(reflect.Type) bool) {
		sections, offsets := typelinks()
		for i, base := range sections {
			for _, offset := range offsets[i] {
				typeAddr := add(base, uintptr(offset), "")
				typ := reflect.TypeOf(*(*any)(unsafe.Pointer(&typeAddr)))
				typ = baseType(typ)
				for typ.Kind() == reflect.Pointer {
					typ = typ.Elem()
				}
				if !yield(typ) {
					return
				}
			}
		}
	}
}
