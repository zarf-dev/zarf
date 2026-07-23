package gguf_parser

import (
	"errors"
	"fmt"
	"slices"
)

// Types for GGMLType.
type (
	// GGMLType is a type of GGML tensor,
	// see https://github.com/ggml-org/llama.cpp/blob/fd1234cb468935ea087d6929b2487926c3afff4b/ggml/include/ggml.h#L368-L410.
	GGMLType uint32

	// GGMLTypeTrait holds the trait of a GGMLType,
	// see https://github.com/ggml-org/llama.cpp/blob/fd1234cb468935ea087d6929b2487926c3afff4b/ggml/src/ggml.c#L586-L876.
	GGMLTypeTrait struct {
		BlockSize uint64 // Original is int, in order to reduce conversion, here we use uint64.
		TypeSize  uint64 // Original is uint32, in order to reduce conversion, here we use uint64.
		Quantized bool
	}
)

// GGMLType constants.
//
// GGMLTypeQ4_2, GGMLTypeQ4_3 are deprecated.
// GGMLTypeQ4_0_4_4, GGMLTypeQ4_0_4_8, GGMLTypeQ4_0_8_8 are deprecated.
// GGMLTypeIQ4_NL_4_4, GGMLTypeIQ4_NL_4_8, GGMLTypeIQ4_NL_8_8 are deprecated.
const (
	GGMLTypeF32 GGMLType = iota
	GGMLTypeF16
	GGMLTypeQ4_0
	GGMLTypeQ4_1
	GGMLTypeQ4_2
	GGMLTypeQ4_3
	GGMLTypeQ5_0
	GGMLTypeQ5_1
	GGMLTypeQ8_0
	GGMLTypeQ8_1
	GGMLTypeQ2_K
	GGMLTypeQ3_K
	GGMLTypeQ4_K
	GGMLTypeQ5_K
	GGMLTypeQ6_K
	GGMLTypeQ8_K
	GGMLTypeIQ2_XXS
	GGMLTypeIQ2_XS
	GGMLTypeIQ3_XXS
	GGMLTypeIQ1_S
	GGMLTypeIQ4_NL
	GGMLTypeIQ3_S
	GGMLTypeIQ2_S
	GGMLTypeIQ4_XS
	GGMLTypeI8
	GGMLTypeI16
	GGMLTypeI32
	GGMLTypeI64
	GGMLTypeF64
	GGMLTypeIQ1_M
	GGMLTypeBF16
	GGMLTypeQ4_0_4_4
	GGMLTypeQ4_0_4_8
	GGMLTypeQ4_0_8_8
	GGMLTypeTQ1_0
	GGMLTypeTQ2_0
	GGMLTypeIQ4_NL_4_4
	GGMLTypeIQ4_NL_4_8
	GGMLTypeIQ4_NL_8_8
	GGMLTypeMXFP4
	_GGMLTypeCount // Unknown
)

