package gguf_parser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"regexp"
	"strings"

	"golang.org/x/exp/constraints"

	"github.com/gpustack/gguf-parser-go/util/anyx"
	"github.com/gpustack/gguf-parser-go/util/bytex"
	"github.com/gpustack/gguf-parser-go/util/funcx"
	"github.com/gpustack/gguf-parser-go/util/osx"
	"github.com/gpustack/gguf-parser-go/util/stringx"
)

// GGUFFile represents a GGUF file,
// see https://github.com/ggerganov/ggml/blob/master/docs/gguf.md#file-structure.
//
// Compared with the complete GGUF file,
// this structure lacks the tensor data part.
type GGUFFile struct {
	/* Basic */

	// Header is the header of the GGUF file.
	Header GGUFHeader `json:"header"`
	// TensorInfos are the tensor infos of the GGUF file,
	// the size of TensorInfos is equal to `Header.TensorCount`.
	TensorInfos GGUFTensorInfos `json:"tensorInfos"`
	// Padding is the padding size of the GGUF file,
	// which is used to split Header and TensorInfos from tensor data.
	Padding int64 `json:"padding"`
	// SplitPaddings holds the padding size slice of the GGUF file splits,
	// each item represents splitting Header and TensorInfos from tensor data.
	//
	// The length of SplitPaddings is the number of split files.
	SplitPaddings []int64 `json:"splitPaddings,omitempty"`
	// TensorDataStartOffset is the offset in bytes of the tensor data in this file.
	//
	// The offset is the start of the file.
	TensorDataStartOffset int64 `json:"tensorDataStartOffset"`
	// SplitTensorDataStartOffsets holds the offset slice in bytes of the tensor data of the GGUF file splits,
	// each item represents the offset of the tensor data in the split file.
	//
	// The length of SplitTensorDataStartOffsets is the number of split files.
	SplitTensorDataStartOffsets []int64 `json:"splitTensorDataStartOffsets,omitempty"`

	/* Appendix */

	// Size is the size of the GGUF file,
	// if the file is split, the size is the sum of all split files.
	Size GGUFBytesScalar `json:"size"`
	// SplitSizes holds the size slice of the GGUF file splits,
	// each item represents the size of the split file.
	//
	// The length of SplitSizes is the number of split files.
	SplitSizes []GGUFBytesScalar `json:"splitSizes,omitempty"`
	// ModelSize is the size of the model when loading.
	ModelSize GGUFBytesScalar `json:"modelSize"`
	// SplitModelSizes holds the size slice of the model,
	// each item represents a size when loading of the split file.
	//
	// The length of SplitModelSizes is the number of split files.
	SplitModelSizes []GGUFBytesScalar `json:"splitModelSizes,omitempty"`
	// ModelParameters is the number of the model parameters.
	ModelParameters GGUFParametersScalar `json:"modelParameters"`
	// ModelBitsPerWeight is the bits per weight of the model,
	// which describes how many bits are used to store a weight,
	// higher is better.
	ModelBitsPerWeight GGUFBitsPerWeightScalar `json:"modelBitsPerWeight"`
}

// GGUFMagic is a magic number of GGUF file,
// see https://github.com/ggerganov/ggml/blob/master/docs/gguf.md#historical-state-of-affairs.
type GGUFMagic uint32

// GGUFMagic constants.
const (
	GGUFMagicGGML   GGUFMagic = 0x67676d6c
	GGUFMagicGGMF   GGUFMagic = 0x67676d66
	GGUFMagicGGJT   GGUFMagic = 0x67676a74
	GGUFMagicGGUFLe GGUFMagic = 0x46554747 // GGUF
	GGUFMagicGGUFBe GGUFMagic = 0x47475546 // GGUF
)

// GGUFVersion is a version of GGUF file format,
// see https://github.com/ggerganov/ggml/blob/master/docs/gguf.md#version-history.
type GGUFVersion uint32

// GGUFVersion constants.
const (
	GGUFVersionV1 GGUFVersion = iota + 1
	GGUFVersionV2
	GGUFVersionV3
)

// GGUFHeader represents the header of a GGUF file.
type GGUFHeader struct {
	// Magic is a magic number that announces that this is a GGUF file.
	Magic GGUFMagic `json:"magic"`
	// Version is a version of the GGUF file format.
	Version GGUFVersion `json:"version"`
	// TensorCount is the number of tensors in the file.
	TensorCount uint64 `json:"tensorCount"`
	// MetadataKVCount is the number of key-value pairs in the metadata.
	MetadataKVCount uint64 `json:"metadataKVCount"`
	// MetadataKV are the key-value pairs in the metadata,
	MetadataKV GGUFMetadataKVs `json:"metadataKV"`
}

// GGUFMetadataValueType is a type of GGUF metadata value,
// see https://github.com/ggerganov/ggml/blob/master/docs/gguf.md#file-structure.
type GGUFMetadataValueType uint32

// GGUFMetadataValueType constants.
const (
	GGUFMetadataValueTypeUint8 GGUFMetadataValueType = iota
	GGUFMetadataValueTypeInt8
	GGUFMetadataValueTypeUint16
	GGUFMetadataValueTypeInt16
	GGUFMetadataValueTypeUint32
	GGUFMetadataValueTypeInt32
	GGUFMetadataValueTypeFloat32
	GGUFMetadataValueTypeBool
	GGUFMetadataValueTypeString
	GGUFMetadataValueTypeArray
	GGUFMetadataValueTypeUint64
	GGUFMetadataValueTypeInt64
	GGUFMetadataValueTypeFloat64
	_GGUFMetadataValueTypeCount // Unknown
)

// Types for GGUFMetadataKV.
type (
	// GGUFMetadataKV is a key-value pair in the metadata of a GGUF file.
	GGUFMetadataKV struct {
		// Key is the key of the metadata key-value pair,
		// which is no larger than 64 bytes long.
		Key string `json:"key"`
		// ValueType is the type of the metadata value.
		ValueType GGUFMetadataValueType `json:"valueType"`
		// Value is the value of the metadata key-value pair.
		Value any `json:"value"`
	}

	// GGUFMetadataKVArrayValue is a value of a GGUFMetadataKV with type GGUFMetadataValueTypeArray.
	GGUFMetadataKVArrayValue struct {
		/* Basic */

		// Type is the type of the array item.
		Type GGUFMetadataValueType `json:"type"`
		// Len is the length of the array.
		Len uint64 `json:"len"`
		// Array holds all array items.
		Array []any `json:"array,omitempty"`

		/* Appendix */

		// StartOffset is the offset in bytes of the GGUFMetadataKVArrayValue in the GGUFFile file.
		//
		// The offset is the start of the file.
		StartOffset int64 `json:"startOffset"`

		// Size is the size of the array in bytes.
		Size int64 `json:"size"`
	}

	// GGUFMetadataKVs is a list of GGUFMetadataKV.
	GGUFMetadataKVs []GGUFMetadataKV
)

// Types for GGUFTensorInfo.
type (
	// GGUFTensorInfo represents a tensor info in a GGUF file.
	GGUFTensorInfo struct {
		/* Basic */

		// Name is the name of the tensor,
		// which is no larger than 64 bytes long.
		Name string `json:"name"`
		// NDimensions is the number of dimensions of the tensor.
		NDimensions uint32 `json:"nDimensions"`
		// Dimensions is the dimensions of the tensor,
		// the length is NDimensions.
		Dimensions []uint64 `json:"dimensions"`
		// Type is the type of the tensor.
		Type GGMLType `json:"type"`
		// Offset is the offset in bytes of the tensor's data in this file.
		//
		// The offset is relative to tensor data, not to the start of the file.
		Offset uint64 `json:"offset"`

		/* Appendix */

		// StartOffset is the offset in bytes of the GGUFTensorInfo in the GGUFFile file.
		//
		// The offset is the start of the file.
		StartOffset int64 `json:"startOffset"`
	}

	// GGUFTensorInfos is a list of GGUFTensorInfo.
	GGUFTensorInfos []GGUFTensorInfo
)

