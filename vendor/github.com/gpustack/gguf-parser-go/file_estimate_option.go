package gguf_parser

import (
	"regexp"
	"slices"
	"strconv"

	"github.com/gpustack/gguf-parser-go/util/ptr"
)

type (
	_GGUFRunEstimateOptions struct {
		// Common
		ParallelSize        *int32
		FlashAttention      bool
		MainGPUIndex        int
		RPCServers          []string
		TensorSplitFraction []float64
		OverriddenTensors   []*GGUFRunOverriddenTensor
		DeviceMetrics       []GGUFRunDeviceMetric

		// LLaMACpp (LMC) specific
		LMCContextSize                    *int32
		LMCRoPEFrequencyBase              *float32
		LMCRoPEFrequencyScale             *float32
		LMCRoPEScalingType                *string
		LMCRoPEScalingOriginalContextSize *int32
		LMCInMaxContextSize               bool
		LMCLogicalBatchSize               *int32
		LMCPhysicalBatchSize              *int32
		LMCVisualMaxImageSize             *uint32
		LMCMaxProjectedCache              *uint32
		LMCCacheKeyType                   *GGMLType
		LMCCacheValueType                 *GGMLType
		LMCOffloadKVCache                 *bool
		LMCOffloadLayers                  *uint64
		LMCSplitMode                      LLaMACppSplitMode
		LMCFullSizeSWACache               bool
		LMCProjector                      *LLaMACppRunEstimate
		LMCDrafter                        *LLaMACppRunEstimate
		LMCAdapters                       []LLaMACppRunEstimate

		// StableDiffusionCpp (SDC) specific
		SDCOffloadLayers                *uint64
		SDCBatchCount                   *int32
		SDCHeight                       *uint32
		SDCWidth                        *uint32
		SDCOffloadConditioner           *bool
		SDCOffloadAutoencoder           *bool
		SDCAutoencoderTiling            *bool
		SDCFreeComputeMemoryImmediately *bool
		SDCUpscaler                     *StableDiffusionCppRunEstimate
		SDCControlNet                   *StableDiffusionCppRunEstimate
	}

	// GGUFRunOverriddenTensor holds the overridden tensor information for the estimate.
	//
	// When BufferType is CPU,
	// it indicates that the tensor should be loaded into the CPU memory,
	// even if it belongs to a GPU offload layer.
	GGUFRunOverriddenTensor struct {
		// PatternRegex is the regex pattern to match the tensor name.
		PatternRegex *regexp.Regexp
		// BufferType is the buffer type to override,
		// it can be "CPU", "CUDA0", "Metal" and others.
		BufferType string

		// _BufferType record parsed buffer type, used internally.
		_BufferType GGUFRunOverriddenTensorBufferType
		// _Index record parsed device index, used internally.
		_Index string
	}

	// GGUFRunDeviceMetric holds the device metric for the estimate.
	//
	// When the device represents a CPU,
	// FLOPS refers to the floating-point operations per second of that CPU,
	// while UpBandwidth indicates the bandwidth of the RAM (since SRAM is typically small and cannot hold all weights,
	// the RAM here refers to the bandwidth of DRAM,
	// unless the device's SRAM can accommodate the corresponding model weights).
	//
	// When the device represents a GPU,
	// FLOPS refers to the floating-point operations per second of that GPU,
	// while UpBandwidth indicates the bandwidth of the VRAM.
	//
	// When the device represents a specific node,
	// FLOPS depends on whether a CPU or GPU is being used,
	// while UpBandwidth refers to the network bandwidth between nodes.
	GGUFRunDeviceMetric struct {
		// FLOPS is the floating-point operations per second of the device.
		FLOPS FLOPSScalar
		// UpBandwidth is the bandwidth of the device to transmit data to calculate,
		// unit is Bps (bytes per second).
		UpBandwidth BytesPerSecondScalar
		// DownBandwidth is the bandwidth of the device to transmit calculated result to next layer,
		// unit is Bps (bytes per second).
		DownBandwidth BytesPerSecondScalar
	}

	// GGUFRunEstimateOption is the options for the estimate.
	GGUFRunEstimateOption func(*_GGUFRunEstimateOptions)
)

