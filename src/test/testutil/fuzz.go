// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"fmt"
	"math/rand"
	"reflect"
)

// FillValue recursively populates v with random, non-zero values for round-trip fuzz tests. Struct
// fields that cannot be set via reflection (unexported) are left zero. Pointers are left nil roughly
// half the time so both the set and unset states are exercised.
func FillValue(v reflect.Value, rng *rand.Rand) {
	switch v.Kind() {
	case reflect.Pointer:
		if rng.Intn(2) == 0 {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		FillValue(v.Elem(), rng)
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Field(i); f.CanSet() {
				FillValue(f, rng)
			}
		}
	case reflect.Slice:
		n := 1 + rng.Intn(2)
		s := reflect.MakeSlice(v.Type(), n, n)
		for i := range n {
			FillValue(s.Index(i), rng)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		for range 1 + rng.Intn(2) {
			key := reflect.New(v.Type().Key()).Elem()
			FillValue(key, rng)
			val := reflect.New(v.Type().Elem()).Elem()
			FillValue(val, rng)
			m.SetMapIndex(key, val)
		}
		v.Set(m)
	case reflect.String:
		v.SetString(fmt.Sprintf("s%d", rng.Intn(1<<30)))
	case reflect.Bool:
		v.SetBool(rng.Intn(2) == 1)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(1 + rng.Intn(1000)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(1 + rng.Intn(1000)))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(1 + rng.Intn(1000)))
	}
}
