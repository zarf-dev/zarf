package restruct

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
)

// Unpacker is a type capable of unpacking a binary representation of itself
// into a native representation. The Unpack function is expected to consume
// a number of bytes from the buffer, then return a slice of the remaining
// bytes in the buffer. You may use a pointer receiver even if the type is
// used by value.
type Unpacker interface {
	Unpack(buf []byte, order binary.ByteOrder) ([]byte, error)
}

type decoder struct {
	structstack
	order      binary.ByteOrder
	sfields    []field
	bitCounter uint8
	bitSize    int
}

func putBit(buf []byte, bitSize int, bit int, val byte) {
	bit = bitSize - 1 - bit
	buf[len(buf)-bit/8-1] |= (val) << (uint(bit) % 8)
}

func (d *decoder) readBit() byte {
	value := (d.buf[0] >> uint(7-d.bitCounter)) & 1
	d.bitCounter++
	if d.bitCounter >= 8 {
		d.buf = d.buf[1:]
		d.bitCounter -= 8
	}
	return value
}

func (d *decoder) readBits(f field, outBuf []byte) {
	var decodedBits int

	// Determine encoded size in bits.
	if d.bitSize == 0 {
		decodedBits = 8 * len(outBuf)
	} else {
		decodedBits = int(d.bitSize)
	}

	// Crop output buffer to relevant bytes only.
	outBuf = outBuf[len(outBuf)-(decodedBits+7)/8:]

	if d.bitCounter == 0 && decodedBits%8 == 0 {
		// Fast path: we are fully byte-aligned.
		copy(outBuf, d.buf)
		d.buf = d.buf[len(outBuf):]
	} else {
		// Slow path: work bit-by-bit.
		// TODO: This needs to be optimized in a way that can be easily
		// understood; the previous optimized version was simply too hard to
		// reason about.
		for i := 0; i < decodedBits; i++ {
			putBit(outBuf, decodedBits, i, d.readBit())
		}
	}
}

func (d *decoder) read8(f field) uint8 {
	b := make([]byte, 1)
	d.readBits(f, b)
	return uint8(b[0])
}

func (d *decoder) read16(f field) uint16 {
	b := make([]byte, 2)
	d.readBits(f, b)
	return d.order.Uint16(b)
}

func (d *decoder) read32(f field) uint32 {
	b := make([]byte, 4)
	d.readBits(f, b)
	return d.order.Uint32(b)
}

func (d *decoder) read64(f field) uint64 {
	b := make([]byte, 8)
	d.readBits(f, b)
	return d.order.Uint64(b)
}

func (d *decoder) readS8(f field) int8 { return int8(d.read8(f)) }

func (d *decoder) readS16(f field) int16 { return int16(d.read16(f)) }

func (d *decoder) readS32(f field) int32 { return int32(d.read32(f)) }

func (d *decoder) readS64(f field) int64 { return int64(d.read64(f)) }

func (d *decoder) readBytes(count int) []byte {
	x := d.buf[0:count]
	d.buf = d.buf[count:]
	return x
}

func (d *decoder) skipBits(count int) {
	d.bitCounter += uint8(count % 8)
	if d.bitCounter > 8 {
		d.bitCounter -= 8
		count += 8
	}
	d.buf = d.buf[count/8:]
}

func (d *decoder) skip(f field, v reflect.Value) {
	d.skipBits(d.fieldbits(f, v))
}

func (d *decoder) unpacker(v reflect.Value) (Unpacker, bool) {
	if s, ok := v.Interface().(Unpacker); ok {
		return s, true
	}

	if !v.CanAddr() {
		return nil, false
	}

	if s, ok := v.Addr().Interface().(Unpacker); ok {
		return s, true
	}

	return nil, false
}

func (d *decoder) setUint(f field, v reflect.Value, x uint64) {
	switch v.Kind() {
	case reflect.Bool:
		b := x != 0
		if f.Flags&InvertedBoolFlag == InvertedBoolFlag {
			b = !b
		}
		v.SetBool(b)
	default:
		v.SetUint(x)
	}
}

func (d *decoder) setInt(f field, v reflect.Value, x int64) {
	switch v.Kind() {
	case reflect.Bool:
		b := x != 0
		if f.Flags&InvertedBoolFlag == InvertedBoolFlag {
			b = !b
		}
		v.SetBool(b)
	default:
		v.SetInt(x)
	}
}

func (d *decoder) switc(f field, v reflect.Value, on interface{}) {
	var def *switchcase

	if v.Kind() != reflect.Struct {
		panic(fmt.Errorf("%s: only switches on structs are valid", f.Name))
	}

	sfields := cachedFieldsFromStruct(f.BinaryType)
	l := len(sfields)

	// Zero out values for decoding.
	for i := 0; i < l; i++ {
		v := v.Field(f.Index)
		v.Set(reflect.Zero(v.Type()))
	}

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

		if d.evalExpr(f.CaseExpr) == on {
			d.read(f, v)
			return
		}
	}

	if def != nil {
		d.read(def.f, def.v)
	}
}