// GGUFRunOverriddenTensorBufferType is the type of the overridden tensor buffer.
type GGUFRunOverriddenTensorBufferType uint32

const (
	_ GGUFRunOverriddenTensorBufferType = iota
	GGUFRunOverriddenTensorBufferTypeCPU
	GGUFRunOverriddenTensorBufferTypeGPU
	GGUFRunOverriddenTensorBufferTypeRPC
	GGUFRunOverriddenTensorBufferTypeUnknown
)

var (
	_GGUFRunOverriddenTensorBufferTypeCPURegex       = regexp.MustCompile(`^(CPU|AMX)`)
	_GGUFRunOverriddenTensorBufferTypeUMAGPURegex    = regexp.MustCompile(`^(Metal|OpenCL)`)
	_GGUFRunOverriddenTensorBufferTypeNonUMAGPURegex = regexp.MustCompile(`^(CUDA|CANN|ROCm|MUSA|SYCL|Vulkan|Kompute)(\d+)?`)
	_GGUFRunOverriddenTensorBufferTypeRPCRegex       = regexp.MustCompile(`^RPC\[(.*)\]`)
)

// ParseBufferType returns the device index of the overridden tensor.
//
// The device index is used to determine which device the tensor belongs to,
// it is according to the buffer type description.
func (odt *GGUFRunOverriddenTensor) ParseBufferType() (GGUFRunOverriddenTensorBufferType, string) {
	if odt == nil {
		return GGUFRunOverriddenTensorBufferTypeUnknown, ""
	}

	if odt._BufferType == 0 {
		odt._BufferType = GGUFRunOverriddenTensorBufferTypeUnknown
		if ms := _GGUFRunOverriddenTensorBufferTypeCPURegex.FindStringSubmatch(odt.BufferType); len(ms) > 1 {
			odt._BufferType, odt._Index = GGUFRunOverriddenTensorBufferTypeCPU, "0"
		}
		if ms := _GGUFRunOverriddenTensorBufferTypeUMAGPURegex.FindStringSubmatch(odt.BufferType); len(ms) > 1 {
			odt._BufferType, odt._Index = GGUFRunOverriddenTensorBufferTypeGPU, "1"
		}
		if ms := _GGUFRunOverriddenTensorBufferTypeRPCRegex.FindStringSubmatch(odt.BufferType); len(ms) > 1 {
			odt._BufferType, odt._Index = GGUFRunOverriddenTensorBufferTypeRPC, ms[1]
		}
		if ms := _GGUFRunOverriddenTensorBufferTypeNonUMAGPURegex.FindStringSubmatch(odt.BufferType); len(ms) > 2 {
			if idx, err := strconv.ParseInt(ms[2], 10, 64); err == nil && idx >= 0 {
				odt._BufferType, odt._Index = GGUFRunOverriddenTensorBufferTypeGPU, ms[2]
			}
		}
	}
	return odt._BufferType, odt._Index
}

// WithParallelSize sets the (decoding sequences) parallel size for the estimate.
func WithParallelSize(size int32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if size <= 0 {
			return
		}
		o.ParallelSize = &size
	}
}

// WithFlashAttention sets the flash attention flag.
func WithFlashAttention() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.FlashAttention = true
	}
}

// WithMainGPUIndex sets the main device for the estimate.
//
// When split mode is LLaMACppSplitModeNone, the main device is the only device.
// When split mode is LLaMACppSplitModeRow, the main device handles the intermediate results and KV.
//
// WithMainGPUIndex needs to combine with WithTensorSplitFraction.
func WithMainGPUIndex(di int) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.MainGPUIndex = di
	}
}

// WithRPCServers sets the RPC servers for the estimate.
func WithRPCServers(srvs []string) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if len(srvs) == 0 {
			return
		}
		o.RPCServers = srvs
	}
}

// WithTensorSplitFraction sets the tensor split cumulative fractions for the estimate.
//
// WithTensorSplitFraction accepts a variadic number of fractions,
// all fraction values must be in the range of [0, 1],
// and the last fraction must be 1.
//
// For example, WithTensorSplitFraction(0.2, 0.4, 0.6, 0.8, 1) will split the tensor into five parts with 20% each.
func WithTensorSplitFraction(fractions []float64) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if len(fractions) == 0 {
			return
		}
		for _, f := range fractions {
			if f < 0 || f > 1 {
				return
			}
		}
		if fractions[len(fractions)-1] != 1 {
			return
		}
		o.TensorSplitFraction = fractions
	}
}