// _GGMLTypeTraits is a table of GGMLTypeTrait for GGMLType.
var _GGMLTypeTraits = map[GGMLType]GGMLTypeTrait{
	GGMLTypeF32:        {BlockSize: 1, TypeSize: 4},
	GGMLTypeF16:        {BlockSize: 1, TypeSize: 2},
	GGMLTypeQ4_0:       {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeQ4_1:       {BlockSize: 32, TypeSize: 20, Quantized: true},
	GGMLTypeQ4_2:       {BlockSize: 0, TypeSize: 0}, // Deprecated
	GGMLTypeQ4_3:       {BlockSize: 0, TypeSize: 0}, // Deprecated
	GGMLTypeQ5_0:       {BlockSize: 32, TypeSize: 22, Quantized: true},
	GGMLTypeQ5_1:       {BlockSize: 32, TypeSize: 24, Quantized: true},
	GGMLTypeQ8_0:       {BlockSize: 32, TypeSize: 34, Quantized: true},
	GGMLTypeQ8_1:       {BlockSize: 32, TypeSize: 36, Quantized: true},
	GGMLTypeQ2_K:       {BlockSize: 256, TypeSize: 84, Quantized: true},
	GGMLTypeQ3_K:       {BlockSize: 256, TypeSize: 110, Quantized: true},
	GGMLTypeQ4_K:       {BlockSize: 256, TypeSize: 144, Quantized: true},
	GGMLTypeQ5_K:       {BlockSize: 256, TypeSize: 176, Quantized: true},
	GGMLTypeQ6_K:       {BlockSize: 256, TypeSize: 210, Quantized: true},
	GGMLTypeQ8_K:       {BlockSize: 256, TypeSize: 292, Quantized: true},
	GGMLTypeIQ2_XXS:    {BlockSize: 256, TypeSize: 66, Quantized: true},
	GGMLTypeIQ2_XS:     {BlockSize: 256, TypeSize: 74, Quantized: true},
	GGMLTypeIQ3_XXS:    {BlockSize: 256, TypeSize: 98, Quantized: true},
	GGMLTypeIQ1_S:      {BlockSize: 256, TypeSize: 50, Quantized: true},
	GGMLTypeIQ4_NL:     {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeIQ3_S:      {BlockSize: 256, TypeSize: 110, Quantized: true},
	GGMLTypeIQ2_S:      {BlockSize: 256, TypeSize: 82, Quantized: true},
	GGMLTypeIQ4_XS:     {BlockSize: 256, TypeSize: 136, Quantized: true},
	GGMLTypeI8:         {BlockSize: 1, TypeSize: 1},
	GGMLTypeI16:        {BlockSize: 1, TypeSize: 2},
	GGMLTypeI32:        {BlockSize: 1, TypeSize: 4},
	GGMLTypeI64:        {BlockSize: 1, TypeSize: 8},
	GGMLTypeF64:        {BlockSize: 1, TypeSize: 8},
	GGMLTypeIQ1_M:      {BlockSize: 256, TypeSize: 56, Quantized: true},
	GGMLTypeBF16:       {BlockSize: 1, TypeSize: 2},
	GGMLTypeQ4_0_4_4:   {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeQ4_0_4_8:   {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeQ4_0_8_8:   {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeTQ1_0:      {BlockSize: 256, TypeSize: 54, Quantized: true},
	GGMLTypeTQ2_0:      {BlockSize: 256, TypeSize: 66, Quantized: true},
	GGMLTypeIQ4_NL_4_4: {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeIQ4_NL_4_8: {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeIQ4_NL_8_8: {BlockSize: 32, TypeSize: 18, Quantized: true},
	GGMLTypeMXFP4:      {BlockSize: 32, TypeSize: 17, Quantized: true},
}

// Trait returns the GGMLTypeTrait of the GGMLType.
func (t GGMLType) Trait() (GGMLTypeTrait, bool) {
	tt, ok := _GGMLTypeTraits[t]
	return tt, ok
}

// IsQuantized returns whether the GGMLType is quantized.
func (t GGMLType) IsQuantized() bool {
	tt, ok := t.Trait()
	if !ok {
		return false
	}
	return tt.Quantized
}

// RowSizeOf returns the size of the given dimensions according to the GGMLType's GGMLTypeTrait,
// which is inspired by
// https://github.com/ggerganov/ggml/blob/0cbb7c0e053f5419cfbebb46fbf4d4ed60182cf5/src/ggml.c#L3142-L3145.
//
// The index of the given dimensions means the number of dimension,
// i.e. 0 is the first dimension, 1 is the second dimension, and so on.
//
// The value of the item is the number of elements in the corresponding dimension.
func (t GGMLType) RowSizeOf(dimensions []uint64) uint64 {
	if len(dimensions) == 0 {
		panic(errors.New("no dimensions"))
	}

	tt, ok := t.Trait()
	if !ok {
		panic(fmt.Errorf("invalid type: %v", t))
	}

	// https://github.com/ggerganov/ggml/blob/a10a8b880c059b3b29356eb9a9f8df72f03cdb6a/src/ggml.c#L2640-L2643
	ds := tt.TypeSize * dimensions[0] / tt.BlockSize // Row size
	for i := 1; i < len(dimensions); i++ {
		ds *= dimensions[i]
	}
	return ds
}

// GGMLMemoryPadding returns the padded size of the given size according to GGML memory padding,
// see https://github.com/ggerganov/ggml/blob/0cbb7c0/include/ggml/ggml.h#L238-L243.
func GGMLMemoryPadding(size uint64) uint64 {
	const align = 16
	return GGMLPadding(size, align)
}

// GGMLPadding returns the padded size of the given size according to given align,
// see https://github.com/ggerganov/ggml/blob/0cbb7c0e053f5419cfbebb46fbf4d4ed60182cf5/include/ggml/ggml.h#L255.
func GGMLPadding(size, align uint64) uint64 {
	return (size + align - 1) &^ (align - 1)
}

// GGML tensor constants.
const (
	// GGMLTensorSize is the size of GGML tensor in bytes,
	// see https://github.com/ggerganov/ggml/blob/0cbb7c0e053f5419cfbebb46fbf4d4ed60182cf5/include/ggml/ggml.h#L606.
	GGMLTensorSize = 368

	// GGMLObjectSize is the size of GGML object in bytes,
	// see https://github.com/ggerganov/ggml/blob/0cbb7c0e053f5419cfbebb46fbf4d4ed60182cf5/include/ggml/ggml.h#L563.
	GGMLObjectSize = 32
)

// GGMLTensorOverhead is the overhead of GGML tensor in bytes,
// see https://github.com/ggerganov/ggml/blob/0cbb7c0e053f5419cfbebb46fbf4d4ed60182cf5/src/ggml.c#L2765-L2767.
func GGMLTensorOverhead() uint64 {
	return GGMLObjectSize + GGMLTensorSize
}

// GGML computation graph constants.
const (
	// GGMLComputationGraphSize is the size of GGML computation graph in bytes.
	GGMLComputationGraphSize = 80

	// GGMLComputationBitsetSize is the size of GGML computation bitset in bytes,
	// see https://github.com/ggml-org/llama.cpp/blob/master/ggml/src/ggml-impl.h#L165.
	GGMLComputationBitsetSize = 4
)

// GGMLComputationGraphOverhead is the overhead of GGML graph in bytes,
// see https://github.com/ggml-org/ggml/blob/5592ffda9c417c3c12232c828247c23d17004c88/src/ggml.c#L5941-L5956.
func GGMLComputationGraphOverhead(nodes uint64, grads bool) uint64 {
	const ps = 8 // c++ pointer size

	hs := GGMLHashSize(nodes * 2)

	var g uint64 = GGMLComputationGraphSize // graph
	g += GGMLPadding(nodes*ps, ps)          // nodes
	g += GGMLPadding(nodes*ps, ps)          // leafs
	g += GGMLPadding(nodes*ps, ps)          // parents
	g += GGMLPadding(hs*ps, ps)             // hash keys
	if grads {
		g += GGMLPadding(hs*ps, ps) // grads
		g += GGMLPadding(hs*ps, ps) // grad_accs
	}
	g += GGMLPadding(GGMLBitsetSize(hs)*GGMLComputationBitsetSize, GGMLComputationBitsetSize) // bitset

	return GGMLObjectSize + GGMLMemoryPadding(g)
}

// GGMLHashSize returns the size of the hash table for the given base,
// see https://github.com/ggerganov/ggml/blob/0cbb7c0e053f5419cfbebb46fbf4d4ed60182cf5/src/ggml.c#L17698-L17722.
func GGMLHashSize(base uint64) uint64 {
	primes := []uint64{
		2, 3, 5, 11, 17, 37, 67, 131, 257, 521, 1031,
		2053, 4099, 8209, 16411, 32771, 65537, 131101,
		262147, 524309, 1048583, 2097169, 4194319, 8388617,
		16777259, 33554467, 67108879, 134217757, 268435459,
		536870923, 1073741827, 2147483659,
	}
	i, ok := slices.BinarySearchFunc(primes, base, func(e, t uint64) int {
		if t >= e {
			return 0
		}
		return -1
	})
	if !ok {
		return base | 1
	}
	return primes[i]
}

// GGMLBitsetSize returns the size of the bitset for the given number of bits,
// see https://github.com/ggml-org/llama.cpp/blob/ec9e0301fef6476df83e94842c3b625501c95566/ggml/src/ggml-impl.h#L166-L171.
func GGMLBitsetSize(n uint64) uint64 {
	return (n + (GGMLComputationBitsetSize*8 - 1)) >> 5
}
