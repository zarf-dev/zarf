package derivation

import (
	"unsafe"
)

// unsafeString takes a byte slice and returns it as a string.
// Consider this method as passing ownership of the byte slice,
// do not mutate it afterwards.
func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// unsafeBytes returns the byte slice backing the string s.
// It's safe to use in situations like hash calculations or
// writing into buffers.
func unsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