var ErrGGUFFileInvalidFormat = errors.New("invalid GGUF format")

// GGMLMaxDims is the maximum number of tensor dimensions supported by GGML,
// mirroring GGML_MAX_DIMS in ggml.h. A NDimensions value above this bound
// is rejected to avoid pathological allocations and to keep tensor size math
// bounded — see CWE-190 hardening (analog of llama.cpp GHSA-vgg9-87g3-85w8).
const GGMLMaxDims uint32 = 4

// mulU64 returns a*b and panics on uint64 overflow. Used in tensor element
// and byte size math where a wrapped result would silently understate memory
// usage and mislead downstream sizing (e.g. GPU layer allocation). Panic is
// chosen over a return error to keep the public Elements/Bytes signatures
// intact; a crafted GGUF that hits this is already malformed.
func mulU64(a, b uint64, what string) uint64 {
	hi, lo := bits.Mul64(a, b)
	if hi != 0 {
		panic(fmt.Errorf("gguf: uint64 overflow in %s: %d * %d", what, a, b))
	}
	return lo
}

// addU64 returns a+b and panics on uint64 overflow. Paired with mulU64 in
// Bytes() so that the accumulated stride sum cannot wrap either.
func addU64(a, b uint64, what string) uint64 {
	s := a + b
	if s < a {
		panic(fmt.Errorf("gguf: uint64 overflow in %s: %d + %d", what, a, b))
	}
	return s
}

// safeSeekDelta computes count*size and converts to a positive int64 suitable
// for io.SeekCurrent. Returns an error rather than panicking because the
// inputs come straight from untrusted file bytes (string lengths, array
// lengths); a crafted GGUF should surface as a parse error, not a crash.
// Without this guard, int64(uint64) silently wraps to a negative offset when
// the value exceeds math.MaxInt64, letting an attacker seek backwards.
func safeSeekDelta(count, size uint64, what string) (int64, error) {
	hi, lo := bits.Mul64(count, size)
	if hi != 0 || lo > math.MaxInt64 {
		return 0, fmt.Errorf("seek delta overflow in %s: %d * %d", what, count, size)
	}
	return int64(lo), nil
}

// ParseGGUFFile parses a GGUF file from the local given path,
// and returns the GGUFFile, or an error if any.
func ParseGGUFFile(path string, opts ...GGUFReadOption) (*GGUFFile, error) {
	var o _GGUFReadOptions
	for _, opt := range opts {
		opt(&o)
	}

	var paths []string
	{
		rs := CompleteShardGGUFFilename(path)
		if rs != nil {
			paths = rs
		} else {
			paths = []string{path}
		}
	}

	fs := make([]_GGUFFileReadSeeker, 0, len(paths))
	defer func() {
		for i := range fs {
			osx.Close(fs[i])
		}
	}()

	for i := range paths {
		if o.MMap {
			mf, err := osx.OpenMmapFile(paths[i])
			if err != nil {
				return nil, fmt.Errorf("open mmap file: %w", err)
			}

			fs = append(fs, _GGUFFileReadSeeker{
				Closer:     mf,
				ReadSeeker: io.NewSectionReader(mf, 0, mf.Len()),
				Size:       mf.Len(),
			})

			continue
		}

		ff, err := osx.Open(paths[i])
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}

		fs = append(fs, _GGUFFileReadSeeker{
			Closer:     ff,
			ReadSeeker: ff,
			Size:       funcx.MustNoError(ff.Stat()).Size(),
		})
	}

	return parseGGUFFile(fs, o)
}

type _GGUFFileReadSeeker struct {
	io.Closer
	io.ReadSeeker
	Size int64
}

func _validateCountWithRemaining(f _GGUFFileReadSeeker, count uint64, version GGUFVersion, what string) error {
	if count == 0 {
		return nil
	}
	var minItemSize int64

	switch strings.ToLower(what) {
	case "metadatakvcount":
		if version <= GGUFVersionV1 {
			minItemSize = 12 // key length (uint32) + value type (uint32) + min value (string length uint32)
		} else {
			minItemSize = 20 // key length (uint64) + value type (uint32) + min value (string length uint64)
		}
	case "tensor":
		if version <= GGUFVersionV1 {
			minItemSize = 20 // name length (uint32) + n_dims (uint32) + type (uint32) + offset (uint64)
		} else {
			minItemSize = 24 // name length (uint64) + n_dims (uint32) + type (uint32) + offset (uint64)
		}
	}

	if minItemSize <= 0 {
		return fmt.Errorf("invalid min item size for %s: %d", what, minItemSize)
	}
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("seek %s count position: %w", what, err)
	}
	remaining := f.Size - pos
	if remaining < 0 {
		return fmt.Errorf("invalid file size: %d", f.Size)
	}
	maxCount := uint64(remaining / minItemSize)
	if maxCount < count {
		return fmt.Errorf("%s count too large for remaining bytes: %d", what, count)
	}

	return nil
}

