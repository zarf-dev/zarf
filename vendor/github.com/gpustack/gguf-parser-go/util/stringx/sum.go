package stringx

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
)

// SumByFNV64a sums up the string(s) by FNV-64a hash algorithm.
func SumByFNV64a(s string, ss ...string) string {
	h := fnv.New64a()

	_, _ = h.Write(ToBytes(&s))
	for i := range ss {
		_, _ = h.Write(ToBytes(&ss[i]))
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// SumBytesByFNV64a sums up the byte slice(s) by FNV-64a hash algorithm.
func SumBytesByFNV64a(bs []byte, bss ...[]byte) string {
	h := fnv.New64a()

	_, _ = h.Write(bs)
	for i := range bss {
		_, _ = h.Write(bss[i])
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// SumBySHA256 sums up the string(s) by SHA256 hash algorithm.
func SumBySHA256(s string, ss ...string) string {
	h := sha256.New()

	_, _ = h.Write(ToBytes(&s))
	for i := range ss {
		_, _ = h.Write(ToBytes(&ss[i]))
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// SumBytesBySHA256 sums up the byte slice(s) by SHA256 hash algorithm.
func SumBytesBySHA256(bs []byte, bss ...[]byte) string {
	h := sha256.New()

	_, _ = h.Write(bs)
	for i := range bss {
		_, _ = h.Write(bss[i])
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// SumBySHA224 sums up the string(s) by SHA224 hash algorithm.
func SumBySHA224(s string, ss ...string) string {
	h := sha256.New224()

	_, _ = h.Write(ToBytes(&s))
	for i := range ss {
		_, _ = h.Write(ToBytes(&ss[i]))
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// SumBytesBySHA224 sums up the byte slice(s) by SHA224 hash algorithm.
func SumBytesBySHA224(bs []byte, bss ...[]byte) string {
	h := sha256.New224()

	_, _ = h.Write(bs)
	for i := range bss {
		_, _ = h.Write(bss[i])
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}