func (d *decoder) read(f field, v reflect.Value) {
	if f.Flags&RootFlag == RootFlag {
		d.setancestor(f, v, d.root())
		return
	}

	if f.Flags&ParentFlag == ParentFlag {
		for i := 1; i < len(d.stack); i++ {
			if d.setancestor(f, v, d.ancestor(i)) {
				break
			}
		}
		return
	}

	if f.SwitchExpr != nil {
		d.switc(f, v, d.evalExpr(f.SwitchExpr))
		return
	}

	struc := d.ancestor(0)

	if f.Name != "_" {
		if s, ok := d.unpacker(v); ok {
			var err error
			d.buf, err = s.Unpack(d.buf, d.order)
			if err != nil {
				panic(err)
			}
			return
		}
	} else {
		d.skipBits(d.fieldbits(f, v))
		return
	}

	if !d.evalIf(f) {
		return
	}

	sfields := d.sfields
	order := d.order

	if f.Order != nil {
		d.order = f.Order
		defer func() { d.order = order }()
	}

	if f.Skip != 0 {
		d.skipBits(f.Skip * 8)
	}

	d.bitSize = d.evalBits(f)
	alen := d.evalSize(f)

	if alen == 0 && f.SIndex != -1 {
		sv := struc.Field(f.SIndex)
		l := len(sfields)
		for i := 0; i < l; i++ {
			if sfields[i].Index != f.SIndex {
				continue
			}
			sf := sfields[i]
			// Must use different codepath for signed/unsigned.
			switch sf.BinaryType.Kind() {
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				alen = int(sv.Int())
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				alen = int(sv.Uint())
			default:
				panic(fmt.Errorf("unsupported size type %s: %s", sf.BinaryType.String(), sf.Name))
			}
			break
		}
	}

	switch f.BinaryType.Kind() {
	case reflect.Array:
		l := f.BinaryType.Len()

		// If the underlying value is a slice, initialize it.
		if f.NativeType.Kind() == reflect.Slice {
			v.Set(reflect.MakeSlice(reflect.SliceOf(f.NativeType.Elem()), l, l))
		}

		switch f.NativeType.Kind() {
		case reflect.String:
			// When using strings, treat as C string.
			str := string(d.readBytes(d.fieldbytes(f, v)))
			nul := strings.IndexByte(str, 0)
			if nul != -1 {
				str = str[0:nul]
			}
			v.SetString(str)
		case reflect.Slice, reflect.Array:
			ef := f.Elem()
			for i := 0; i < l; i++ {
				d.read(ef, v.Index(i))
			}
		default:
			panic(fmt.Errorf("invalid array cast type: %s", f.NativeType.String()))
		}

	case reflect.Struct:
		d.push(v)
		d.sfields = cachedFieldsFromStruct(f.BinaryType)
		l := len(d.sfields)
		for i := 0; i < l; i++ {
			f := d.sfields[i]
			v := v.Field(f.Index)
			if v.CanSet() {
				d.read(f, v)
			} else {
				d.skip(f, v)
			}
		}
		d.sfields = sfields
		d.pop(v)

	case reflect.Ptr:
		v.Set(reflect.New(v.Type().Elem()))
		d.read(f.Elem(), v.Elem())

	case reflect.Slice, reflect.String:
		fixed := func() {
			switch f.NativeType.Elem().Kind() {
			case reflect.Uint8:
				v.SetBytes(d.readBytes(d.fieldbytes(f, v)))
			default:
				ef := f.Elem()
				for i := 0; i < alen; i++ {
					d.read(ef, v.Index(i))
				}
			}
		}
		switch f.NativeType.Kind() {
		case reflect.String:
			v.SetString(string(d.readBytes(alen)))
		case reflect.Array:
			if f.WhileExpr != nil {
				i := 0
				ef := f.Elem()
				for d.evalWhile(f) {
					d.read(ef, v.Index(i))
					i++
				}
			} else {
				fixed()
			}
		case reflect.Slice:
			if f.WhileExpr != nil {
				switch f.NativeType.Kind() {
				case reflect.Slice:
					ef := f.Elem()
					for d.evalWhile(f) {
						nv := reflect.New(ef.NativeType).Elem()
						d.read(ef, nv)
						v.Set(reflect.Append(v, nv))
					}
				}
			} else {
				v.Set(reflect.MakeSlice(f.NativeType, alen, alen))
				fixed()
			}
		default:
			panic(fmt.Errorf("invalid array cast type: %s", f.NativeType.String()))
		}

	case reflect.Int8:
		d.setInt(f, v, int64(d.readS8(f)))
	case reflect.Int16:
		d.setInt(f, v, int64(d.readS16(f)))
	case reflect.Int32:
		d.setInt(f, v, int64(d.readS32(f)))
	case reflect.Int64:
		d.setInt(f, v, d.readS64(f))

	case reflect.Uint8, reflect.Bool:
		d.setUint(f, v, uint64(d.read8(f)))
	case reflect.Uint16:
		d.setUint(f, v, uint64(d.read16(f)))
	case reflect.Uint32:
		d.setUint(f, v, uint64(d.read32(f)))
	case reflect.Uint64:
		d.setUint(f, v, d.read64(f))

	case reflect.Float32:
		v.SetFloat(float64(math.Float32frombits(d.read32(f))))
	case reflect.Float64:
		v.SetFloat(math.Float64frombits(d.read64(f)))

	case reflect.Complex64:
		v.SetComplex(complex(
			float64(math.Float32frombits(d.read32(f))),
			float64(math.Float32frombits(d.read32(f))),
		))
	case reflect.Complex128:
		v.SetComplex(complex(
			math.Float64frombits(d.read64(f)),
			math.Float64frombits(d.read64(f)),
		))
	}

	if f.InExpr != nil {
		v.Set(reflect.ValueOf(d.evalExpr(f.InExpr)))
	}
}