// WithOverriddenTensors sets the overridden tensors for the estimate.
func WithOverriddenTensors(tensors []GGUFRunOverriddenTensor) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if len(tensors) == 0 {
			return
		}
		for _, t := range tensors {
			if t.PatternRegex == nil || t.BufferType == "" {
				return
			}
		}
		o.OverriddenTensors = make([]*GGUFRunOverriddenTensor, len(tensors))
		for i := range tensors {
			o.OverriddenTensors[i] = &tensors[i]
		}
	}
}

// WithDeviceMetrics sets the device metrics for the estimate.
func WithDeviceMetrics(metrics []GGUFRunDeviceMetric) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if len(metrics) == 0 {
			return
		}
		o.DeviceMetrics = metrics
	}
}

// WithLLaMACppContextSize sets the context size for the estimate.
func WithLLaMACppContextSize(size int32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if size <= 0 {
			return
		}
		o.LMCContextSize = &size
	}
}

// WithLLaMACppRoPE sets the RoPE parameters for the estimate.
func WithLLaMACppRoPE(
	frequencyBase float64,
	frequencyScale float64,
	scalingType string,
	scalingOriginalContextSize int32,
) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if frequencyBase > 0 {
			o.LMCRoPEFrequencyBase = ptr.Float32(float32(frequencyBase))
		}
		if frequencyScale > 0 {
			o.LMCRoPEFrequencyScale = ptr.Float32(float32(frequencyScale))
		}
		if slices.Contains([]string{"none", "linear", "yarn"}, scalingType) {
			o.LMCRoPEScalingType = &scalingType
		}
		if scalingOriginalContextSize > 0 {
			o.LMCRoPEScalingOriginalContextSize = ptr.To(scalingOriginalContextSize)
		}
	}
}

// WithinLLaMACppMaxContextSize limits the context size to the maximum,
// if the context size is over the maximum.
func WithinLLaMACppMaxContextSize() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.LMCInMaxContextSize = true
	}
}

// WithLLaMACppLogicalBatchSize sets the logical batch size for the estimate.
func WithLLaMACppLogicalBatchSize(size int32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if size <= 0 {
			return
		}
		o.LMCLogicalBatchSize = &size
	}
}

// WithLLaMACppPhysicalBatchSize sets the physical batch size for the estimate.
func WithLLaMACppPhysicalBatchSize(size int32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if size <= 0 {
			return
		}
		o.LMCPhysicalBatchSize = &size
	}
}

// _GGUFEstimateCacheTypeAllowList is the allow list of cache key and value types.
var _GGUFEstimateCacheTypeAllowList = []GGMLType{
	GGMLTypeF32,
	GGMLTypeF16,
	GGMLTypeBF16,
	GGMLTypeQ8_0,
	GGMLTypeQ4_0, GGMLTypeQ4_1,
	GGMLTypeIQ4_NL,
	GGMLTypeQ5_0, GGMLTypeQ5_1,
}

// WithLLaMACppCacheKeyType sets the cache key type for the estimate.
func WithLLaMACppCacheKeyType(t GGMLType) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if slices.Contains(_GGUFEstimateCacheTypeAllowList, t) {
			o.LMCCacheKeyType = &t
		}
	}
}

// WithLLaMACppCacheValueType sets the cache value type for the estimate.
func WithLLaMACppCacheValueType(t GGMLType) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if slices.Contains(_GGUFEstimateCacheTypeAllowList, t) {
			o.LMCCacheValueType = &t
		}
	}
}

// WithoutLLaMACppOffloadKVCache disables offloading the KV cache.
func WithoutLLaMACppOffloadKVCache() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.LMCOffloadKVCache = ptr.To(false)
	}
}

// WithLLaMACppOffloadLayers sets the number of layers to offload.
func WithLLaMACppOffloadLayers(layers uint64) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.LMCOffloadLayers = &layers
	}
}

