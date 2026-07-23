//go:build !stdjson

package json

import (
	stdjson "encoding/json"
	"strconv"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func init() {
	// borrowed from https://github.com/json-iterator/go/issues/145#issuecomment-323483602
	decodeNumberAsInt64IfPossible := func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch iter.WhatIsNext() {
		case jsoniter.NumberValue:
			var number stdjson.Number

			iter.ReadVal(&number)
			i, err := strconv.ParseInt(string(number), 10, 64)

			if err == nil {
				*(*any)(ptr) = i
				return
			}

			f, err := strconv.ParseFloat(string(number), 64)
			if err == nil {
				*(*any)(ptr) = f
				return
			}
		default:
			*(*any)(ptr) = iter.Read()
		}
	}
	jsoniter.RegisterTypeDecoderFunc("interface {}", decodeNumberAsInt64IfPossible)
	jsoniter.RegisterTypeDecoderFunc("any", decodeNumberAsInt64IfPossible)
}

var (
	Marshal    = json.Marshal
	Unmarshal  = json.Unmarshal
	NewDecoder = json.NewDecoder
)