func parseGGUFFile(fs []_GGUFFileReadSeeker, o _GGUFReadOptions) (_ *GGUFFile, err error) {
	var gf GGUFFile

	for _, f := range fs {
		var bo binary.ByteOrder = binary.LittleEndian

		// magic
		var magic GGUFMagic
		if err = binary.Read(f, bo, &magic); err != nil {
			return nil, fmt.Errorf("read magic: %w", err)
		}
		switch magic {
		default:
			return nil, ErrGGUFFileInvalidFormat
		case GGUFMagicGGML, GGUFMagicGGMF, GGUFMagicGGJT:
			return nil, fmt.Errorf("unsupported format: %s", magic)
		case GGUFMagicGGUFLe:
		case GGUFMagicGGUFBe:
			bo = binary.BigEndian
		}
		gf.Header.Magic = magic

		// version
		var version GGUFVersion
		if err = binary.Read(f, bo, &version); err != nil {
			return nil, fmt.Errorf("read version: %w", err)
		}
		if version > GGUFVersionV3 {
			return nil, fmt.Errorf("unsupported GGUF version: %d (supported: %d-%d)",
				version, GGUFVersionV1, GGUFVersionV3)
		}
		gf.Header.Version = version

		rd := _GGUFReader{v: version, o: o, f: f, bo: bo}

		// tensor count
		var tensorCount uint64
		if version <= GGUFVersionV1 {
			tensorCount, err = rd.ReadUint64FromUint32()
		} else {
			tensorCount, err = rd.ReadUint64()
		}
		if err != nil {
			return nil, fmt.Errorf("read tensor count: %w", err)
		}
		if err := _validateCountWithRemaining(f, tensorCount, version, "tensor"); err != nil {
			return nil, err
		}
		gf.Header.TensorCount += tensorCount

		// metadata kv count
		var metadataKVCount uint64
		if version <= GGUFVersionV1 {
			metadataKVCount, err = rd.ReadUint64FromUint32()
		} else {
			metadataKVCount, err = rd.ReadUint64()
		}
		if err != nil {
			return nil, fmt.Errorf("read metadata kv count: %w", err)
		}
		if err := _validateCountWithRemaining(f, metadataKVCount, version, "metadatakvcount"); err != nil {
			return nil, err
		}
		gf.Header.MetadataKVCount += metadataKVCount

		// metadata kv
		{
			rd := _GGUFMetadataReader{_GGUFReader: rd}
			kvs := make(GGUFMetadataKVs, metadataKVCount)
			for i := uint64(0); i < metadataKVCount; i++ {
				kvs[i], err = rd.Read()
				if err != nil {
					return nil, fmt.Errorf("read metadata kv %d: %w", i, err)
				}
			}
			for i := range kvs {
				if kvs[i].Key == "split.no" {
					gf.Header.MetadataKVCount--
					continue
				}
				gf.Header.MetadataKV = append(gf.Header.MetadataKV, kvs[i])
			}
		}

		// tensor infos
		if gf.TensorInfos == nil {
			tc, ok := gf.Header.MetadataKV.Get("split.tensors.count")
			if ok {
				gf.TensorInfos = make(GGUFTensorInfos, 0, anyx.Number[int](tc.Value))
			} else {
				// avoid preallocating with tensorCount (could be huge); start empty and append
				gf.TensorInfos = make(GGUFTensorInfos, 0)
			}
		}
		{
			rd := _GGUFTensorInfoReader{_GGUFReader: rd}
			tis := make(GGUFTensorInfos, 0)
			for i := uint64(0); i < tensorCount; i++ {
				ti, err := rd.Read()
				if err != nil {
					return nil, fmt.Errorf("read tensor info %d: %w", i, err)
				}
				tis = append(tis, ti)
			}
			gf.TensorInfos = append(gf.TensorInfos, tis...)
		}

		pds, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("seek padding start: %w", err)
		}

		// padding
		var padding int64
		{
			// The global alignment to use, as described above.
			// This can vary to allow for different alignment schemes, but it must be a multiple of 8.
			// Some writers may not write the alignment.
			// If the alignment is not specified, assume it is 32.
			var ag uint32 = 32
			if v, ok := gf.Header.MetadataKV.Get("general.alignment"); ok {
				ag = v.ValueUint32()
			}
			padding = int64(ag) - (pds % int64(ag))
		}
		if len(fs) == 1 {
			gf.Padding = padding
		}
		gf.SplitPaddings = append(gf.SplitPaddings, padding)

		// tensor data offset
		tensorDataStartOffset := pds + padding
		if len(fs) == 1 {
			gf.TensorDataStartOffset = tensorDataStartOffset
		}
		gf.SplitTensorDataStartOffsets = append(gf.SplitTensorDataStartOffsets, tensorDataStartOffset)

		// size
		size := GGUFBytesScalar(f.Size)
		gf.Size += size
		gf.SplitSizes = append(gf.SplitSizes, size)

		// model size
		modelSize := GGUFBytesScalar(f.Size - tensorDataStartOffset)
		gf.ModelSize += modelSize
		gf.SplitModelSizes = append(gf.SplitModelSizes, modelSize)
	}

	// model parameters
	gf.ModelParameters = GGUFParametersScalar(gf.TensorInfos.Elements())

	// bpw
	if gf.ModelParameters != 0 {
		gf.ModelBitsPerWeight = GGUFBitsPerWeightScalar(float64(gf.ModelSize) * 8 / float64(gf.ModelParameters))
	}

	return &gf, nil
}

// Types for GGUF hierarchical tensors.
type (
	// GGUFTensorInfoFilter is a filter to filter out if the given tensor name matches.
	// Return true if the name matches, and false otherwise.
	GGUFTensorInfoFilter func(name string) bool

	// IGGUFTensorInfos is an interface for GGUF tensor infos,
	// which includes basic operations.
	IGGUFTensorInfos interface {
		// Get returns the GGUFTensorInfo with the given name,
		// and true if found, and false otherwise.
		Get(name string) (info GGUFTensorInfo, found bool)
		// GetFileType returns the GGUFFileType.
		GetFileType() GGUFFileType
		// Match returns true if the name matches the given regex, and false otherwise.
		Match(nameRegex *regexp.Regexp) bool
		// Search returns a list of GGUFTensorInfo with the names that match the given regex.
		Search(nameRegex *regexp.Regexp) (infos []GGUFTensorInfo)
		// Index returns a map value to the GGUFTensorInfo with the given names,
		// and the number of names found.
		Index(names []string) (infos map[string]GGUFTensorInfo, found int)
		// Elements returns the number of elements(parameters).
		Elements(filter ...GGUFTensorInfoFilter) uint64
		// Bytes returns the number of bytes.
		Bytes(filter ...GGUFTensorInfoFilter) uint64
		// Count returns the number of tensors.
		Count() uint64
	}

	// GGUFLayerTensorInfos represents hierarchical tensor infos of a GGUF file,
	// it can save GGUFNamedTensorInfos, GGUFTensorInfos, and GGUFTensorInfo.
	GGUFLayerTensorInfos []IGGUFTensorInfos

	// GGUFNamedTensorInfos is the namespace for relevant tensors,
	// which must has a name.
	GGUFNamedTensorInfos struct {
		// Name is the name of the namespace.
		Name string `json:"name"`
		// GGUFLayerTensorInfos can save GGUFNamedTensorInfos, GGUFTensorInfos, or GGUFTensorInfo.
		//
		// If the item is type of GGUFTensorInfo, it must be the leaf node.
		//
		// Any branch nodes are type of GGUFNamedTensorInfos or GGUFTensorInfos,
		// which can be nested.
		//
		// Branch nodes store in type pointer.
		GGUFLayerTensorInfos `json:"items,omitempty"`
	}
)

// Layers converts the GGUFTensorInfos to GGUFLayerTensorInfos.
func (gf *GGUFFile) Layers(ignores ...string) GGUFLayerTensorInfos {
	return gf.TensorInfos.Layers(ignores...)
}