// LLaMACppSplitMode is the split mode for LLaMACpp.
type LLaMACppSplitMode uint

const (
	LLaMACppSplitModeLayer LLaMACppSplitMode = iota
	LLaMACppSplitModeRow
	LLaMACppSplitModeNone
	_LLAMACppSplitModeMax
)

// WithLLaMACppSplitMode sets the split mode for the estimate.
func WithLLaMACppSplitMode(mode LLaMACppSplitMode) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if mode < _LLAMACppSplitModeMax {
			o.LMCSplitMode = mode
		}
	}
}

// WithLLaMACppFullSizeSWACache enables full size sliding window attention cache.
func WithLLaMACppFullSizeSWACache() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.LMCFullSizeSWACache = true
	}
}

// WithLLaMACppVisualMaxImageSize sets the visual maximum image size input for the estimate.
func WithLLaMACppVisualMaxImageSize(size uint32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if size == 0 {
			return
		}
		o.LMCVisualMaxImageSize = &size
	}
}

// WithLLaMACppMaxProjectedCache sets the maximum projected embedding cache for the estimate.
func WithLLaMACppMaxProjectedCache(cacheSize uint32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if cacheSize == 0 {
			return
		}
		o.LMCMaxProjectedCache = ptr.To(cacheSize)
	}
}

// WithLLaMACppDrafter sets the drafter estimate usage.
func WithLLaMACppDrafter(dft *LLaMACppRunEstimate) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.LMCDrafter = dft
	}
}

// WithLLaMACppProjector sets the multimodal projector estimate usage.
func WithLLaMACppProjector(prj *LLaMACppRunEstimate) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.LMCProjector = prj
	}
}

// WithLLaMACppAdapters sets the adapters estimate usage.
func WithLLaMACppAdapters(adp []LLaMACppRunEstimate) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if len(adp) == 0 {
			return
		}
		o.LMCAdapters = adp
	}
}

// WithStableDiffusionCppOffloadLayers sets the number of layers to offload.
func WithStableDiffusionCppOffloadLayers(layers uint64) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.SDCOffloadLayers = &layers
	}
}

// WithStableDiffusionCppBatchCount sets the batch count for the estimate.
func WithStableDiffusionCppBatchCount(count int32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if count == 0 {
			return
		}
		o.SDCBatchCount = ptr.To(count)
	}
}

// WithStableDiffusionCppHeight sets the image height for the estimate.
func WithStableDiffusionCppHeight(height uint32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if height == 0 {
			return
		}
		o.SDCHeight = ptr.To(height)
	}
}

// WithStableDiffusionCppWidth sets the image width for the estimate.
func WithStableDiffusionCppWidth(width uint32) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		if width == 0 {
			return
		}
		o.SDCWidth = ptr.To(width)
	}
}

// WithoutStableDiffusionCppOffloadConditioner disables offloading the conditioner(text encoder).
func WithoutStableDiffusionCppOffloadConditioner() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.SDCOffloadConditioner = ptr.To(false)
	}
}

// WithoutStableDiffusionCppOffloadAutoencoder disables offloading the autoencoder.
func WithoutStableDiffusionCppOffloadAutoencoder() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.SDCOffloadAutoencoder = ptr.To(false)
	}
}

// WithStableDiffusionCppAutoencoderTiling enables tiling for the autoencoder.
func WithStableDiffusionCppAutoencoderTiling() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.SDCAutoencoderTiling = ptr.To(true)
	}
}

// WithStableDiffusionCppFreeComputeMemoryImmediately enables freeing compute memory immediately.
func WithStableDiffusionCppFreeComputeMemoryImmediately() GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.SDCFreeComputeMemoryImmediately = ptr.To(true)
	}
}

// WithStableDiffusionCppUpscaler sets the upscaler estimate usage.
func WithStableDiffusionCppUpscaler(ups *StableDiffusionCppRunEstimate) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.SDCUpscaler = ups
	}
}

// WithStableDiffusionCppControlNet sets the control net estimate usage.
func WithStableDiffusionCppControlNet(cn *StableDiffusionCppRunEstimate) GGUFRunEstimateOption {
	return func(o *_GGUFRunEstimateOptions) {
		o.SDCControlNet = cn
	}
}
