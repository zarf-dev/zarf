package console

import (
	"io"
	"slices"
	"strconv"
	"time"
)

type buffer []byte

func (b *buffer) Grow(n int) {
	*b = slices.Grow(*b, n)
}

func (b *buffer) Bytes() []byte {
	return *b
}

func (b *buffer) String() string {
	return string(*b)
}

func (b *buffer) Len() int {
	return len(*b)
}

func (b *buffer) Cap() int {
	return cap(*b)
}

func (b *buffer) WriteTo(dst io.Writer) (int64, error) {
	l := len(*b)
	if l == 0 {
		return 0, nil
	}
	n, err := dst.Write(*b)
	if err != nil {
		return int64(n), err
	}
	if n < l {
		return int64(n), io.ErrShortWrite
	}
	b.Reset()
	return int64(n), nil
}

func (b *buffer) Reset() {
	*b = (*b)[:0]
}

func (b *buffer) Clone() buffer {
	return append(buffer(nil), *b...)
}

func (b *buffer) Clip() {
	*b = slices.Clip(*b)
}

func (b *buffer) copy(src *buffer) {
	if src.Len() > 0 {
		b.Append(src.Bytes())
	}
}

func (b *buffer) Append(data []byte) {
	*b = append(*b, data...)
}

func (b *buffer) AppendString(s string) {
	*b = append(*b, s...)
}

// func (b *buffer) AppendQuotedString(s string) {
// 	b.buff = strconv.AppendQuote(b.buff, s)
// }

func (b *buffer) AppendByte(byt byte) {
	*b = append(*b, byt)
}

func (b *buffer) AppendTime(t time.Time, format string) {
	*b = t.AppendFormat(*b, format)
}

func (b *buffer) AppendInt(i int64) {
	*b = strconv.AppendInt(*b, i, 10)
}

func (b *buffer) AppendUint(i uint64) {
	*b = strconv.AppendUint(*b, i, 10)
}

func (b *buffer) AppendFloat(i float64) {
	*b = strconv.AppendFloat(*b, i, 'g', -1, 64)
}

func (b *buffer) AppendBool(i bool) {
	*b = strconv.AppendBool(*b, i)
}

func (b *buffer) AppendDuration(d time.Duration) {
	*b = appendDuration(*b, d)
}
