package ptr

import (
	"time"

	"golang.org/x/exp/constraints"
)

func Int(v int) *int {
	return Ref(v)
}

func IntDeref(v *int, def int) int {
	return Deref(v, def)
}

func Int8(v int8) *int8 {
	return Ref(v)
}

func Int8Deref(v *int8, def int8) int8 {
	return Deref(v, def)
}

func Int16(v int16) *int16 {
	return Ref(v)
}

func Int16Deref(v *int16, def int16) int16 {
	return Deref(v, def)
}

func Int32(v int32) *int32 {
	return Ref(v)
}

func Int32Deref(v *int32, def int32) int32 {
	return Deref(v, def)
}

func Int64(v int64) *int64 {
	return Ref(v)
}

func Int64Deref(v *int64, def int64) int64 {
	return Deref(v, def)
}

func Uint(v uint) *uint {
	return Ref(v)
}

func UintDeref(v *uint, def uint) uint {
	return Deref(v, def)
}

func Uint8(v uint8) *uint8 {
	return Ref(v)
}

func Uint8Deref(v *uint8, def uint8) uint8 {
	return Deref(v, def)
}

func Uint16(v uint16) *uint16 {
	return Ref(v)
}

func Uint16Deref(v *uint16, def uint16) uint16 {
	return Deref(v, def)
}

func Uint32(v uint32) *uint32 {
	return Ref(v)
}

func Uint32Deref(v *uint32, def uint32) uint32 {
	return Deref(v, def)
}

func Uint64(v uint64) *uint64 {
	return Ref(v)
}

func Uint64Deref(v *uint64, def uint64) uint64 {
	return Deref(v, def)
}

func Float32(v float32) *float32 {
	return Ref(v)
}

func Float32Deref(v *float32, def float32) float32 {
	return Deref(v, def)
}

func Float64(v float64) *float64 {
	return Ref(v)
}

func Float64Deref(v *float64, def float64) float64 {
	return Deref(v, def)
}

func String(v string) *string {
	return Ref(v)
}

func StringDeref(v *string, def string) string {
	return Deref(v, def)
}

func Bool(v bool) *bool {
	return Ref(v)
}

func BoolDeref(v *bool, def bool) bool {
	return Deref(v, def)
}

func Duration(v time.Duration) *time.Duration {
	return Ref(v)
}

func DurationDeref(v *time.Duration, def time.Duration) time.Duration {
	return Deref(v, def)
}

func Time(v time.Time) *time.Time {
	return Ref(v)
}

func TimeDeref(v *time.Time, def time.Time) time.Time {
	return Deref(v, def)
}

type Pointerable interface {
	constraints.Ordered | ~bool | time.Time
}

func Ref[T Pointerable](v T) *T {
	return &v
}

func To[T Pointerable](v T) *T {
	return Ref(v)
}

func Deref[T Pointerable](ptr *T, def T) T {
	if ptr != nil {
		return *ptr
	}

	return def
}

func Equal[T Pointerable](a, b *T) bool {
	if a != nil && b != nil {
		return *a == *b
	}

	return false
}