func (kv GGUFMetadataKV) ValueUint8() uint8 {
	if kv.ValueType != GGUFMetadataValueTypeUint8 {
		panic(fmt.Errorf("key %q try to get type Uint8 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[uint8](kv.Value)
}

func (kv GGUFMetadataKV) ValueInt8() int8 {
	if kv.ValueType != GGUFMetadataValueTypeInt8 {
		panic(fmt.Errorf("key %q try to get type Int8 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[int8](kv.Value)
}

func (kv GGUFMetadataKV) ValueUint16() uint16 {
	if kv.ValueType != GGUFMetadataValueTypeUint16 {
		panic(fmt.Errorf("key %q try to get type Uint16 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[uint16](kv.Value)
}

func (kv GGUFMetadataKV) ValueInt16() int16 {
	if kv.ValueType != GGUFMetadataValueTypeInt16 {
		panic(fmt.Errorf("key %q try to get type Int16 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[int16](kv.Value)
}

func (kv GGUFMetadataKV) ValueUint32() uint32 {
	if kv.ValueType != GGUFMetadataValueTypeUint32 {
		panic(fmt.Errorf("key %q try to get type Uint32 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[uint32](kv.Value)
}

func (kv GGUFMetadataKV) ValueInt32() int32 {
	if kv.ValueType != GGUFMetadataValueTypeInt32 {
		panic(fmt.Errorf("key %q try to get type Int32 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[int32](kv.Value)
}

func (kv GGUFMetadataKV) ValueFloat32() float32 {
	if kv.ValueType != GGUFMetadataValueTypeFloat32 {
		panic(fmt.Errorf("key %q try to get type Float32 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[float32](kv.Value)
}

func (kv GGUFMetadataKV) ValueBool() bool {
	if kv.ValueType != GGUFMetadataValueTypeBool {
		panic(fmt.Errorf("key %q try to get type Bool but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Bool(kv.Value)
}

func (kv GGUFMetadataKV) ValueString() string {
	if kv.ValueType != GGUFMetadataValueTypeString {
		panic(fmt.Errorf("key %q try to get type String but type %v", kv.Key, kv.ValueType))
	}
	return anyx.String(kv.Value)
}

func (kv GGUFMetadataKV) ValueArray() GGUFMetadataKVArrayValue {
	if kv.ValueType != GGUFMetadataValueTypeArray {
		panic(fmt.Errorf("key %q try to get type Array but type %v", kv.Key, kv.ValueType))
	}
	switch t := kv.Value.(type) {
	case GGUFMetadataKVArrayValue:
		return t
	case map[string]any:
		return GGUFMetadataKVArrayValue{
			Type: anyx.Number[GGUFMetadataValueType](t["type"]),
			Len:  anyx.Number[uint64](t["len"]),
			Array: func() []any {
				if vv, ok := t["array"].([]any); ok {
					return vv
				}
				return nil
			}(),
			StartOffset: anyx.Number[int64](t["startOffset"]),
			Size:        anyx.Number[int64](t["size"]),
		}
	default:
		panic(fmt.Errorf("key %q try to get type Array but type %T", kv.Key, kv.Value))
	}
}

func (kv GGUFMetadataKV) ValueUint64() uint64 {
	if kv.ValueType != GGUFMetadataValueTypeUint64 {
		panic(fmt.Errorf("key %q try to get type Uint64 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[uint64](kv.Value)
}

func (kv GGUFMetadataKV) ValueInt64() int64 {
	if kv.ValueType != GGUFMetadataValueTypeInt64 {
		panic(fmt.Errorf("key %q try to get type Int64 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[int64](kv.Value)
}

func (kv GGUFMetadataKV) ValueFloat64() float64 {
	if kv.ValueType != GGUFMetadataValueTypeFloat64 {
		panic(fmt.Errorf("key %q try to get type Float64 but type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[float64](kv.Value)
}

// ValueNumeric returns the numeric values of the GGUFMetadataKV,
// and panics if the value type is not numeric.
//
// ValueNumeric is a generic function, and the type T must be constraints.Integer or constraints.Float.
//
// Compare to the GGUFMetadataKV's Value* functions,
// ValueNumeric will cast the original value to the target type.
func ValueNumeric[T constraints.Integer | constraints.Float](kv GGUFMetadataKV) T {
	switch kv.ValueType {
	case GGUFMetadataValueTypeUint8:
	case GGUFMetadataValueTypeInt8:
	case GGUFMetadataValueTypeUint16:
	case GGUFMetadataValueTypeInt16:
	case GGUFMetadataValueTypeUint32:
	case GGUFMetadataValueTypeInt32:
	case GGUFMetadataValueTypeFloat32:
	case GGUFMetadataValueTypeUint64:
	case GGUFMetadataValueTypeInt64:
	case GGUFMetadataValueTypeFloat64:
	default:
		panic(fmt.Errorf("key %q try to get type Numeric but got type %v", kv.Key, kv.ValueType))
	}
	return anyx.Number[T](kv.Value)
}

func (av GGUFMetadataKVArrayValue) ValuesUint8() []uint8 {
	if av.Type != GGUFMetadataValueTypeUint8 {
		panic(fmt.Errorf("try to get type Uint8 but got type %v", av.Type))
	}
	v := make([]uint8, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[uint8](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesInt8() []int8 {
	if av.Type != GGUFMetadataValueTypeInt8 {
		panic(fmt.Errorf("try to get type Int8 but got type %v", av.Type))
	}
	v := make([]int8, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[int8](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesUint16() []uint16 {
	if av.Type != GGUFMetadataValueTypeUint16 {
		panic(fmt.Errorf("try to get type Uint16 but got type %v", av.Type))
	}
	v := make([]uint16, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[uint16](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesInt16() []int16 {
	if av.Type != GGUFMetadataValueTypeInt16 {
		panic(fmt.Errorf("try to get type Int16 but got type %v", av.Type))
	}
	v := make([]int16, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[int16](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesUint32() []uint32 {
	if av.Type != GGUFMetadataValueTypeUint32 {
		panic(fmt.Errorf("try to get type Uint8 but got type %v", av.Type))
	}
	v := make([]uint32, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[uint32](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesInt32() []int32 {
	if av.Type != GGUFMetadataValueTypeInt32 {
		panic(fmt.Errorf("try to get type Int32 but got type %v", av.Type))
	}
	v := make([]int32, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[int32](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesFloat32() []float32 {
	if av.Type != GGUFMetadataValueTypeFloat32 {
		panic(fmt.Errorf("try to get type Float32 but got type %v", av.Type))
	}
	v := make([]float32, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[float32](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesBool() []bool {
	if av.Type != GGUFMetadataValueTypeBool {
		panic(fmt.Errorf("try to get type Bool but got type %v", av.Type))
	}
	v := make([]bool, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Bool(av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesString() []string {
	if av.Type != GGUFMetadataValueTypeString {
		panic(fmt.Errorf("try to get type String but got type %v", av.Type))
	}
	v := make([]string, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.String(av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesArray() []GGUFMetadataKVArrayValue {
	if av.Type != GGUFMetadataValueTypeArray {
		panic(fmt.Errorf("try to get type Array but got type %v", av.Type))
	}
	v := make([]GGUFMetadataKVArrayValue, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		switch t := av.Array[i].(type) {
		case GGUFMetadataKVArrayValue:
			v[i] = t
		case map[string]any:
			v[i] = GGUFMetadataKVArrayValue{
				Type: anyx.Number[GGUFMetadataValueType](t["type"]),
				Len:  anyx.Number[uint64](t["len"]),
				Array: func() []any {
					if vv, ok := t["array"].([]any); ok {
						return vv
					}
					return nil
				}(),
				StartOffset: anyx.Number[int64](t["startOffset"]),
				Size:        anyx.Number[int64](t["size"]),
			}
		default:
			panic(fmt.Errorf("try to get type Array but got type %T", av.Array[i]))
		}
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesUint64() []uint64 {
	if av.Type != GGUFMetadataValueTypeUint64 {
		panic(fmt.Errorf("try to get type Uint16 but got type %v", av.Type))
	}
	v := make([]uint64, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[uint64](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesInt64() []int64 {
	if av.Type != GGUFMetadataValueTypeInt64 {
		panic(fmt.Errorf("try to get type Int64 but got type %v", av.Type))
	}
	v := make([]int64, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[int64](av.Array[i])
	}
	return v
}

func (av GGUFMetadataKVArrayValue) ValuesFloat64() []float64 {
	if av.Type != GGUFMetadataValueTypeFloat64 {
		panic(fmt.Errorf("try to get type Float64 but got type %v", av.Type))
	}
	v := make([]float64, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		v[i] = anyx.Number[float64](av.Array[i])
	}
	return v
}

// ValuesNumeric returns the numeric values of the GGUFMetadataKVArrayValue,
// and panics if the value type is not numeric.
//
// ValuesNumeric is a generic function, and the type T must be constraints.Integer or constraints.Float.
//
// Compare to the GGUFMetadataKVArrayValue's Value* functions,
// ValuesNumeric will cast the original value to the target type.
func ValuesNumeric[T constraints.Integer | constraints.Float](av GGUFMetadataKVArrayValue) []T {
	v := make([]T, av.Len)
	for i := uint64(0); i < av.Len; i++ {
		switch av.Type {
		case GGUFMetadataValueTypeUint8:
		case GGUFMetadataValueTypeInt8:
		case GGUFMetadataValueTypeUint16:
		case GGUFMetadataValueTypeInt16:
		case GGUFMetadataValueTypeUint32:
		case GGUFMetadataValueTypeInt32:
		case GGUFMetadataValueTypeFloat32:
		case GGUFMetadataValueTypeUint64:
		case GGUFMetadataValueTypeInt64:
		case GGUFMetadataValueTypeFloat64:
		default:
			panic(fmt.Errorf("try to get type Numeric but got type %v", av.Type))
		}
		if av.Array != nil {
			v[i] = anyx.Number[T](av.Array[i])
		}
	}
	return v
}

// Get returns the GGUFMetadataKV with the given key,
// and true if found, and false otherwise.
func (kvs GGUFMetadataKVs) Get(key string) (value GGUFMetadataKV, found bool) {
	for i := range kvs {
		if kvs[i].Key == key {
			return kvs[i], true
		}
	}
	return GGUFMetadataKV{}, false
}

// Search returns a list of GGUFMetadataKV with the keys that match the given regex.
func (kvs GGUFMetadataKVs) Search(keyRegex *regexp.Regexp) (values []GGUFMetadataKV) {
	for i := range kvs {
		if keyRegex.MatchString(kvs[i].Key) {
			values = append(values, kvs[i])
		}
	}
	return values
}

// Index returns a map value to the GGUFMetadataKVs with the given keys,
// and the number of keys found.
func (kvs GGUFMetadataKVs) Index(keys []string) (values map[string]GGUFMetadataKV, found int) {
	ks := make(map[string]struct{}, len(keys))
	for i := range keys {
		ks[keys[i]] = struct{}{}
	}
	values = make(map[string]GGUFMetadataKV)
	for i := range kvs {
		if _, ok := ks[kvs[i].Key]; ok {
			values[kvs[i].Key] = kvs[i]
			found++
		}
		if found == len(ks) {
			break
		}
	}
	return values, found
}

// Get returns the GGUFTensorInfo with the given name,
// and true if found, and false otherwise.
func (ti GGUFTensorInfo) Get(name string) (info GGUFTensorInfo, found bool) {
	if ti.Name == name {
		return ti, true
	}
	return GGUFTensorInfo{}, false
}

// GetFileType returns the GGUFFileType.
func (ti GGUFTensorInfo) GetFileType() GGUFFileType {
	return GetFileType(map[GGMLType]int{ti.Type: 1})
}

// Match returns true if the name of the GGUFTensorInfo matches the given regex.
func (ti GGUFTensorInfo) Match(nameRegex *regexp.Regexp) bool {
	return nameRegex.MatchString(ti.Name)
}

// Search returns a list of GGUFTensorInfo with the names that match the given regex.
func (ti GGUFTensorInfo) Search(nameRegex *regexp.Regexp) (infos []GGUFTensorInfo) {
	if nameRegex.MatchString(ti.Name) {
		return []GGUFTensorInfo{ti}
	}
	return nil
}

// Index returns a map value to the GGUFTensorInfo with the given names,
// and the number of names found.
func (ti GGUFTensorInfo) Index(names []string) (infos map[string]GGUFTensorInfo, found int) {
	if len(names) == 0 {
		return nil, 0
	}
	if names[0] == ti.Name {
		return map[string]GGUFTensorInfo{ti.Name: ti}, 1
	}
	return nil, 0
}

// Elements returns the number of elements of the GGUFTensorInfo,
// which is inspired by
// https://github.com/ggerganov/ggml/blob/a10a8b880c059b3b29356eb9a9f8df72f03cdb6a/src/ggml.c#L2597-L2601.
func (ti GGUFTensorInfo) Elements(filter ...GGUFTensorInfoFilter) uint64 {
	if ti.NDimensions == 0 {
		return 0
	}

	for i := range filter {
		if filter[i] != nil && !filter[i](ti.Name) {
			return 0
		}
	}

	ret := uint64(1)
	for i := uint32(0); i < ti.NDimensions; i++ {
		// Overflow-checked: a wrapped product would silently understate the
		// element count and mislead memory/VRAM estimation downstream.
		ret = mulU64(ret, ti.Dimensions[i], "Elements")
	}
	return ret
}

// Bytes returns the number of bytes of the GGUFTensorInfo,
// which is inspired by
// https://github.com/ggerganov/ggml/blob/a10a8b880c059b3b29356eb9a9f8df72f03cdb6a/src/ggml.c#L2609-L2626.
func (ti GGUFTensorInfo) Bytes(filter ...GGUFTensorInfoFilter) uint64 {
	if ti.NDimensions == 0 {
		return 0
	}

	tt, ok := ti.Type.Trait()
	if !ok {
		panic(fmt.Errorf("invalid type: %v", ti.Type))
	}

	for i := range filter {
		if filter[i] != nil && !filter[i](ti.Name) {
			return 0
		}
	}

	// https://github.com/ggerganov/ggml/blob/a10a8b880c059b3b29356eb9a9f8df72f03cdb6a/src/ggml.c#L3210-L3214
	//
	// Every uint64 multiplication and addition below is overflow-checked. A
	// silent wrap here would cause Bytes() to drastically understate tensor
	// memory and let crafted GGUFs bypass sizing/allocation guards.
	nb := make([]uint64, 0, ti.NDimensions)
	{
		nb = append(nb, tt.TypeSize)
		nb = append(nb, mulU64(nb[0], ti.Dimensions[0]/tt.BlockSize, "Bytes nb[1]"))
		for i := uint32(2); i < ti.NDimensions; i++ {
			nb = append(nb, mulU64(nb[i-1], ti.Dimensions[i-1], "Bytes nb[i]"))
		}
	}

	var ret uint64
	if tt.BlockSize == 1 {
		ret = tt.TypeSize
		for i := uint32(0); i < ti.NDimensions; i++ {
			ret = addU64(ret, mulU64(ti.Dimensions[i]-1, nb[i], "Bytes stride"), "Bytes sum")
		}
		return ret
	}

	ret = mulU64(ti.Dimensions[0], nb[0], "Bytes head") / tt.BlockSize
	for i := uint32(1); i < ti.NDimensions; i++ {
		ret = addU64(ret, mulU64(ti.Dimensions[i]-1, nb[i], "Bytes stride"), "Bytes sum")
	}
	return ret
}

// Count returns the number of GGUF tensors of the GGUFTensorInfo,
// which is always 1.
func (ti GGUFTensorInfo) Count() uint64 {
	return 1
}

// Get returns the GGUFTensorInfo with the given name,
// and true if found, and false otherwise.
func (tis GGUFTensorInfos) Get(name string) (info GGUFTensorInfo, found bool) {
	for i := range tis {
		if tis[i].Name == name {
			return tis[i], true
		}
	}
	return GGUFTensorInfo{}, false
}

// GetFileType returns the GGUFFileType represented the mostly GGMLType of the GGUFTensorInfos.
func (tis GGUFTensorInfos) GetFileType() GGUFFileType {
	if len(tis) == 0 {
		return _GGUFFileTypeCount
	}

	cm := make(map[GGMLType]int)
	for i := range tis {
		cm[tis[i].Type]++
	}

	return GetFileType(cm)
}

// Match returns true if a tensor of GGUFTensorInfos matches the given regex.
func (tis GGUFTensorInfos) Match(nameRegex *regexp.Regexp) bool {
	for i := range tis {
		if nameRegex.MatchString(tis[i].Name) {
			return true
		}
	}
	return false
}

// Search returns a list of GGUFTensorInfo with the names that match the given regex.
func (tis GGUFTensorInfos) Search(nameRegex *regexp.Regexp) (infos []GGUFTensorInfo) {
	for i := range tis {
		if nameRegex.MatchString(tis[i].Name) {
			infos = append(infos, tis[i])
		}
	}
	return infos
}

// Index returns a map value to the GGUFTensorInfos with the given names,
// and the number of names found.
func (tis GGUFTensorInfos) Index(names []string) (infos map[string]GGUFTensorInfo, found int) {
	ns := make(map[string]struct{}, len(names))
	for i := range names {
		ns[names[i]] = struct{}{}
	}
	infos = make(map[string]GGUFTensorInfo)
	for i := range tis {
		if _, ok := ns[tis[i].Name]; ok {
			infos[tis[i].Name] = tis[i]
			found++
		}
		if found == len(ns) {
			break
		}
	}
	return infos, found
}

// Elements returns the number of elements of the GGUFTensorInfos.
func (tis GGUFTensorInfos) Elements() uint64 {
	var ret uint64
	for i := range tis {
		ret += tis[i].Elements()
	}
	return ret
}

// Bytes returns the number of bytes of the GGUFTensorInfos.
func (tis GGUFTensorInfos) Bytes() uint64 {
	var ret uint64
	for i := range tis {
		ret += tis[i].Bytes()
	}
	return ret
}

// Count returns the number of GGUF tensors of the GGUFTensorInfos.
func (tis GGUFTensorInfos) Count() uint64 {
	return uint64(len(tis))
}

// Layers converts the GGUFTensorInfos to GGUFLayerTensorInfos.
func (tis GGUFTensorInfos) Layers(ignores ...string) GGUFLayerTensorInfos {
	if len(tis) == 0 {
		return nil
	}

	ls := tis.layers()
	if len(ignores) != 0 {
		_, ls, _ = ls.Cut(ignores)
		return ls
	}
	return ls
}

var numberRegex = regexp.MustCompile(`^\d+$`)

func (tis GGUFTensorInfos) layers() GGUFLayerTensorInfos {
	var ret GGUFLayerTensorInfos

	pm := make(map[string]any)
	for i := range tis {
		ps := strings.Split(tis[i].Name, ".")
		if len(ps) < 2 {
			ret = append(ret, tis[i])
			continue
		}
		switch {
		default:
			ret = append(ret, tis[i])
		case ps[0] == "blk" || ps[0] == "block":
			// LLaMACpp.
			p := strings.Join([]string{ps[0], ps[1]}, ".")
			if _, ok := pm[p]; !ok {
				l := &GGUFNamedTensorInfos{Name: p}
				pm[p] = l
				ret = append(ret, l)
			}
			l := pm[p].(*GGUFNamedTensorInfos)
			l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, tis[i])
		case (ps[0] == "v" || ps[0] == "t" || ps[0] == "a") && ps[1] == "blk":
			// LLaMACpp CLIP.
			p := ps[0]
			if _, ok := pm[p]; !ok {
				l := &GGUFNamedTensorInfos{Name: p}
				pm[p] = l
				ret = append(ret, l)
			}
			l := pm[p].(*GGUFNamedTensorInfos)
			if len(ps) < 3 {
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, tis[i])
				continue
			}
			p = strings.Join([]string{ps[0], ps[1], ps[2]}, ".")
			if _, ok := pm[p]; !ok {
				xl := &GGUFNamedTensorInfos{Name: p}
				pm[p] = xl
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, xl)
			}
			xl := pm[p].(*GGUFNamedTensorInfos)
			xl.GGUFLayerTensorInfos = append(xl.GGUFLayerTensorInfos, tis[i])
		case ((ps[0] == "dec" || ps[0] == "enc") && ps[1] == "blk") ||
			((ps[0] == "decoder" || ps[0] == "encoder") && ps[1] == "block"):
			// BERT.
			p := ps[0]
			if _, ok := pm[p]; !ok {
				l := &GGUFNamedTensorInfos{Name: p}
				pm[p] = l
				ret = append(ret, l)
			}
			l := pm[p].(*GGUFNamedTensorInfos)
			if len(ps) < 3 {
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, tis[i])
				continue
			}
			p = strings.Join([]string{ps[0], ps[1], ps[2]}, ".")
			if _, ok := pm[p]; !ok {
				xl := &GGUFNamedTensorInfos{Name: p}
				pm[p] = xl
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, xl)
			}
			xl := pm[p].(*GGUFNamedTensorInfos)
			xl.GGUFLayerTensorInfos = append(xl.GGUFLayerTensorInfos, tis[i])
		case ps[0] == "first_stage_model":
			// StableDiffusionCpp Autoencoder.
			p := strings.Join([]string{ps[0], ps[1]}, ".")
			if _, ok := pm[p]; !ok {
				l := &GGUFNamedTensorInfos{Name: p}
				pm[p] = l
				ret = append(ret, l)
			}
			l := pm[p].(*GGUFNamedTensorInfos)
			if len(ps) < 3 {
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, tis[i])
				continue
			}
			p = strings.Join([]string{ps[0], ps[1], ps[2]}, ".")
			if _, ok := pm[p]; !ok {
				xl := &GGUFNamedTensorInfos{Name: p}
				pm[p] = xl
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, xl)
			}
			xl := pm[p].(*GGUFNamedTensorInfos)
			xl.GGUFLayerTensorInfos = append(xl.GGUFLayerTensorInfos, tis[i])
		case ps[0] == "cond_stage_model":
			// StableDiffusionCpp Conditioner.
			if len(ps) < 3 {
				ret = append(ret, tis[i])
				continue
			}
			p := strings.Join([]string{ps[0], ps[1], ps[2]}, ".")
			if !numberRegex.MatchString(ps[1]) {
				p = strings.Join([]string{ps[0], ps[1]}, ".")
			}
			if _, ok := pm[p]; !ok {
				l := &GGUFNamedTensorInfos{Name: p}
				pm[p] = l
				ret = append(ret, l)
			}
			l := pm[p].(*GGUFNamedTensorInfos)
			if len(ps) < 4 {
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, tis[i])
				continue
			}
			p = strings.Join([]string{ps[0], ps[1], ps[2], ps[3]}, ".")
			if !numberRegex.MatchString(ps[1]) {
				p = strings.Join([]string{ps[0], ps[1], ps[2]}, ".")
			}
			if _, ok := pm[p]; !ok {
				xl := &GGUFNamedTensorInfos{Name: p}
				pm[p] = xl
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, xl)
			}
			xl := pm[p].(*GGUFNamedTensorInfos)
			xl.GGUFLayerTensorInfos = append(xl.GGUFLayerTensorInfos, tis[i])
		case ps[0] == "model" && ps[1] == "diffusion_model": // nolint: goconst
			// StableDiffusionCpp.
			p := "model.diffusion_model"
			if _, ok := pm[p]; !ok {
				l := &GGUFNamedTensorInfos{Name: p}
				pm[p] = l
				ret = append(ret, l)
			}
			l := pm[p].(*GGUFNamedTensorInfos)
			if len(ps) < 3 {
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, tis[i])
				continue
			}
			p = strings.Join([]string{"model.diffusion_model", ps[2]}, ".")
			if _, ok := pm[p]; !ok {
				xl := &GGUFNamedTensorInfos{Name: p}
				pm[p] = xl
				l.GGUFLayerTensorInfos = append(l.GGUFLayerTensorInfos, xl)
			}
			xl := pm[p].(*GGUFNamedTensorInfos)
			xl.GGUFLayerTensorInfos = append(xl.GGUFLayerTensorInfos, tis[i])
		}
	}
	return ret
}

// Get returns the IGGUFTensorInfos with the given name,
// and true if found, and false otherwise.
func (ltis GGUFLayerTensorInfos) Get(name string) (info GGUFTensorInfo, found bool) {
	for i := range ltis {
		switch v := ltis[i].(type) {
		case GGUFTensorInfo:
			if v.Name == name {
				return v, true
			}
		case *GGUFNamedTensorInfos:
			info, found = v.GGUFLayerTensorInfos.Get(name)
			if found {
				return info, true
			}
		}
	}
	return GGUFTensorInfo{}, false
}

// GetFileType returns the GGUFFileType represented the mostly GGMLType of the GGUFLayerTensorInfos.
func (ltis GGUFLayerTensorInfos) GetFileType() GGUFFileType {
	if len(ltis) == 0 {
		return _GGUFFileTypeCount
	}

	cm := make(map[GGMLType]int)
	for i := range ltis {
		switch v := ltis[i].(type) {
		case GGUFTensorInfo:
			cm[v.Type]++
		case *GGUFNamedTensorInfos:
			cm[v.GetFileType().GGMLType()]++
		}
	}

	return GetFileType(cm)
}

// Match returns true if a tensor of GGUFLayerTensorInfos matches the given regex.
func (ltis GGUFLayerTensorInfos) Match(nameRegex *regexp.Regexp) bool {
	for i := range ltis {
		switch v := ltis[i].(type) {
		case GGUFTensorInfo:
			if nameRegex.MatchString(v.Name) {
				return true
			}
		case *GGUFNamedTensorInfos:
			if v.Match(nameRegex) {
				return true
			}
		}
	}
	return false
}

// Search returns a list of GGUFTensorInfo with the names that match the given regex.
func (ltis GGUFLayerTensorInfos) Search(nameRegex *regexp.Regexp) (infos []GGUFTensorInfo) {
	for i := range ltis {
		switch v := ltis[i].(type) {
		case GGUFTensorInfo:
			if nameRegex.MatchString(v.Name) {
				infos = append(infos, v)
			}
		case *GGUFNamedTensorInfos:
			infos = append(infos, v.Search(nameRegex)...)
		}
	}
	return infos
}

// Index returns a map value to the GGUFTensorInfos with the given names,
// and the number of names found.
func (ltis GGUFLayerTensorInfos) Index(names []string) (infos map[string]GGUFTensorInfo, found int) {
	ns := make(map[string]struct{}, len(names))
	for i := range names {
		ns[names[i]] = struct{}{}
	}
	infos = make(map[string]GGUFTensorInfo)
	for i := range ltis {
		switch v := ltis[i].(type) {
		case GGUFTensorInfo:
			if _, ok := ns[v.Name]; ok {
				infos[v.Name] = v
				found++
			}
		case *GGUFNamedTensorInfos:
			inf, _ := v.Index(names)
			for k := range inf {
				infos[k] = inf[k]
				found++
			}
		}
		if found == len(ns) {
			break
		}
	}
	return infos, found
}

// Elements returns the number of elements of the GGUFLayerTensorInfos.
func (ltis GGUFLayerTensorInfos) Elements(filter ...GGUFTensorInfoFilter) uint64 {
	var ret uint64
	for i := range ltis {
		ret += ltis[i].Elements(filter...)
	}
	return ret
}

// Bytes returns the number of bytes of the GGUFLayerTensorInfos.
func (ltis GGUFLayerTensorInfos) Bytes(filter ...GGUFTensorInfoFilter) uint64 {
	var ret uint64
	for i := range ltis {
		ret += ltis[i].Bytes(filter...)
	}
	return ret
}

// Count returns the number of GGUF tensors of the GGUFLayerTensorInfos.
func (ltis GGUFLayerTensorInfos) Count() uint64 {
	var ret uint64
	for i := range ltis {
		ret += ltis[i].Count()
	}
	return ret
}

// Cut splits the GGUFLayerTensorInfos into two parts,
// and returns the GGUFLayerTensorInfos with the names that match the given names at first,
// and the GGUFLayerTensorInfos without the names at second,
// and true if the GGUFLayerTensorInfos with the names are found, and false otherwise.
//
// The given names support glob pattern, for example, "a*" matches "a", "ab", "abc", and so on.
func (ltis GGUFLayerTensorInfos) Cut(names []string) (before, after GGUFLayerTensorInfos, found bool) {
	prefixes := make(map[string]struct{})
	matches := make(map[string]struct{})
	for i := range names {
		if strings.HasSuffix(names[i], "*") {
			prefixes[strings.TrimSuffix(names[i], "*")] = struct{}{}
		} else {
			matches[names[i]] = struct{}{}
		}
	}
	before = make(GGUFLayerTensorInfos, 0, len(names))
	after = make(GGUFLayerTensorInfos, 0, len(ltis))

	for i := range ltis {
		switch v := ltis[i].(type) {
		case GGUFTensorInfo:
			if len(matches) != 0 {
				if _, ok := matches[v.Name]; ok {
					before = append(before, v)
					continue
				}
			}
			if len(prefixes) != 0 {
				var check bool
				for prefix := range prefixes {
					if strings.HasPrefix(v.Name, prefix) {
						before = append(before, v)
						check = true
						break
					}
				}
				if check {
					continue
				}
			}
			after = append(after, v)
		case *GGUFNamedTensorInfos:
			if len(matches) != 0 {
				if _, ok := matches[v.Name]; ok {
					before = append(before, v)
					continue
				}
			}
			if len(prefixes) != 0 {
				var check bool
				for prefix := range prefixes {
					if strings.HasPrefix(v.Name, prefix) {
						before = append(before, v)
						check = true
						break
					}
				}
				if check {
					continue
				}
			}
			after = append(after, v)
		}
	}
	return before, after, len(before) > 0
}

type _GGUFReader struct {
	v  GGUFVersion
	o  _GGUFReadOptions
	f  io.ReadSeeker
	bo binary.ByteOrder
}

func (rd _GGUFReader) ReadUint8() (v uint8, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read uint8: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadInt8() (v int8, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read int8: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadUint16() (v uint16, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read uint16: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadInt16() (v int16, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read int16: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadUint32() (v uint32, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read uint32: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadUint64FromUint32() (uint64, error) {
	v, err := rd.ReadUint32()
	return uint64(v), err
}

func (rd _GGUFReader) ReadInt32() (v int32, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read int32: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadFloat32() (v float32, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read float32: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadBool() (v bool, err error) {
	b, err := rd.ReadUint8()
	if err != nil {
		return false, fmt.Errorf("read bool: %w", err)
	}
	return b != 0, nil
}

func (rd _GGUFReader) ReadString() (v string, err error) {
	var l uint64
	if rd.v <= GGUFVersionV1 {
		l, err = rd.ReadUint64FromUint32()
	} else {
		l, err = rd.ReadUint64()
	}
	if err != nil {
		return "", fmt.Errorf("read string length: %w", err)
	}

	b := bytex.GetBytes(l)
	defer bytex.Put(b)
	if _, err = rd.f.Read(b); err != nil {
		return "", fmt.Errorf("read string: %w", err)
	}

	return string(bytes.TrimSpace(b)), nil
}

func (rd _GGUFReader) SkipReadingString() (err error) {
	var l uint64
	if rd.v <= GGUFVersionV1 {
		l, err = rd.ReadUint64FromUint32()
	} else {
		l, err = rd.ReadUint64()
	}
	if err != nil {
		return fmt.Errorf("read string length: %w", err)
	}
	// Bound-check before casting to int64: a length > math.MaxInt64 would
	// wrap to a negative offset and seek backwards through the file.
	delta, err := safeSeekDelta(l, 1, "skip string")
	if err != nil {
		return err
	}
	_, err = rd.f.Seek(delta, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("seek string: %w", err)
	}
	return nil
}

func (rd _GGUFReader) ReadArray(key string) (v GGUFMetadataKVArrayValue, err error) {
	v.StartOffset, err = rd.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return v, fmt.Errorf("read array start: %w", err)
	}

	if err = binary.Read(rd.f, rd.bo, &v.Type); err != nil {
		return v, fmt.Errorf("read array item type: %w", err)
	}

	if rd.v <= GGUFVersionV1 {
		v.Len, err = rd.ReadUint64FromUint32()
	} else {
		v.Len, err = rd.ReadUint64()
	}
	if err != nil {
		return v, fmt.Errorf("read array length: %w", err)
	}

	itemStart, err := rd.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return v, fmt.Errorf("seek array item start: %w", err)
	}

	if !rd.o.SkipLargeMetadata || stringx.HasSuffixes(key, ".feed_forward_length", ".attention.head_count") {
		v.Array = make([]any, v.Len)
		for i := uint64(0); i < v.Len; i++ {
			v.Array[i], err = rd.ReadValue(key, v.Type)
			if err != nil {
				return v, fmt.Errorf("read array item %d: %w", i, err)
			}
		}

		itemEnd, err := rd.f.Seek(0, io.SeekCurrent)
		if err != nil {
			return v, fmt.Errorf("seek array item end: %w", err)
		}
		v.Size = itemEnd - itemStart

		return v, nil
	}

	// Each branch computes v.Len*elemSize through safeSeekDelta so a crafted
	// array length cannot overflow int64 and turn the forward seek into a
	// negative-offset seek (which would re-expose earlier bytes to the
	// parser at an attacker-chosen offset).
	switch v.Type {
	case GGUFMetadataValueTypeUint8, GGUFMetadataValueTypeInt8, GGUFMetadataValueTypeBool:
		var delta int64
		delta, err = safeSeekDelta(v.Len, 1, "skip array u8")
		if err == nil {
			_, err = rd.f.Seek(delta, io.SeekCurrent)
		}
	case GGUFMetadataValueTypeUint16, GGUFMetadataValueTypeInt16:
		var delta int64
		delta, err = safeSeekDelta(v.Len, 2, "skip array u16")
		if err == nil {
			_, err = rd.f.Seek(delta, io.SeekCurrent)
		}
	case GGUFMetadataValueTypeUint32, GGUFMetadataValueTypeInt32, GGUFMetadataValueTypeFloat32:
		var delta int64
		delta, err = safeSeekDelta(v.Len, 4, "skip array u32")
		if err == nil {
			_, err = rd.f.Seek(delta, io.SeekCurrent)
		}
	case GGUFMetadataValueTypeUint64, GGUFMetadataValueTypeInt64, GGUFMetadataValueTypeFloat64:
		var delta int64
		delta, err = safeSeekDelta(v.Len, 8, "skip array u64")
		if err == nil {
			_, err = rd.f.Seek(delta, io.SeekCurrent)
		}
	case GGUFMetadataValueTypeString:
		for i := uint64(0); i < v.Len; i++ {
			if err = rd.SkipReadingString(); err != nil {
				return v, fmt.Errorf("seek array[string] %d: %w", i, err)
			}
		}
	default:
		// Should not happen.
		panic(fmt.Errorf("invalid type: %v", v.Type))
	}
	if err != nil {
		return v, fmt.Errorf("seek array end: %w", err)
	}

	itemEnd, err := rd.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return v, fmt.Errorf("seek array item end: %w", err)
	}
	v.Size = itemEnd - itemStart

	return v, nil
}

func (rd _GGUFReader) ReadUint64() (v uint64, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read uint64: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadInt64() (v int64, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read int64: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadFloat64() (v float64, err error) {
	err = binary.Read(rd.f, rd.bo, &v)
	if err != nil {
		return 0, fmt.Errorf("read float64: %w", err)
	}
	return v, nil
}

func (rd _GGUFReader) ReadValue(vk string, vt GGUFMetadataValueType) (v any, err error) {
	if vt >= _GGUFMetadataValueTypeCount {
		return nil, fmt.Errorf("invalid type: %v", vt)
	}

	switch vt {
	case GGUFMetadataValueTypeUint8:
		v, err = rd.ReadUint8()
	case GGUFMetadataValueTypeInt8:
		v, err = rd.ReadInt8()
	case GGUFMetadataValueTypeUint16:
		v, err = rd.ReadUint16()
	case GGUFMetadataValueTypeInt16:
		v, err = rd.ReadInt16()
	case GGUFMetadataValueTypeUint32:
		v, err = rd.ReadUint32()
	case GGUFMetadataValueTypeInt32:
		v, err = rd.ReadInt32()
	case GGUFMetadataValueTypeFloat32:
		v, err = rd.ReadFloat32()
	case GGUFMetadataValueTypeBool:
		v, err = rd.ReadBool()
	case GGUFMetadataValueTypeString:
		v, err = rd.ReadString()
	case GGUFMetadataValueTypeArray:
		v, err = rd.ReadArray(vk)
	case GGUFMetadataValueTypeUint64:
		v, err = rd.ReadUint64()
	case GGUFMetadataValueTypeInt64:
		v, err = rd.ReadInt64()
	case GGUFMetadataValueTypeFloat64:
		v, err = rd.ReadFloat64()
	default:
		// Should not happen.
		panic(fmt.Errorf("invalid type: %v", vt))
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

type _GGUFMetadataReader struct {
	_GGUFReader
}

func (rd _GGUFMetadataReader) Read() (kv GGUFMetadataKV, err error) {
	kv.Key, err = rd.ReadString()
	if err != nil {
		return kv, fmt.Errorf("read key: %w", err)
	}

	{
		vt, err := rd.ReadUint32()
		if err != nil {
			return kv, fmt.Errorf("read value type: %w", err)
		}
		kv.ValueType = GGUFMetadataValueType(vt)
		if kv.ValueType >= _GGUFMetadataValueTypeCount {
			return kv, fmt.Errorf("invalid value type: %v", kv.ValueType)
		}
	}

	kv.Value, err = rd.ReadValue(kv.Key, kv.ValueType)
	if err != nil {
		return kv, fmt.Errorf("read %s value: %w", kv.Key, err)
	}

	return kv, nil
}

type _GGUFTensorInfoReader struct {
	_GGUFReader
}

func (rd _GGUFTensorInfoReader) Read() (ti GGUFTensorInfo, err error) {
	ti.StartOffset, err = rd.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return ti, fmt.Errorf("seek tensor info start: %w", err)
	}

	ti.Name, err = rd.ReadString()
	if err != nil {
		return ti, fmt.Errorf("read name: %w", err)
	}

	ti.NDimensions, err = rd.ReadUint32()
	if err != nil {
		return ti, fmt.Errorf("read n dimensions: %w", err)
	}
	// Reject malformed dimension counts before allocating; GGML caps tensors
	// at GGMLMaxDims, and 0 is meaningless. Without this a crafted file with
	// NDimensions == math.MaxUint32 would request a huge slice allocation.
	if ti.NDimensions == 0 || ti.NDimensions > GGMLMaxDims {
		return ti, fmt.Errorf("invalid n dimensions: %d (must be 1..%d)", ti.NDimensions, GGMLMaxDims)
	}

	ti.Dimensions = make([]uint64, ti.NDimensions)
	for i := uint32(0); i < ti.NDimensions; i++ {
		if rd.v <= GGUFVersionV1 {
			ti.Dimensions[i], err = rd.ReadUint64FromUint32()
		} else {
			ti.Dimensions[i], err = rd.ReadUint64()
		}
		if err != nil {
			return ti, fmt.Errorf("read dimension %d: %w", i, err)
		}
	}

	{
		v, err := rd.ReadUint32()
		if err != nil {
			return ti, fmt.Errorf("read type: %w", err)
		}
		ti.Type = GGMLType(v)
		if ti.Type >= _GGMLTypeCount {
			return ti, fmt.Errorf("%v: This quantized type is currently unsupported", ti.Type)
		}
	}

	ti.Offset, err = rd.ReadUint64()
	if err != nil {
		return ti, fmt.Errorf("read offset: %w", err)
	}

	return ti, nil
}
