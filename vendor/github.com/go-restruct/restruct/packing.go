/*
Package restruct implements packing and unpacking of raw binary formats.

Structures can be created with struct tags annotating the on-disk or in-memory
layout of the structure, using the "struct" struct tag, like so:

	struct {
		Length int `struct:"int32,sizeof=Packets"`
		Packets []struct{
			Source    string    `struct:"[16]byte"`
			Timestamp int       `struct:"int32,big"`
			Data      [256]byte `struct:"skip=8"`
		}
	}

To unpack data in memory to this structure, simply use Unpack with a byte slice:

	msg := Message{}
	restruct.Unpack(data, binary.LittleEndian, &msg)
*/
package restruct

import (
	"encoding/binary"
	"reflect"
)

func fieldFromIntf(v interface{}) (field, reflect.Value) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	f := fieldFromType(val.Type())
	return f, val
}

/*
Unpack reads data from a byteslice into a value.

Two types of values are directly supported here: Unpackers and structs. You can
pass them by value or by pointer, although it is an error if Restruct is
unable to set a value because it is unaddressable.

For structs, each field will be read sequentially based on a straightforward
interpretation of the type. For example, an int32 will be read as a 32-bit
signed integer, taking 4 bytes of memory. Structures and arrays are laid out
flat with no padding or metadata.

Unexported fields are ignored, except for fields named _ - those fields will
be treated purely as padding. Padding will not be preserved through packing
and unpacking.

The behavior of deserialization can be customized using struct tags. The
following struct tag syntax is supported:

	`struct:"[flags...]"`

Flags are comma-separated keys. The following are available:

	type              A bare type name, e.g. int32 or []string. For integer
	                  types, it is possible to specify the number of bits,
	                  allowing the definition of bitfields, by appending a
	                  colon followed by the number of bits. For example,
	                  uint32:20 would specify a field that is 20 bits long.

	sizeof=[Field]    Specifies that the field should be treated as a count of
	                  the number of elements in Field.

	sizefrom=[Field]  Specifies that the field should determine the number of
	                  elements in itself by reading the counter in Field.

	skip=[Count]      Skips Count bytes before the field. You can use this to
	                  e.g. emulate C structure alignment.

	big,msb           Specifies big endian byte order. When applied to
	                  structs, this will apply to all fields under the struct.

	little,lsb        Specifies little endian byte order. When applied to
	                  structs, this will apply to all fields under the struct.

	variantbool       Specifies that the boolean `true` value should be
	                  encoded as -1 instead of 1.

	invertedbool      Specifies that the `true` and `false` encodings for
	                  boolean should be swapped.
*/
func Unpack(data []byte, order binary.ByteOrder, v interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if err, ok = r.(error); !ok {
				panic(err)
			}
		}
	}()

	f, val := fieldFromIntf(v)
	ss := structstack{allowexpr: expressionsEnabled, buf: data}
	d := decoder{structstack: ss, order: order}
	d.read(f, val)

	return
}

/*
SizeOf returns the binary encoded size of the given value, in bytes.
*/
func SizeOf(v interface{}) (size int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	ss := structstack{allowexpr: expressionsEnabled}
	f, val := fieldFromIntf(v)
	return ss.fieldbytes(f, val), nil
}

/*
BitSize returns the binary encoded size of the given value, in bits.
*/
func BitSize(v interface{}) (size int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	ss := structstack{allowexpr: expressionsEnabled}
	f, val := fieldFromIntf(v)
	return ss.fieldbits(f, val), nil
}

/*
Pack writes data from a datastructure into a byteslice.

Two types of values are directly supported here: Packers and structs. You can
pass them by value or by pointer.

Each structure is serialized in the same way it would be deserialized with
Unpack. See Unpack documentation for the struct tag format.
*/
func Pack(order binary.ByteOrder, v interface{}) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			data = nil
			err = r.(error)
		}
	}()

	ss := structstack{allowexpr: expressionsEnabled, buf: []byte{}}

	f, val := fieldFromIntf(v)
	data = make([]byte, ss.fieldbytes(f, val))

	ss.buf = data
	e := encoder{structstack: ss, order: order}
	e.write(f, val)

	return
}
