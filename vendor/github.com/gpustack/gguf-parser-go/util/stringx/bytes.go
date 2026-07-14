package stringx

import "unsafe"

// FromBytes converts a byte slice to a string.
func FromBytes(b *[]byte) string {
	return unsafe.String(unsafe.SliceData(*b), len(*b))
}

// ToBytes converts a string to a byte slice,
// which is impossible to modify the item of slice.
func ToBytes(s *string) (bs []byte) {
	return unsafe.Slice(unsafe.StringData(*s), len(*s))
}
