package bdb

import "encoding/binary"

func byteOrder(swapped bool) binary.ByteOrder {
	var order binary.ByteOrder = binary.LittleEndian
	if swapped {
		order = binary.BigEndian
	}
	return order
}
