// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"math/rand"
	"reflect"
	"strings"
)

const (
	// maxElements specifies maximum number of elements in array/slice
	maxElements = 10
	// maxStringLen specifies maximum string length
	maxStringLen = 20

	// the following unicode range covers:
	// - Latin-1 Supplement (0x00A0–0x00FF),
	// - Latin Extended-A (0x0100–0x017F),
	// - Latin Extended-B (0x0180–0x024F),
	// - IPA Extensions (0x0250–0x02AF).
	unicodeRangeLo = 0x00A0
	unicodeRangeHi = 0x02AF
)

// FillValue recursively populates v for round-trip fuzz tests. Struct fields that cannot be set via
// reflection (unexported) are left zero. Pointers are generated as nil, empty, or populated so
// their presence is tested independently from their contents.
func FillValue(v reflect.Value, rng *rand.Rand) {
	switch v.Kind() {
	case reflect.Pointer:
		switch rng.Intn(3) {
		case 0:
			return
		case 1:
			v.Set(reflect.New(v.Type().Elem()))
			return
		default:
			v.Set(reflect.New(v.Type().Elem()))
			FillValue(v.Elem(), rng)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Field(i); f.CanSet() {
				FillValue(f, rng)
			}
		}
	case reflect.Slice:
		n := 1 + rng.Intn(maxElements)
		s := reflect.MakeSlice(v.Type(), n, n)
		for i := range n {
			FillValue(s.Index(i), rng)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		for range 1 + rng.Intn(maxElements) {
			key := reflect.New(v.Type().Key()).Elem()
			FillValue(key, rng)
			val := reflect.New(v.Type().Elem()).Elem()
			FillValue(val, rng)
			m.SetMapIndex(key, val)
		}
		v.Set(m)
	case reflect.String:
		v.SetString(randString(rng, rng.Intn(maxStringLen)))
	case reflect.Bool:
		v.SetBool(rng.Intn(2) == 1)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(rng.Int31()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(rng.Uint32()))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(rng.Float64())
	}
}

func randString(rng *rand.Rand, n int) string {
	var sb strings.Builder
	span := int(unicodeRangeHi-unicodeRangeLo) + 1
	for range n {
		r := unicodeRangeLo + rune(rng.Intn(span))
		sb.WriteRune(r)
	}
	return sb.String()
}
