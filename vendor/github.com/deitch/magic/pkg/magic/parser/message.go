package parser

import (
	"encoding/binary"
	"fmt"
	"io"
)

// messageParser create a message given a string pattern, the ReaderAt and the position where the last read took place.
// reference https://man7.org/linux/man-pages/man3/printf.3.html
func messageParser(r io.ReaderAt, pos int64, pattern string) string {
	var (
		data []any
	)
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c != '%' {
			continue
		}
		var b []byte
		// it indicates a pattern
		i++
		c = pattern[i]
		// any additional flags, width, precision or modifiers?
		switch c {
		case '#', '+', '-', ' ', '0':
			i++
			c = pattern[i]
		}
		switch c {
		case 's':
			// string, we hit a null
			b2 := make([]byte, 1)
			for {
				n, err := r.ReadAt(b2, pos)
				if err != nil || n != len(b2) || b2[0] == 0 {
					break
				}
				b = append(b, b2[0])
				pos++
			}
			data = append(data, string(b))
		case '%':
			// '%%' is a literal '%'
		case 'd', 'i':
			// signed decimal integer
			b = make([]byte, 4)
			n, err := r.ReadAt(b, pos)
			if err != nil || n != len(b) {
				break
			}
			data = append(data, int32(binary.LittleEndian.Uint32(b)))
		case 'u', 'x', 'X':
			// signed decimal integer
			b = make([]byte, 4)
			n, err := r.ReadAt(b, pos)
			if err != nil || n != len(b) {
				break
			}
			data = append(data, binary.LittleEndian.Uint32(b))
		}
	}
	return fmt.Sprintf(pattern, data...)
}
