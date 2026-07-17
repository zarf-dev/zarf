package gguf_parser

import (
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/gpustack/gguf-parser-go/util/anyx"
	"github.com/gpustack/gguf-parser-go/util/ptr"
	"github.com/gpustack/gguf-parser-go/util/slicex"
)

// Types for LLaMACpp estimation.
type (
	// LLaMACppRunEstimate represents the estimated result of loading the GGUF file in llama.cpp.
	LLaMACppRunEstimate struct {
		// Type describes what type this GGUF file is.
		Type string `json:"type"`
		// Architecture describes what architecture this GGUF file implements.
		//
		// All lowercase ASCII.
		Architecture string `json:"architecture"`
		// ClipProjectorType is the type of the projector used in the clip model.
		//
		// Only used when Architecture is "clip".
		ClipProjectorType string `json:"clipProjectorType,omitempty"`
		// AdapterType is the type of the adapter.
		//
		// Only used when Architecture is "adapter".
		AdapterType string `json:"adapterType,omitempty"`
		// FlashAttention is the flag to indicate whether enable the flash attention,
		// true for enable.
		FlashAttention bool `json:"flashAttention"`
		// ContextSize is the size of the context.
		ContextSize uint64 `json:"contextSize"`
		// OffloadLayers is the number of offloaded layers.
		OffloadLayers uint64 `json:"offloadLayers"`
		// FullOffloaded is the flag to indicate whether the layers are fully offloaded,
		// false for partial offloaded or zero offloaded.
		FullOffloaded bool `json:"fullOffloaded"`
		// NoMMap is the flag to indicate whether support the mmap,
		// true for support.
		NoMMap bool `json:"noMMap"`
		// EmbeddingOnly is the flag to indicate whether the model is used for embedding only,
		// true for embedding only.
		EmbeddingOnly bool `json:"embeddingOnly"`
		// Reranking is the flag to indicate whether the model is used for reranking,
		// true for reranking.
		//
		// Only available when EmbeddingOnly is true.
		Reranking bool `json:"reranking"`
		// Distributable is the flag to indicate whether the model is distributable,
		// true for distributable.
		Distributable bool `json:"distributable"`
		// LogicalBatchSize is the logical batch size.
		LogicalBatchSize int32 `json:"logicalBatchSize"`
		// PhysicalBatchSize is the physical batch size.
		PhysicalBatchSize int32 `json:"physicalBatchSize"`
		// Devices represents the usage for running the GGUF file,
		// the first device is the CPU, and the rest are GPUs.
		Devices []LLaMACppRunDeviceUsage `json:"devices"`
		// Drafter is the estimated result of drafter.
		Drafter *LLaMACppRunEstimate `json:"drafter,omitempty"`
		// Projector is the estimated result of multimodal projector.
		Projector *LLaMACppRunEstimate `json:"projector,omitempty"`
		// Adapters is the estimated result of adapters.
		Adapters []LLaMACppRunEstimate `json:"adapters,omitempty"`
		// MaximumTokensPerSecond represents the maximum tokens per second for running the GGUF file.
		MaximumTokensPerSecond *GGUFTokensPerSecondScalar `json:"maximumTokensPerSecond,omitempty"`
	}

	// LLaMACppRunDeviceUsage represents the usage for running the GGUF file in llama.cpp.
	LLaMACppRunDeviceUsage struct {
		// HandleLayers is the number of layers that the device can handle.
		HandleLayers uint64 `json:"handleLayers"`
		// HandleSWALayers is the number of layers that the device can handle in sliding window attention (SWA),
		// the non SWA layers is `HandleLayers - HandleSWALayers`.
		HandleSWALayers uint64 `json:"handleSWALayers"`
		// HandleLastLayer is the index of the last layer the device can handle,
		// -1 means the device does not handle the last layer.
		HandleLastLayer int `json:"handleLastLayer"`
		// HandleOutputLayer is the flag to indicate whether the device can handle the output layer,
		// true for handle.
		HandleOutputLayer bool `json:"handleOutputLayer"`
		// Remote is the flag to indicate whether the device is remote,
		// true for remote.
		Remote bool `json:"remote"`
		// Position is the relative position of the device,
		// starts from 0.
		//
		// If Remote is true, Position is the position of the remote devices,
		// Otherwise, Position is the position of the device in the local devices.
		Position int `json:"position"`
		// Endpoint is the endpoint of the remote device, empty for local devices.
		Endpoint string `json:"endpoint,omitempty"`
		// Footprint is the memory footprint for bootstrapping.
		Footprint GGUFBytesScalar `json:"footprint"`
		// Parameter is the running parameters that the device processes.
		Parameter LLaMACppParameterUsage `json:"parameter"`
		// Weight is the memory usage of weights that the device loads.
		Weight LLaMACppWeightMemoryUsage `json:"weight"`
		// KVCache is the memory usage of kv that the device caches.
		KVCache LLaMACppKVCacheMemoryUsage `json:"kvCache"`
		// Computation is the memory usage of computation that the device processes.
		Computation LLaMACppComputationMemoryUsage `json:"computation"`
	}

	// LLaMACppParameterUsage represents the parameter usage for running the GGUF file in llama.cpp.
	LLaMACppParameterUsage struct {
		// KVCache is the parameter usage for caching previous KV.
		KVCache GGUFParametersScalar `json:"kvCache"`
		// Input is the parameter usage for input tensors.
		Input GGUFParametersScalar `json:"input"`
		// Compute is the parameter usage for compute tensors.
		Compute GGUFParametersScalar `json:"compute"`
		// ComputeOverridden is the parameter usage for overridden compute tensors.
		ComputeOverridden GGUFParametersScalar `json:"computeOverridden"`
		// Output is the parameter usage for output tensors.
		Output GGUFParametersScalar `json:"output"`
	}

	// LLaMACppWeightMemoryUsage represents the memory usage of loading weights in llama.cpp.
	LLaMACppWeightMemoryUsage struct {
		// Input is the memory usage for loading input tensors.
		Input GGUFBytesScalar `json:"input"`
		// Compute is the memory usage for loading compute tensors.
		Compute GGUFBytesScalar `json:"compute"`
		// ComputeOverridden is the memory usage for loading overridden compute tensors.
		ComputeOverridden GGUFBytesScalar `json:"computeOverridden"`
		// Output is the memory usage for loading output tensors.
		Output GGUFBytesScalar `json:"output"`
	}

	// LLaMACppKVCacheMemoryUsage represents the memory usage of caching previous KV in llama.cpp.
	LLaMACppKVCacheMemoryUsage struct {
		// Key is the memory usage for caching previous keys.
		Key GGUFBytesScalar `json:"key"`
		// Value is the memory usage for caching previous values.
		Value GGUFBytesScalar `json:"value"`
	}

	// LLaMACppComputationMemoryUsage represents the memory usage of computation in llama.cpp.
	LLaMACppComputationMemoryUsage struct {
		// Footprint is the memory footprint for computation.
		Footprint GGUFBytesScalar `json:"footprint"`
		// Input is the memory usage for input.
		Input GGUFBytesScalar `json:"input"`
		// Compute is the memory usage for computation.
		Compute GGUFBytesScalar `json:"graph"`
		// Output is the memory usage for output.
		Output GGUFBytesScalar `json:"output"`
	}
)

// EstimateLLaMACppRun estimates the usages of the GGUF file in llama.cpp.
func (gf *GGUFFile) EstimateLLaMACppRun(opts ...GGUFRunEstimateOption) (e LLaMACppRunEstimate) {
	// Options
	var o _GGUFRunEstimateOptions
	for _, opt := range opts {
		opt(&o)
	}
	switch {
	case o.TensorSplitFraction == nil:
		o.TensorSplitFraction = []float64{1}
		o.MainGPUIndex = 0
	case o.MainGPUIndex < 0 || o.MainGPUIndex >= len(o.TensorSplitFraction):
		panic("main device index must be range of 0 to the length of tensor split fraction")
	}
	if len(o.DeviceMetrics) > 0 {
		for i, j := 0, len(o.DeviceMetrics)-1; i < len(o.TensorSplitFraction)-j; i++ {
			o.DeviceMetrics = append(o.DeviceMetrics, o.DeviceMetrics[j])
		}
		o.DeviceMetrics = o.DeviceMetrics[:len(o.TensorSplitFraction)+1]
	}
	if o.LMCCacheKeyType == nil {
		o.LMCCacheKeyType = ptr.To(GGMLTypeF16)
	}
	if o.LMCCacheValueType == nil {
		o.LMCCacheValueType = ptr.To(GGMLTypeF16)
	}
	if o.LMCOffloadKVCache == nil {
		o.LMCOffloadKVCache = ptr.To(true)
	}
	if o.LMCLogicalBatchSize == nil {
		o.LMCLogicalBatchSize = ptr.To(int32(2048))
	} else {
		// See https://github.com/ggerganov/llama.cpp/blob/0bf16de07b0692e7df26b9a633e232bbd66e0360/src/llama.cpp#L16519-L16525.
		o.LMCLogicalBatchSize = ptr.To(max(32, *o.LMCLogicalBatchSize))
	}
	if o.LMCPhysicalBatchSize == nil {
		o.LMCPhysicalBatchSize = ptr.To(int32(512))
	}
	if *o.LMCPhysicalBatchSize > *o.LMCLogicalBatchSize {
		panic("physical batch size must be less than or equal to logical batch size")
	}
	if o.LMCSplitMode >= _LLAMACppSplitModeMax {
		panic("split mode must be less than max")
	}

	// Devices.
	e.Devices = make([]LLaMACppRunDeviceUsage, len(o.TensorSplitFraction)+1)
	for i := range e.Devices {
		e.Devices[i].HandleLastLayer = -1
	}
	for j := range e.Devices[1:] {
		e.Devices[j+1].Remote = j < len(o.RPCServers)
		if e.Devices[j+1].Remote {
			e.Devices[j+1].Position = j
			e.Devices[j+1].Endpoint = o.RPCServers[j]
		} else {
			e.Devices[j+1].Position = j - len(o.RPCServers)
		}
	}

	// Metadata.
	a := gf.Architecture()
	e.Type = a.Type
	e.Architecture = a.Architecture
	e.ClipProjectorType = a.ClipProjectorType
	e.AdapterType = a.AdapterType

	switch a.Type {
	case "model":
		t := gf.Tokenizer()
		gf.estimateLLaMACppRunInModel(&o, &a, &t, &e)
	case "projector":
		// For projector model,
		// see https://github.com/ggerganov/llama.cpp/blob/148ec970b62c3c5ae0a8bfdaad2fc237aaae350d/examples/llava/clip.cpp#L994-L1008.
		if ptr.Deref(o.LMCOffloadLayers, math.MaxUint64) != 0 {
			// Full offload.
			o.LMCOffloadLayers = ptr.To[uint64](math.MaxUint64)
		} else {
			// Zero offload.
			o.LMCOffloadLayers = ptr.To[uint64](0)
		}
		gf.estimateLLaMACppRunInProjector(&o, &a, &e)
	case "adapter":
		gf.estimateLLaMACppRunInAdapter(&o, &a, &e)
	case "imatrix":
		gf.estimateLLaMACppRunInIMatrix(&o, &a, &e)
	}

	return e
}

// estimateLLaMACppRunInModel estimates the usages of the GGUF file for model,
// including the usages of footprint, weight, KV cache, and computation.
func (gf *GGUFFile) estimateLLaMACppRunInModel(o *_GGUFRunEstimateOptions, a *GGUFArchitecture, t *GGUFTokenizer, e *LLaMACppRunEstimate) {
	ls := gf.Layers()
	ioLs, tfLs, _ := ls.Cut([]string{
		"position_*",
		"token_*",
		"cls.*",
		"output.*",
		"output_*",
		"rope_factors_*",
	})
	ipLs, opLs, _ := ioLs.Cut([]string{
		"position_*",
		"token_*",
	})

	if a.BlockCount == 0 {
		a.BlockCount = uint64(len(tfLs))
	}

	// Using sliding window attention.
	usingSWA := a.AttentionSlidingWindowPattern != 1 && !o.LMCFullSizeSWACache

	// Full offload: nLoadLayers == 0 && isOffloadOutputLayer
	// Zero offload: nOffloadLayers == 0
	// Partial offload: !Full offload && !Zero offload
	var (
		nOffloadLayers       uint64
		nActualOffloadLayers uint64
		nLoadLayers          = a.BlockCount
		idxOutputDevice      int

		fullOffload, zeroOffload          bool
		nSWALoadLayers, nSWAOffloadLayers uint64
	)
	{
		var isOffloadOutputLayer bool

		switch v := o.LMCOffloadLayers; {
		case v == nil:
			o.LMCOffloadLayers = ptr.To(a.BlockCount)
			nOffloadLayers = a.BlockCount
			isOffloadOutputLayer = true
		case *v != 0:
			nOffloadLayers = *v
			if nOffloadLayers > a.BlockCount {
				isOffloadOutputLayer = true
				nOffloadLayers = a.BlockCount
			}
		}
		nActualOffloadLayers = nOffloadLayers
		if isOffloadOutputLayer {
			nActualOffloadLayers += 1
		}
		nLoadLayers -= nOffloadLayers

		fullOffload = nLoadLayers == 0 && isOffloadOutputLayer
		zeroOffload = nOffloadLayers == 0

		e.FullOffloaded = fullOffload
		e.OffloadLayers = nOffloadLayers

		for i, j, offloadStart := uint64(0), 0, a.BlockCount-nOffloadLayers; i < a.BlockCount; i++ {
			switch {
			case i < nLoadLayers:
				e.Devices[0].HandleLayers += 1
				e.Devices[0].HandleLastLayer = int(i)
				if usingSWA && (a.AttentionSlidingWindowPattern == 0 || i%uint64(a.AttentionSlidingWindowPattern) != 0) {
					e.Devices[0].HandleSWALayers += 1
					nSWALoadLayers += 1
				}
			case i >= offloadStart:
				x := float64(i-offloadStart) / float64(nActualOffloadLayers)
				j = slicex.UpperBound(o.TensorSplitFraction, x)
				e.Devices[j+1].HandleLayers += 1
				e.Devices[j+1].HandleLastLayer = int(i)
				if usingSWA && (a.AttentionSlidingWindowPattern == 0 || i%uint64(a.AttentionSlidingWindowPattern) != 0) {
					e.Devices[j+1].HandleSWALayers += 1
					nSWAOffloadLayers += 1
				}
				if fullOffload && i == a.BlockCount-1 {
					idxOutputDevice = j + 1
				}
			}
		}

		e.Devices[idxOutputDevice].HandleOutputLayer = true
	}

	// Flash attention.
	{
		// Grok is not compatible with flash attention,
		// see https://github.com/ggerganov/llama.cpp/blob/19d3c8293b1f61acbe2dab1d49a17950fd788a4a/src/llama.cpp#L9566-L9569.
		if a.Architecture == "grok" {
			o.FlashAttention = false
		}
		// Fallback to FP16 if the value type is quantized when disabling flash attention,
		// see https://github.com/ggerganov/llama.cpp/blob/19d3c8293b1f61acbe2dab1d49a17950fd788a4a/src/llama.cpp#L9576-L9579.
		if o.LMCCacheValueType.IsQuantized() && !o.FlashAttention {
			o.LMCCacheValueType = ptr.To(GGMLTypeF16)
		}

		e.FlashAttention = o.FlashAttention
	}

	// Embedding.
	if !a.AttentionCausal {
		ropeFrequencyBase := ptr.Deref(o.LMCRoPEFrequencyBase, a.RoPEFrequencyBase)
		ropeFrequencyScale := ptr.Deref(o.LMCRoPEFrequencyScale, a.RoPEFrequencyScale)
		ropeScalingType := ptr.Deref(o.LMCRoPEScalingType, a.RoPEScalingType)
		ropeScalingOriginalContextSize := ptr.Deref(o.LMCRoPEScalingOriginalContextSize, int32(a.RoPEScalingOriginalContextLength))
		isRoPECustomized := ropeFrequencyBase != a.RoPEFrequencyBase ||
			ropeFrequencyScale != a.RoPEFrequencyScale ||
			ropeScalingType != a.RoPEScalingType ||
			(ropeScalingType == "yarn" && ropeScalingOriginalContextSize != int32(a.RoPEScalingOriginalContextLength))

		e.EmbeddingOnly = true
		o.LMCContextSize = ptr.To(ptr.Deref(o.LMCContextSize, int32(a.MaximumContextLength)))
		// Set context size/physical batch size/logical batch size to the training context size.
		if !isRoPECustomized {
			o.LMCContextSize = ptr.To(min(int32(a.MaximumContextLength), *o.LMCContextSize))
		}
		o.LMCLogicalBatchSize = o.LMCContextSize
		o.LMCPhysicalBatchSize = o.LMCLogicalBatchSize
		// Reranking.
		if _, found := gf.TensorInfos.Index([]string{"cls.bias", "cls.weight"}); found > 0 {
			e.Reranking = true
		}
		if !e.Reranking && a.PoolingType == 4 { // 0: None, 1: Mean, 2: Cls, 3: Last, 4: Rank
			e.Reranking = true
		}
	}

	// Distributable,
	// fix by https://github.com/ggerganov/llama.cpp/pull/11047.
	e.Distributable = true

	// Batch size.
	e.LogicalBatchSize = *o.LMCLogicalBatchSize
	e.PhysicalBatchSize = *o.LMCPhysicalBatchSize

	// Padding alignment.
	paddingAlign := uint64(32)
	if o.FlashAttention {
		paddingAlign = 256
	}

	// Init hyperparameters,
	// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L6957-L7000.
	var (
		nContext uint64
		nTokens  uint64
		nBatch   uint64
		nOutputs uint64
		nSeq     uint64
		nKV      uint64
	)
	{
		nContext = a.MaximumContextLength
		if o.LMCContextSize != nil {
			nContext = uint64(*o.LMCContextSize)
		}
		if o.LMCInMaxContextSize {
			nContext = min(nContext, a.MaximumContextLength)
		}
		// Padding context size,
		// see https://github.com/ggerganov/llama.cpp/blob/278d0e18469aacf505be18ce790a63c7cc31be26/src/llama.cpp#L19001-L19002.
		nContext = GGMLPadding(nContext, paddingAlign)

		// Correct token size,
		// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L12221-L12224.
		nTokens = min(nContext, uint64(*o.LMCPhysicalBatchSize))
		nBatch = nTokens
		nOutputs = nTokens
		nSeq = uint64(ptr.Deref(o.ParallelSize, 1))
		nKV = nContext

		e.ContextSize = nContext
	}

	// Footprint.
	{
		// Bootstrap.
		e.Devices[0].Footprint = GGUFBytesScalar(5*1024*1024) /* model load */ + (gf.Size - gf.ModelSize) /* metadata */

		// Tokens,
		// https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L6380-L6384.
		fp := t.TokensLength * (4 /* token type */ + 4 /* token score*/)
		if t.Model == "gpt2" {
			fp += t.MergesLength * (48 /* key type */ + 56 /* value type */)
		}
		fp += t.TokensLength * (32 /* id to token vector */ + (24 + 32) /* token to id map*/)
		e.Devices[0].Footprint += GGUFBytesScalar(fp)

		// Output buffer,
		// see https://github.com/ggerganov/llama.cpp/blob/7672adeec7a79ea271058c63106c142ba84f951a/llama.cpp#L11940-L12003.
		ob := a.EmbeddingLength * nOutputs * 4 /* float32 size */
		if a.AttentionCausal {
			ob += a.VocabularyLength * nOutputs * 4 /* float32 size */
		}
		if fullOffload {
			e.Devices[idxOutputDevice].Footprint += GGUFBytesScalar(ob)
		} else {
			e.Devices[0].Footprint += GGUFBytesScalar(ob)
		}
	}

	// Weight & Parameter.
	{
		filter := func(idx int) GGUFTensorInfoFilter {
			if len(o.OverriddenTensors) == 0 {
				return nil
			}
			return func(name string) bool {
				for _, ot := range o.OverriddenTensors {
					bt, bi := ot.ParseBufferType()
					switch {
					case bt == GGUFRunOverriddenTensorBufferTypeUnknown:
						continue
					case bt == GGUFRunOverriddenTensorBufferTypeCPU && idx == 0:
						continue
					case bt == GGUFRunOverriddenTensorBufferTypeGPU &&
						(e.Devices[idx].Remote || anyx.Number[int](bi)+1 != idx):
						continue
					case bt == GGUFRunOverriddenTensorBufferTypeRPC &&
						(!e.Devices[idx].Remote || e.Devices[idx].Endpoint != bi):
						continue
					}
					if ot.PatternRegex.MatchString(name) {
						return false
					}
				}
				return true
			}
		}

		// If overridden tensors are provided,
		// we need to search the tensors of the overridden pattern,
		// and place them in the correct device.
		if len(o.OverriddenTensors) != 0 {
			for _, ot := range o.OverriddenTensors {
				bt, bi := ot.ParseBufferType()
				if bt == GGUFRunOverriddenTensorBufferTypeUnknown {
					continue
				}
				var sls GGUFTensorInfos = ls.Search(ot.PatternRegex)
				if len(sls) == 0 {
					continue
				}
				switch bt {
				case GGUFRunOverriddenTensorBufferTypeCPU:
					e.Devices[0].Weight.ComputeOverridden += GGUFBytesScalar(sls.Bytes())
					e.Devices[0].Parameter.ComputeOverridden += GGUFParametersScalar(sls.Elements())
				case GGUFRunOverriddenTensorBufferTypeGPU:
					idx := anyx.Number[int](bi) + 1
					e.Devices[idx].Weight.ComputeOverridden += GGUFBytesScalar(sls.Bytes())
					e.Devices[idx].Parameter.ComputeOverridden += GGUFParametersScalar(sls.Elements())
				default:
					for i, d := range e.Devices[1:] {
						if d.Endpoint == bi {
							e.Devices[i+1].Weight.ComputeOverridden += GGUFBytesScalar(sls.Bytes())
							e.Devices[i+1].Parameter.ComputeOverridden += GGUFParametersScalar(sls.Elements())
							break
						}
					}
				}
			}
		}

		// Compute.
		for i, j, offloadStart := 0, 0, len(tfLs)-int(nOffloadLayers); i < len(tfLs); i++ {
			idx := 0
			if i >= offloadStart {
				x := float64(i-offloadStart) / float64(nActualOffloadLayers)
				j = slicex.UpperBound(o.TensorSplitFraction, x)
				idx = j + 1
			}
			f := filter(idx)
			e.Devices[idx].Weight.Compute += GGUFBytesScalar(tfLs[i].Bytes(f))
			e.Devices[idx].Parameter.Compute += GGUFParametersScalar(tfLs[i].Elements(f))
		}

		// IO,
		// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L4930-L5002.
		e.Devices[0].Weight.Input = GGUFBytesScalar(ipLs.Bytes())
		e.Devices[0].Parameter.Input = GGUFParametersScalar(ipLs.Elements())
		var (
			wg GGUFBytesScalar
			ps GGUFParametersScalar
		)
		if _, ok := opLs.Get("output.weight"); ok {
			wg = GGUFBytesScalar(opLs.Bytes())
			ps = GGUFParametersScalar(opLs.Elements())
		} else {
			wg = GGUFBytesScalar(opLs.Bytes()) + e.Devices[0].Weight.Input /* duplicate the input layer */
			ps = GGUFParametersScalar(opLs.Elements() + ipLs.Elements())
		}
		e.Devices[0].Weight.Output = wg
		if fullOffload {
			e.Devices[idxOutputDevice].Weight.Output = wg
			e.Devices[idxOutputDevice].Parameter.Output = ps
		} else {
			e.Devices[0].Parameter.Output = ps
		}
	}

	// KV cache.
	if a.AttentionCausal {
		switch {
		// Recurrent,
		// see https://github.com/ggml-org/llama.cpp/blob/704bb7a71c01dc07c1478b85f6322bf5dfde1eaf/src/llama-hparams.cpp#L68-L88.
		case a.AttentionRecurrent:
			var r, s uint64
			if a.RWKVHeadSize > 0 {
				r = uint64(a.RWKVTokenShiftCount) * a.EmbeddingLength
				s = uint64(a.RWKVHeadSize) * a.EmbeddingLength
			} else {
				r = uint64((a.SSMConvolutionKernel - 1) * (a.SSMInnerSize + 2*a.SSMGroupCount*a.SSMStateSize))
				s = uint64(a.SSMStateSize * a.SSMInnerSize)
			}

			rps, sps := r*nSeq, s*nSeq
			rrs, srs := GGMLTypeF32.RowSizeOf([]uint64{rps}), GGMLTypeF32.RowSizeOf([]uint64{sps})

			e.Devices[0].KVCache.Key += GGUFBytesScalar(rrs * nLoadLayers)
			e.Devices[0].KVCache.Value += GGUFBytesScalar(srs * nLoadLayers)
			e.Devices[0].Parameter.KVCache += GGUFParametersScalar((rrs + srs) * nLoadLayers)
			if !*o.LMCOffloadKVCache {
				e.Devices[0].KVCache.Key += GGUFBytesScalar(rrs * nOffloadLayers)
				e.Devices[0].KVCache.Value += GGUFBytesScalar(srs * nOffloadLayers)
				e.Devices[0].Parameter.KVCache += GGUFParametersScalar((rrs + srs) * nOffloadLayers)
			} else if !zeroOffload {
				for i, d := range e.Devices[1:] {
					e.Devices[i+1].KVCache.Key += GGUFBytesScalar(rrs * d.HandleLayers)
					e.Devices[i+1].KVCache.Value += GGUFBytesScalar(srs * d.HandleLayers)
					e.Devices[i+1].Parameter.KVCache += GGUFParametersScalar((rrs + srs) * d.HandleLayers)
				}
			}

			if !a.AttentionHybrid {
				break
			}

			fallthrough
		// Causal,
		// see https://github.com/ggml-org/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L2479-L2501.
		default:
			akl, avl := uint64(a.AttentionKeyLength), uint64(a.AttentionValueLength)
			if a.AttentionKeyLengthMLA > 0 && a.AttentionValueLengthMLA > 0 {
				akl, avl = uint64(a.AttentionKeyLengthMLA), uint64(a.AttentionValueLengthMLA)
			}
			kGQA := akl * a.AttentionHeadCountKV
			vGQA := avl * a.AttentionHeadCountKV
			kps, vps := kGQA*nKV, vGQA*nKV
			krs, vrs := o.LMCCacheKeyType.RowSizeOf([]uint64{kps}), o.LMCCacheValueType.RowSizeOf([]uint64{vps})

			if !usingSWA {
				e.Devices[0].KVCache.Key += GGUFBytesScalar(krs * nLoadLayers)
				e.Devices[0].KVCache.Value += GGUFBytesScalar(vrs * nLoadLayers)
				e.Devices[0].Parameter.KVCache += GGUFParametersScalar((kps + vps) * nLoadLayers)
				if !*o.LMCOffloadKVCache {
					e.Devices[0].KVCache.Key += GGUFBytesScalar(krs * nOffloadLayers)
					e.Devices[0].KVCache.Value += GGUFBytesScalar(vrs * nOffloadLayers)
					e.Devices[0].Parameter.KVCache += GGUFParametersScalar((kps + vps) * nOffloadLayers)
				} else if !zeroOffload {
					for i, d := range e.Devices[1:] {
						e.Devices[i+1].KVCache.Key += GGUFBytesScalar(krs * d.HandleLayers)
						e.Devices[i+1].KVCache.Value += GGUFBytesScalar(vrs * d.HandleLayers)
						e.Devices[i+1].Parameter.KVCache += GGUFParametersScalar((kps + vps) * d.HandleLayers)
					}
				}
			} else {
				// Sliding window attention size,
				// see https://github.com/ggml-org/llama.cpp/blob/3079e9ac8e04ef6eddeb0c164d72edb6b6fd2df5/src/llama-kv-cache.cpp#L1640-L1642.
				swas := min(nKV, GGMLPadding(a.AttentionSlidingWindow*nSeq+uint64(*o.LMCLogicalBatchSize), paddingAlign))
				swaKps, swaVps := kGQA*swas, vGQA*swas
				swaKrs, swaVrs := o.LMCCacheKeyType.RowSizeOf([]uint64{swaKps}), o.LMCCacheValueType.RowSizeOf([]uint64{swaVps})

				nNonSWALoadLayers, nNonSWAOffloadLayers := nLoadLayers-nSWALoadLayers, nOffloadLayers-nSWAOffloadLayers

				e.Devices[0].KVCache.Key += GGUFBytesScalar(swaKrs*nSWALoadLayers + krs*nNonSWALoadLayers)
				e.Devices[0].KVCache.Value += GGUFBytesScalar(swaVrs*nSWALoadLayers + vrs*nNonSWALoadLayers)
				e.Devices[0].Parameter.KVCache += GGUFParametersScalar((swaKps+swaVps)*nSWALoadLayers + (kps+vps)*nNonSWALoadLayers)
				if !*o.LMCOffloadKVCache {
					e.Devices[0].KVCache.Key += GGUFBytesScalar(swaKrs*nSWAOffloadLayers + krs*nNonSWAOffloadLayers)
					e.Devices[0].KVCache.Value += GGUFBytesScalar(swaVrs*nSWAOffloadLayers + vrs*nNonSWAOffloadLayers)
					e.Devices[0].Parameter.KVCache += GGUFParametersScalar((swaKps+swaVps)*nSWAOffloadLayers + (kps+vps)*nNonSWAOffloadLayers)
				} else if !zeroOffload {
					for i, d := range e.Devices[1:] {
						e.Devices[i+1].KVCache.Key += GGUFBytesScalar(swaKrs*d.HandleSWALayers + krs*(d.HandleLayers-d.HandleSWALayers))
						e.Devices[i+1].KVCache.Value += GGUFBytesScalar(swaVrs*d.HandleSWALayers + vrs*(d.HandleLayers-d.HandleSWALayers))
						e.Devices[i+1].Parameter.KVCache += GGUFParametersScalar((swaKps+swaVps)*d.HandleSWALayers + (kps+vps)*(d.HandleLayers-d.HandleSWALayers))
					}
				}
			}
		}
	}

	// Computation.
	{
		// See https://github.com/ggml-org/llama.cpp/blob/ec9e0301fef6476df83e94842c3b625501c95566/src/llama-context.cpp#L1241-L1243.
		maxNodes := max(1024, uint64(8*len(gf.TensorInfos)))

		// Bootstrap, compute metadata.
		cm := GGMLTensorOverhead()*maxNodes + GGMLComputationGraphOverhead(maxNodes, false)
		e.Devices[0].Computation.Footprint = GGUFBytesScalar(cm)

		// Scheduler overhead,
		// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L16149.
		e.Devices[0].Computation.Footprint += GGUFBytesScalar(4 * 1024 * 1024)

		// GGML context,
		// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L5015-L5036.
		gc := 2 /* buffer count */ * GGMLTensorOverhead() * (uint64(len(gf.TensorInfos)) + 1 + a.BlockCount*3)
		e.Devices[0].Computation.Footprint += GGUFBytesScalar(gc)

		// Tensor usage,
		// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L16149.
		//
		// First, get the usage of input layer,
		// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L2279-L2290.
		var (
			inpTokens = GGMLTypeI32.RowSizeOf([]uint64{nBatch})                    // I32 [n_batch]
			inpEmbd   = GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, nBatch}) // F32 [n_embd, n_batch]
			inpPos    = GGMLTypeI32.RowSizeOf([]uint64{nBatch})                    // I32 [n_batch]
			inpOutIds = GGMLTypeI32.RowSizeOf([]uint64{nOutputs})                  // I32 [n_outputs],
			inpKQMask = GGMLTypeF32.RowSizeOf([]uint64{nKV, nBatch})               // F32 [n_kv, n_batch]
			inpSMask  = GGMLTypeF32.RowSizeOf([]uint64{1, nSeq})                   // F32 [1, n_seq]
			inpSSeq   = GGMLTypeI32.RowSizeOf([]uint64{nSeq, nBatch})              // I32 [n_seq, n_batch]
		)
		if a.AttentionRecurrent {
			e.Devices[0].Computation.Input = GGUFBytesScalar(inpTokens + inpEmbd + 2*inpSMask + inpSSeq + inpOutIds)
		} else {
			e.Devices[0].Computation.Input = GGUFBytesScalar(inpTokens + inpEmbd + inpPos + inpKQMask + inpOutIds)
		}
		{
			var v GGUFBytesScalar
			if a.AttentionRecurrent {
				v = GGUFBytesScalar(inpEmbd + inpSMask + inpSSeq)
			} else {
				v = GGUFBytesScalar(inpEmbd + inpPos + inpKQMask)
			}
			if len(o.RPCServers) == 0 && len(o.TensorSplitFraction) > 1 {
				if a.ExpertCount > 0 {
					v *= 2
				} else {
					v *= 4
				}
			}
			for i := range e.Devices[1:] {
				e.Devices[i+1].Computation.Input += v
			}
		}
		// Since the steps between transformer layers are serial,
		// the allocated memory can be reused for the next layer.
		// So, we only consider the usage of the largest layer,
		// which is the last layer by default.
		if a.AttentionRecurrent && !a.AttentionHybrid {
			if a.RWKVHeadSize > 0 {
				attnInc := uint64(0)
				for _, l := range tfLs[len(tfLs)-1].Search(regexp.MustCompile(`.*\.\d+\.(attn_norm|attn_norm_2)\.weight`)) {
					rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nBatch})
					attnInc += rs
				}
				ffnInc := uint64(0)
				for _, l := range tfLs[len(tfLs)-1].Search(regexp.MustCompile(`.*\.\d+\.time_mix_(lerp_x|receptance|decay_w2|key|value|gate|w2|output)\.weight`)) { // nolint: lll
					switch {
					case strings.HasSuffix(l.Name, ".time_mix_w2.weight"):
						rs := GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, 1, nTokens, l.Dimensions[l.NDimensions-1]})
						ffnInc += rs
					case strings.HasSuffix(l.Name, ".time_mix_output.weight"):
						rs := GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, nBatch + uint64(a.RWKVHeadSize)*nSeq})
						ffnInc += rs
					default:
						rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nBatch})
						ffnInc += rs
					}
				}
				cp := GGUFBytesScalar(attnInc + ffnInc)
				for i := range e.Devices[1:] {
					e.Devices[i+1].Computation.Compute = cp
				}
			} else {
				r := uint64((a.SSMConvolutionKernel - 1) * (a.SSMInnerSize + 2*a.SSMGroupCount*a.SSMStateSize))
				convInc := GGMLTypeF32.RowSizeOf([]uint64{r, nSeq}) // F32 [n_embd_key_gqa, nSeq] reshape
				for _, l := range tfLs[len(tfLs)-1].Search(regexp.MustCompile(`.*\.\d+\.(attn_norm|ssm_in|ssm_conv1d)\.weight`)) {
					if !strings.HasSuffix(l.Name, ".ssm_conv1d.weight") {
						rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
						convInc += rs
						continue
					}
					// https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L10379.
					rs := GGMLTypeF32.RowSizeOf([]uint64{uint64(a.SSMInnerSize)*nTokens + uint64(a.SSMConvolutionKernel)*uint64(a.SSMInnerSize)*nSeq})
					convInc += rs
				}
				ssmInc := uint64(0)
				for _, l := range tfLs[len(tfLs)-1].Search(regexp.MustCompile(`.*\.\d+\.ssm_(dt\.weight|a)`)) {
					if !strings.HasSuffix(l.Name, ".ssm_a") {
						rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
						ssmInc += rs
						continue
					}
					// https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L10413.
					rs := GGMLTypeF32.RowSizeOf([]uint64{uint64(a.SSMInnerSize)*nTokens + uint64(a.SSMStateSize)*uint64(a.SSMInnerSize)*nSeq})
					ssmInc += rs
				}
				cp := GGUFBytesScalar(convInc + ssmInc)
				for i := range e.Devices[1:] {
					e.Devices[i+1].Computation.Compute = cp
				}
			}
		} else {
			loadAttnInc, offloadAttnInc := uint64(0), uint64(0)
			{
				rs := o.LMCCacheKeyType.RowSizeOf([]uint64{uint64(a.AttentionKeyLength), nKV, a.AttentionHeadCountKV})
				loadAttnInc = rs // k-?
				rs = o.LMCCacheValueType.RowSizeOf([]uint64{uint64(a.AttentionValueLength), nKV, a.AttentionHeadCountKV})
				loadAttnInc += rs // v-?
			}
			if o.FlashAttention {
				// https://github.com/ggerganov/llama.cpp/blob/172c8256840ffd882ab9992ecedbb587d9b21f15/llama.cpp#L7387.
				offloadAttnInc = GGMLTypeF16.RowSizeOf([]uint64{nKV, nTokens})
				for _, l := range tfLs[len(tfLs)-1].Search(regexp.MustCompile(`.*\.\d+\.attn_(norm|q|qkv|q_b)\.weight`)) {
					if strings.HasSuffix(l.Name, ".attn_norm.weight") {
						rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
						offloadAttnInc += rs
						continue
					}
					rs := l.Bytes()
					offloadAttnInc += rs
				}
				// https://github.com/ggerganov/llama.cpp/blob/172c8256840ffd882ab9992ecedbb587d9b21f15/llama.cpp#L6986-L6992.
				rs := o.LMCCacheKeyType.RowSizeOf([]uint64{uint64(a.AttentionKeyLength), nKV, a.AttentionHeadCountKV})
				offloadAttnInc += rs
				// https://github.com/ggerganov/llama.cpp/blob/172c8256840ffd882ab9992ecedbb587d9b21f15/llama.cpp#L7000-L7007.
				rs = o.LMCCacheValueType.RowSizeOf([]uint64{uint64(a.AttentionValueLength), nKV, a.AttentionHeadCountKV})
				offloadAttnInc += rs
			} else {
				offloadAttnInc = uint64(0)
				for _, l := range tfLs[len(tfLs)-1].Search(regexp.MustCompile(`.*\.\d+\.attn_(norm|q|qkv|q_b)\.weight`)) {
					var rs uint64
					switch {
					default: // norm.
						rs = GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
						offloadAttnInc += rs
					case strings.HasSuffix(l.Name, ".attn_q.weight"):
						rs = GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[0], nTokens})
						offloadAttnInc += rs * 2 // Qcur.
						rs = GGMLTypeF32.RowSizeOf([]uint64{nKV, nTokens, a.AttentionHeadCount})
						offloadAttnInc += rs // kq.
						if !zeroOffload && !fullOffload {
							offloadAttnInc += loadAttnInc
						}
					case strings.HasSuffix(l.Name, ".attn_qkv.weight"):
						rs = GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[0], nTokens})
						offloadAttnInc += rs * 2 // Qcur.
						rs = GGMLTypeF32.RowSizeOf([]uint64{nKV, nTokens, a.AttentionHeadCount})
						offloadAttnInc += rs // kq.
						rs = GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, a.EmbeddingLength * 3})
						offloadAttnInc += rs // wqkv.
						if !zeroOffload && !fullOffload {
							offloadAttnInc += loadAttnInc
						}
					case strings.HasSuffix(l.Name, ".attn_q_b.weight"):
						rs = GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
						offloadAttnInc += rs * 2 // q-?
						rs = GGMLTypeF32.RowSizeOf([]uint64{nKV, nTokens, a.AttentionHeadCount})
						offloadAttnInc += rs // kq.
					}
				}
			}
			ffnInc := uint64(0)
			for _, l := range tfLs[len(tfLs)-1].Search(regexp.MustCompile(`.*\.\d+\.(attn_norm|ffn_norm|ffn_gate|ffn_up)\.weight`)) {
				rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
				ffnInc += rs
			}
			if a.ExpertCount > 0 || a.ExpertUsedCount > 0 {
				rs := GGMLTypeF32.RowSizeOf([]uint64{uint64(a.ExpertCount), a.EmbeddingLength})
				ffnInc += rs // ffn_gate_input
				rs = GGMLTypeF32.RowSizeOf([]uint64{uint64(a.ExpertCount), nTokens})
				ffnInc += rs // ffn_moe_logits
				rs = GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, uint64(a.ExpertUsedCount), nTokens})
				ffnInc += rs // ffn_moe_down
			}
			if !zeroOffload {
				e.Devices[0].Computation.Compute = GGUFBytesScalar(loadAttnInc + ffnInc)
			} else {
				e.Devices[0].Computation.Compute = GGUFBytesScalar(loadAttnInc)
			}
			{
				cp := GGUFBytesScalar(max(offloadAttnInc, ffnInc))
				for i := range e.Devices[1:] {
					e.Devices[i+1].Computation.Compute = cp
				}
				if nLoadLayers > 1 {
					for i := range e.Devices[1:] {
						if e.Devices[i+1].Remote {
							continue
						}
						e.Devices[i+1].Computation.Compute += GGUFBytesScalar(loadAttnInc)
						break
					}
				}
			}
		}
		// Finally, get the usage of output layer.
		if a.AttentionCausal {
			var outInc uint64
			if a.AttentionRecurrent {
				outInc += inpSMask + inpSSeq
			}
			if l, ok := opLs.Get("output_norm.weight"); ok {
				rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
				outInc += rs
			}
			if l, ok := opLs.Get("output.weight"); ok {
				rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
				outInc += rs
			} else if l, ok := ipLs.Get("token_embd.weight"); ok {
				rs := GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
				outInc += rs
			}
			e.Devices[idxOutputDevice].Computation.Output += GGUFBytesScalar(outInc)
		}
	}

	// Drafter.
	e.Drafter = o.LMCDrafter

	// Projector.
	e.Projector = o.LMCProjector

	// Adapters.
	e.Adapters = o.LMCAdapters

	// Maximum tokens per second.
	if ds, dmss := e.Devices, o.DeviceMetrics; len(dmss) != 0 {
		ltss := make([]float64, len(dmss))
		bs := anyx.Number[float64](*o.LMCLogicalBatchSize) / float64(nBatch)
		for i, dm := range dmss {
			fl, upbw, dwbw := float64(max(dm.FLOPS, 1)), float64(max(dm.UpBandwidth, 1)), float64(max(dm.DownBandwidth, 1))
			cmpops := float64(ds[i].Parameter.Compute+ds[i].Parameter.ComputeOverridden)*2 /* FMA */ *bs + float64(ds[i].Parameter.Input) + float64(ds[i].Parameter.Output) // nolint: lll
			cmps := float64(ds[i].Weight.Sum())
			cmplat := max(cmpops/fl, cmps/upbw)
			kvcops := float64(ds[i].Parameter.KVCache) * 2 /* FMA */ * bs
			kvcs := float64(ds[i].KVCache.Sum()) * bs
			kvclat := max(kvcops/fl, kvcs/upbw)
			ffs := float64(GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, nBatch}))
			ffslat := ffs / dwbw
			lays := float64(ds[i].HandleLayers)
			if ds[i].HandleOutputLayer {
				lays += 1
			}
			ltss[i] = (cmplat + kvclat + ffslat) * lays / float64(a.BlockCount+2)
		}
		lt := float64(0)
		ltmax := slices.Max(ltss)
		for i := range ltss {
			lt += ltss[i] / ltmax * ltss[i]
		}
		e.MaximumTokensPerSecond = ptr.To(GGUFTokensPerSecondScalar(1 / lt))
	}
}

// estimateLLaMACppRunInProjector estimates the usages of the GGUF file for projector.
func (gf *GGUFFile) estimateLLaMACppRunInProjector(o *_GGUFRunEstimateOptions, a *GGUFArchitecture, e *LLaMACppRunEstimate) {
	ls := gf.Layers()
	ioLs, tfLs, _ := ls.Cut([]string{
		"mm.*",
		// Vision specific IO layers.
		"v.patch_embd.*",
		"v.class_embd",
		"v.position_embd.*",
		"v.pre_ln.*",
		"v.post_ln.*",
		"model.*",
		"resampler.*",
		// Audio specific IO layers.
		"a.position_embd.*",
		"a.conv1d.*",
		"a.post_ln.*",
	})
	ipLs, opLs, _ := ioLs.Cut([]string{
		// Vision specific Input layers.
		"v.patch_embd.*",
		"v.class_embd",
		"v.position_embd.*",
		"v.pre_ln.*",
		"model.*",
		// Audio specific Input layers.
		"a.position_embd.*",
		"a.conv1d.*",
	})

	// Block count.
	if a.ClipHasVisionEncoder && a.ClipVisionBlockCount == 0 {
		if len(tfLs) == 1 {
			if ntfLs, ok := tfLs[0].(*GGUFNamedTensorInfos); ok && slices.Contains([]string{"v"}, ntfLs.Name) {
				a.ClipVisionBlockCount = uint64(len(ntfLs.GGUFLayerTensorInfos))
			}
		}
		if a.ClipVisionBlockCount == 0 {
			a.ClipVisionBlockCount = uint64(len(tfLs))
		}
	}
	if a.ClipHasAudioEncoder && a.ClipAudioBlockCount == 0 {
		if len(tfLs) == 1 {
			if ntfLs, ok := tfLs[0].(*GGUFNamedTensorInfos); ok && slices.Contains([]string{"a"}, ntfLs.Name) {
				a.ClipAudioBlockCount = uint64(len(ntfLs.GGUFLayerTensorInfos))
			}
		}
		if a.ClipAudioBlockCount == 0 {
			a.ClipAudioBlockCount = uint64(len(tfLs))
		}
	}

	// Offload layers.
	if *o.LMCOffloadLayers == math.MaxUint64 {
		e.FullOffloaded = true
		e.OffloadLayers = a.ClipVisionBlockCount + a.ClipAudioBlockCount
		o.LMCOffloadLayers = ptr.To(e.OffloadLayers)
	} else {
		e.FullOffloaded = false
		e.OffloadLayers = 0
	}

	// Footprint.
	{
		// Bootstrap.
		e.Devices[0].Footprint = GGUFBytesScalar(5*1024*1024) /* model load */ + (gf.Size - gf.ModelSize) /* metadata */
	}

	idx := 0 // Default to the main host's RAM.
	if e.FullOffloaded {
		for i := 1; i < len(e.Devices); i++ {
			if !e.Devices[i].Remote {
				idx = i
				break
			}
		}
	}

	// Weight & Parameter.
	{
		// Compute.
		e.Devices[idx].HandleLayers = *o.LMCOffloadLayers
		e.Devices[idx].HandleLastLayer = int(e.Devices[idx].HandleLayers - 1)
		e.Devices[idx].Weight.Compute = GGUFBytesScalar(tfLs.Bytes())
		e.Devices[idx].Parameter.Compute = GGUFParametersScalar(tfLs.Elements())

		// IO.
		e.Devices[idx].Weight.Input = GGUFBytesScalar(ipLs.Bytes())
		e.Devices[idx].Parameter.Input = GGUFParametersScalar(ipLs.Elements())
		e.Devices[idx].Weight.Output = GGUFBytesScalar(opLs.Bytes())
		e.Devices[idx].Parameter.Output = GGUFParametersScalar(opLs.Elements())
	}

	if a.ClipHasVisionEncoder {
		// Init hyperparameters,
		// see https://github.com/ggerganov/llama.cpp/blob/0827b2c1da299805288abbd556d869318f2b121e/examples/llava/clip.cpp#L599-L636.
		var (
			heightMaxSize uint64 // y
			widthMaxSize  uint64 // x
			// See https://github.com/ggml-org/llama.cpp/blob/6385b843a8dc8e15b8362196039720c58dd79fa2/tools/mtmd/clip.cpp#L3462.
			nPatches       uint64
			patchesMaxSize uint64
			// See https://github.com/ggml-org/llama.cpp/blob/6385b843a8dc8e15b8362196039720c58dd79fa2/tools/mtmd/clip.cpp#L4016.
			projectionDim uint64 // NB(thxCode): do not sure if there is the correct name.
		)
		// See https://github.com/ggerganov/llama.cpp/blob/0827b2c1da299805288abbd556d869318f2b121e/examples/llava/llava.cpp#L397-L411,
		//     https://github.com/ggerganov/llama.cpp/blob/0827b2c1da299805288abbd556d869318f2b121e/examples/llava/clip.cpp#L2323-L2345,
		//     https://github.com/ggerganov/llama.cpp/blob/0827b2c1da299805288abbd556d869318f2b121e/examples/llava/clip.cpp#L2767-L2794.
		heightMaxSize = uint64(a.ClipVisionImageSize)
		widthMaxSize = heightMaxSize
		if a.ClipHasQwen2VLMerger ||
			a.ClipProjectorType == "qwen2vl_merger" ||
			a.ClipProjectorType == "qwen2.5vl_merger" ||
			a.ClipProjectorType == "qwen2.5o" ||
			a.ClipProjectorType == "pixtral" {
			// See https://github.com/ggml-org/llama.cpp/blob/ec9e0301fef6476df83e94842c3b625501c95566/tools/mtmd/clip.cpp#L2217.
			heightMaxSize = uint64(ptr.Deref(o.LMCVisualMaxImageSize, 1024))
			widthMaxSize = heightMaxSize
		}
		nPatchSize := uint64(a.ClipVisionPatchSize)
		nPatchesHeight := heightMaxSize / nPatchSize
		nPatchesWidth := widthMaxSize / nPatchSize
		nPatches = nPatchesHeight * nPatchesWidth
		patchesMaxSize = 1
		switch {
		case a.ClipHasLLaVAProjector ||
			a.ClipProjectorType == "mlp" ||
			a.ClipProjectorType == "mlp_norm" ||
			a.ClipProjectorType == "ldp" ||
			a.ClipProjectorType == "ldpv2":
			// LLaVA 1.6 uses up to 6 patches
			if a.ClipVisionMMPatchMergeType != "flat" {
				patchesMaxSize = 6
			}
		case a.ClipHasMiniCPMVProjector ||
			a.ClipProjectorType == "resampler":
			// MiniCPM-V uses up to 10 patches
			patchesMaxSize = 10
		case a.ClipProjectorType == "adapter":
			// Granite vision uses up to 10 patches + base patch
			patchesMaxSize = 11
		}

		if o.LMCMaxProjectedCache != nil {
			patchesMaxSize += uint64(*o.LMCMaxProjectedCache)
		}

		switch a.ClipProjectorType {
		case "ldp":
			nPatches /= 4
			if ti, ok := gf.TensorInfos.Get("mm.model.mb_block.1.block.2.1.bias"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "ldpv2":
			nPatches /= 4
			if ti, ok := gf.TensorInfos.Get("mm.model.peg.0.bias"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "mlp":
			if ti, ok := gf.TensorInfos.Get("mm.2.bias"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "mlp_norm":
			if ti, ok := gf.TensorInfos.Get("mm.3.bias"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "resampler":
			if ti, ok := gf.TensorInfos.Get("resampler.query"); ok {
				nPatches = ti.Dimensions[1]
				projectionDim = ti.Dimensions[0]
			}
		case "adapter":
			nPatches /= 4
			nPatches += 2
			if ti, ok := gf.TensorInfos.Get("adapter.linear.dense_4h_to_h.weight"); ok {
				projectionDim = ti.Dimensions[1]
			}
		case "qwen2vl_merger", "qwen2.5vl_merger", "qwen2.5o":
			nSizePatch := uint64(a.ClipVisionPatchSize * 2)
			heightPatchSize := heightMaxSize / nSizePatch
			if heightMaxSize%nSizePatch > 0 {
				heightPatchSize++
			}
			widthPatchSize := widthMaxSize / nSizePatch
			if widthMaxSize%nSizePatch > 0 {
				widthPatchSize++
			}
			nPatches = heightPatchSize * widthPatchSize
			if ti, ok := gf.TensorInfos.Get("mm.2.bias"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "gemma3":
			nPerSide := uint64(a.ClipVisionImageSize) / uint64(a.ClipVisionPatchSize)
			nPerSide2DPool := nPerSide / uint64(a.ClipVisionProjectorScaleFactor)
			nPatches = nPerSide2DPool * nPerSide2DPool
			if ti, ok := gf.TensorInfos.Get("mm.input_projection.weight"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "idefics3", "llama4":
			nPatches /= uint64(a.ClipVisionProjectorScaleFactor * a.ClipVisionProjectorScaleFactor)
			if ti, ok := gf.TensorInfos.Get("mm.model.fc.weight"); ok {
				projectionDim = ti.Dimensions[1]
			}
		case "pixtral":
			heightPatchSize := heightMaxSize / uint64(a.ClipVisionPatchSize)
			if a.ClipVisionSpatialMergeSize > 0 {
				heightPatchSize /= uint64(a.ClipVisionSpatialMergeSize)
			}
			widthPatchSize := widthMaxSize / uint64(a.ClipVisionPatchSize)
			if a.ClipVisionSpatialMergeSize > 0 {
				widthPatchSize /= uint64(a.ClipVisionSpatialMergeSize)
			}
			nPatches = heightPatchSize*widthPatchSize + heightPatchSize - 1 /* [IMG_BREAK] per row */
			if ti, ok := gf.TensorInfos.Get("mm.2.bias"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "internvl":
			nPatches /= uint64(a.ClipVisionProjectorScaleFactor * a.ClipVisionProjectorScaleFactor)
			if ti, ok := gf.TensorInfos.Get("mm.model.mlp.3.weight"); ok {
				projectionDim = ti.Dimensions[1]
			}
		}

		// Footprint
		{
			// Image Embed,
			// see https://github.com/ggerganov/llama.cpp/blob/0827b2c1da299805288abbd556d869318f2b121e/examples/llava/llava.cpp#L401-L407.
			e.Devices[0].Footprint += GGUFBytesScalar(patchesMaxSize * nPatches * projectionDim * 4 /* float32 size */)
		}

		// Computation.
		{
			// See https://github.com/ggml-org/llama.cpp/blob/ec9e0301fef6476df83e94842c3b625501c95566/tools/mtmd/clip.cpp#L374.
			var maxNodes uint64 = 8192

			// Bootstrap, compute metadata.
			cm := GGMLTensorOverhead()*maxNodes + GGMLComputationGraphOverhead(maxNodes, false)
			e.Devices[0].Computation.Footprint += GGUFBytesScalar(cm)

			// Scheduler overhead,
			// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L16149.
			e.Devices[0].Computation.Footprint += GGUFBytesScalar(4 * 1024 * 1024)

			// GGML context,
			// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L5015-L5036.
			gc := 2 /* buffer count */ * GGMLTensorOverhead() * (uint64(len(gf.TensorInfos)) + 1 + a.ClipVisionBlockCount*3)
			e.Devices[0].Computation.Footprint += GGUFBytesScalar(gc)

			// Tensor usage.
			var (
				hasClassEmbd bool
				nPositions   uint64
				nBatch       uint64
				nEmbd        uint64
				nHead        uint64
			)
			{
				_, hasClassEmbd = ipLs.Get("v.class_embd")
				nPositions = nPatches
				if hasClassEmbd {
					nPositions += 1
				}
				if a.ClipHasQwen2VLMerger ||
					a.ClipProjectorType == "qwen2vl_merger" ||
					a.ClipProjectorType == "qwen2.5vl_merger" ||
					a.ClipProjectorType == "qwen2.5o" {
					nPositions *= 4
				}
				nBatch = 1
				nEmbd = a.ClipVisionEmbeddingLength
				nHead = a.ClipVisionAttentionHeadCount
			}
			// First, get the usage of input layer.
			{
				var (
					inpRaw     = GGMLTypeF32.RowSizeOf([]uint64{widthMaxSize, heightMaxSize, 3, nBatch}) // F32 [img_width, img_height, 3, n_batch]
					inpRawCnt  = GGMLTypeF32.RowSizeOf([]uint64{nPatches, nEmbd, nBatch})                // I32 [n_patches, n_embd, n_batch]
					inpEmbd    = GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions, nBatch})              // F32 [n_embd, n_positions, n_batch]
					inpPosEmbd = GGMLTypeF32.RowSizeOf([]uint64{projectionDim, nPatches, nBatch})        // F32 [mmproj, n_patches, n_batch]
					inpPos     = GGMLTypeI32.RowSizeOf([]uint64{nPositions})                             // I32 [n_positions]
					inpPatches = GGMLTypeI32.RowSizeOf([]uint64{nPatches})                               // I32 [n_patches]
				)
				e.Devices[idx].Computation.Input += GGUFBytesScalar(inpRaw + inpRawCnt + inpPos + inpPatches)
				if a.ClipHasMiniCPMVProjector ||
					a.ClipProjectorType == "resampler" {
					e.Devices[idx].Computation.Input += GGUFBytesScalar(inpPosEmbd)
				}
				if hasClassEmbd {
					e.Devices[idx].Computation.Input += GGUFBytesScalar(inpEmbd)
				}
				if a.ClipVisionWindowAttentionPattern > 0 { // Qwen2.5 VL
					inpWindowIndex := GGMLTypeI32.RowSizeOf([]uint64{nPatches})              // I32 [n_patches]
					inpWindowMask := GGMLTypeI32.RowSizeOf([]uint64{nPositions, nPositions}) // I32 [n_positions, n_positions]
					e.Devices[idx].Computation.Input += GGUFBytesScalar(inpWindowIndex + inpWindowMask)
				}
			}
			// Since the steps between transformer layers are serial,
			// the allocated memory can be reused for the next layer.
			// So, we only consider the usage of a certain layer.
			{
				compNorm := GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions}) * 2
				compVcur := GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions})
				compKcur := GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions})
				compKQcur := GGMLTypeF32.RowSizeOf([]uint64{nPositions, nPositions, nHead})
				e.Devices[idx].Computation.Compute += GGUFBytesScalar(compNorm + compVcur + compKcur + compKQcur)
			}
		}
	}

	if a.ClipHasAudioEncoder {
		// See https://github.com/ggml-org/llama.cpp/blob/6385b843a8dc8e15b8362196039720c58dd79fa2/tools/mtmd/mtmd-audio.cpp#L311.
		var projectionDim uint64 // NB(thxCode): do not sure if there is the correct name.
		{
			if ti, ok := gf.TensorInfos.Get("a.position_embd.weight"); ok {
				projectionDim = ti.Dimensions[1]
			}
		}

		// Computation.
		{
			// See https://github.com/ggml-org/llama.cpp/blob/ec9e0301fef6476df83e94842c3b625501c95566/tools/mtmd/clip.cpp#L374.
			var maxNodes uint64 = 8192

			// Bootstrap, compute metadata.
			cm := GGMLTensorOverhead()*maxNodes + GGMLComputationGraphOverhead(maxNodes, false)
			e.Devices[0].Computation.Footprint += GGUFBytesScalar(cm)

			// Scheduler overhead,
			// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L16149.
			e.Devices[0].Computation.Footprint += GGUFBytesScalar(4 * 1024 * 1024)

			// GGML context,
			// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L5015-L5036.
			gc := 2 /* buffer count */ * GGMLTensorOverhead() * (uint64(len(gf.TensorInfos)) + 1 + a.ClipAudioBlockCount*3)
			e.Devices[0].Computation.Footprint += GGUFBytesScalar(gc)

			// Tensor usage.
			var (
				nPositions uint64
				nBatch     uint64
				nEmbd      uint64
				nHead      uint64
			)
			{
				nPositions = projectionDim
				nBatch = 1
				nEmbd = a.ClipAudioEmbeddingLength
				nHead = a.ClipAudioAttentionHeadCount
			}
			// First, get the usage of input layer.
			{
				inpEmbd := GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions, nBatch}) // F32 [n_embed, n_positions, n_batch]
				e.Devices[idx].Computation.Input += GGUFBytesScalar(inpEmbd)
			}
			// Since the steps between transformer layers are serial,
			// the allocated memory can be reused for the next layer.
			// So, we only consider the usage of a certain layer.
			{
				compNorm := GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions})
				compVcur := GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions})
				compKcur := GGMLTypeF32.RowSizeOf([]uint64{nEmbd, nPositions})
				compKQcur := GGMLTypeF32.RowSizeOf([]uint64{nPositions, nPositions, nHead})
				e.Devices[idx].Computation.Compute += GGUFBytesScalar(compNorm + compVcur + compKcur + compKQcur)
			}
		}
	}
}

// estimateLLaMACppRunInAdapter estimates the usages of the GGUF file for adapter.
func (gf *GGUFFile) estimateLLaMACppRunInAdapter(o *_GGUFRunEstimateOptions, a *GGUFArchitecture, e *LLaMACppRunEstimate) {
	ls := gf.Layers()
	ioLs, tfLs, _ := ls.Cut([]string{
		"position_*",
		"token_*",
		"cls.*",
		"output.*",
		"output_*",
	})
	ipLs, opLs, _ := ioLs.Cut([]string{
		"position_*",
		"token_*",
	})

	if a.BlockCount == 0 {
		a.BlockCount = uint64(len(tfLs))
	}

	// Full offload: nLoadLayers == 0 && isOffloadOutputLayer
	// Zero offload: nOffloadLayers == 0
	// Partial offload: !Full offload && !Zero offload
	var (
		nOffloadLayers       uint64
		nActualOffloadLayers uint64
		nLoadLayers          = a.BlockCount
		idxOutputDevice      int

		fullOffload bool
	)
	{
		var isOffloadOutputLayer bool

		switch v := o.LMCOffloadLayers; {
		case v == nil:
			o.LMCOffloadLayers = ptr.To(a.BlockCount)
			nOffloadLayers = a.BlockCount
			isOffloadOutputLayer = true
		case *v != 0:
			nOffloadLayers = *v
			if nOffloadLayers > a.BlockCount {
				isOffloadOutputLayer = true
				nOffloadLayers = a.BlockCount
			}
		}
		nActualOffloadLayers = nOffloadLayers
		if isOffloadOutputLayer {
			nActualOffloadLayers += 1
		}
		nLoadLayers -= nOffloadLayers

		fullOffload = nLoadLayers == 0 && isOffloadOutputLayer

		e.FullOffloaded = fullOffload
		e.OffloadLayers = nOffloadLayers

		for i, j, offloadStart := 0, 0, len(tfLs)-int(nOffloadLayers); i < len(tfLs); i++ {
			switch {
			case i < int(nLoadLayers):
				e.Devices[0].HandleLayers += 1
				e.Devices[0].HandleLastLayer = i
			case i >= offloadStart:
				x := float64(i-offloadStart) / float64(nActualOffloadLayers)
				j = slicex.UpperBound(o.TensorSplitFraction, x)
				e.Devices[j+1].HandleLayers += 1
				e.Devices[j+1].HandleLastLayer = i
				if fullOffload && i == len(tfLs)-1 {
					idxOutputDevice = j + 1
				}
			}
		}

		e.Devices[idxOutputDevice].HandleOutputLayer = true
	}

	// Distributable.
	e.Distributable = false

	// Footprint.
	{
		// Bootstrap.
		e.Devices[0].Footprint = GGUFBytesScalar(5*1024*1024) /* model load */ + (gf.Size - gf.ModelSize) /* metadata */
	}

	// Weight & Parameter.
	{
		// Compute.
		for i, j, offloadStart := 0, 0, len(tfLs)-int(nOffloadLayers); i < len(tfLs); i++ {
			idx := 0
			if i >= offloadStart {
				x := float64(i-offloadStart) / float64(nActualOffloadLayers)
				j = slicex.UpperBound(o.TensorSplitFraction, x)
				idx = j + 1
			}
			e.Devices[idx].Weight.Compute += GGUFBytesScalar(tfLs[i].Bytes())
			e.Devices[idx].Parameter.Compute += GGUFParametersScalar(tfLs[i].Elements())
		}

		// IO,
		// see https://github.com/ggerganov/llama.cpp/blob/d6ef0e77dd25f54fb5856af47e3926cf6f36c281/llama.cpp#L4930-L5002.
		e.Devices[0].Weight.Input = GGUFBytesScalar(ipLs.Bytes())
		e.Devices[0].Parameter.Input = GGUFParametersScalar(ipLs.Elements())
		var (
			wg GGUFBytesScalar
			ps GGUFParametersScalar
		)
		if _, ok := opLs.Get("output.weight"); ok {
			wg = GGUFBytesScalar(opLs.Bytes())
			ps = GGUFParametersScalar(opLs.Elements())
		} else {
			wg = GGUFBytesScalar(opLs.Bytes()) + e.Devices[0].Weight.Input /* duplicate the input layer */
			ps = GGUFParametersScalar(opLs.Elements() + ipLs.Elements())
		}
		e.Devices[0].Weight.Output = wg
		if fullOffload {
			e.Devices[idxOutputDevice].Weight.Output = wg
			e.Devices[idxOutputDevice].Parameter.Output = ps
		} else {
			e.Devices[0].Parameter.Output = ps
		}
	}
}

// estimateLLaMACppRunInIMatrix estimates the usages of the GGUF file for imatrix.
func (gf *GGUFFile) estimateLLaMACppRunInIMatrix(_ *_GGUFRunEstimateOptions, a *GGUFArchitecture, e *LLaMACppRunEstimate) {
	ls := gf.Layers()

	if a.BlockCount == 0 {
		a.BlockCount = uint64(len(ls))
	}

	// Distributable.
	e.Distributable = false

	// Footprint.
	{
		// Bootstrap.
		e.Devices[0].Footprint = GGUFBytesScalar(5*1024*1024) /* model load */ + (gf.Size - gf.ModelSize) /* metadata */
	}

	// Weight & Parameter.
	{
		var (
			wg GGUFBytesScalar
			ps GGUFParametersScalar
		)
		wg = GGUFBytesScalar(ls.Bytes())
		ps = GGUFParametersScalar(ls.Elements())
		e.Devices[0].Weight.Compute = wg
		e.Devices[0].Parameter.Compute = ps
	}
}

// Types for LLaMACpp estimated summary.
type (
	// LLaMACppRunEstimateSummary represents the summary of the usage for loading the GGUF file in llama.cpp.
	LLaMACppRunEstimateSummary struct {
		/* Basic */

		// Items
		Items []LLaMACppRunEstimateSummaryItem `json:"items"`

		/* Appendix */

		// Type describes what type this GGUF file is.
		Type string `json:"type"`
		// Architecture describes what architecture this GGUF file implements.
		//
		// All lowercase ASCII.
		Architecture string `json:"architecture"`
		// ClipProjectorType is the type of the projector used in the clip model.
		//
		// Only used when Architecture is "clip".
		ClipProjectorType string `json:"clipProjectorType,omitempty"`
		// AdapterType is the type of the adapter.
		//
		// Only used when Architecture is "adapter".
		AdapterType string `json:"adapterType,omitempty"`
		// ContextSize is the size of the context.
		ContextSize uint64 `json:"contextSize"`
		// FlashAttention is the flag to indicate whether enable the flash attention,
		// true for enable.
		FlashAttention bool `json:"flashAttention"`
		// NoMMap is the flag to indicate whether the file must be loaded without mmap,
		// true for total loaded.
		NoMMap bool `json:"noMMap"`
		// EmbeddingOnly is the flag to indicate whether the model is used for embedding only,
		// true for embedding only.
		EmbeddingOnly bool `json:"embeddingOnly"`
		// Reranking is the flag to indicate whether the model is used for reranking,
		// true for reranking.
		//
		// Only available when EmbeddingOnly is true.
		Reranking bool `json:"reranking"`
		// Distributable is the flag to indicate whether the model is distributable,
		// true for distributable.
		Distributable bool `json:"distributable"`
		// LogicalBatchSize is the logical batch size.
		LogicalBatchSize int32 `json:"logicalBatchSize"`
		// PhysicalBatchSize is the physical batch size.
		PhysicalBatchSize int32 `json:"physicalBatchSize"`
	}

	// LLaMACppRunEstimateSummaryItem represents one summary item for loading the GGUF file in llama.cpp.
	LLaMACppRunEstimateSummaryItem struct {
		// OffloadLayers is the number of offloaded layers.
		OffloadLayers uint64 `json:"offloadLayers"`
		// FullOffloaded is the flag to indicate whether the layers are fully offloaded,
		// false for partial offloaded or zero offloaded.
		FullOffloaded bool `json:"fullOffloaded"`
		// MaximumTokensPerSecond is the maximum tokens per second for running the GGUF file.
		MaximumTokensPerSecond *GGUFTokensPerSecondScalar `json:"maximumTokensPerSecond,omitempty"`
		// RAM is the memory usage for loading the GGUF file in RAM.
		RAM LLaMACppRunEstimateMemory `json:"ram"`
		// VRAMs is the memory usage for loading the GGUF file in VRAM per device.
		VRAMs []LLaMACppRunEstimateMemory `json:"vrams"`
	}

	// LLaMACppRunEstimateMemory represents the memory usage for loading the GGUF file in llama.cpp.
	LLaMACppRunEstimateMemory struct {
		// HandleLayers is the number of layers that the device can handle.
		HandleLayers uint64 `json:"handleLayers"`
		// HandleLastLayer is the index of the last layer the device can handle.
		HandleLastLayer int `json:"handleLastLayer"`
		// HandleOutputLayer is the flag to indicate whether the device can handle the output layer,
		// true for handle.
		HandleOutputLayer bool `json:"handleOutputLayer"`
		// Remote is the flag to indicate whether the device is remote,
		// true for remote.
		Remote bool `json:"remote"`
		// Position is the relative position of the device,
		// starts from 0.
		//
		// If Remote is true, Position is the position of the remote devices,
		// Otherwise, Position is the position of the device in the local devices.
		Position int `json:"position"`
		// UMA represents the usage of Unified Memory Architecture.
		UMA GGUFBytesScalar `json:"uma"`
		// NonUMA represents the usage of Non-Unified Memory Architecture.
		NonUMA GGUFBytesScalar `json:"nonuma"`
	}
)

// SummarizeItem returns the corresponding LLaMACppRunEstimateSummaryItem with the given options.
func (e LLaMACppRunEstimate) SummarizeItem(mmap bool, nonUMARamFootprint, nonUMAVramFootprint uint64) (emi LLaMACppRunEstimateSummaryItem) {
	emi.OffloadLayers, emi.FullOffloaded = e.OffloadLayers, e.FullOffloaded
	if emi.FullOffloaded {
		emi.OffloadLayers++ // The output layer is offloaded.
	}
	emi.MaximumTokensPerSecond = e.MaximumTokensPerSecond

	// RAM.
	{
		fp := e.Devices[0].Footprint
		wg := e.Devices[0].Weight.Sum()
		kv := e.Devices[0].KVCache.Sum()
		cp := e.Devices[0].Computation.Sum()

		emi.RAM.HandleLayers = e.Devices[0].HandleLayers
		emi.RAM.HandleLastLayer = e.Devices[0].HandleLastLayer
		emi.RAM.HandleOutputLayer = e.Devices[0].HandleOutputLayer

		// UMA.
		emi.RAM.UMA = fp + wg + kv + cp
		if !e.NoMMap && (mmap || e.FullOffloaded) {
			emi.RAM.UMA -= wg
			if !mmap {
				emi.RAM.UMA += e.Devices[0].Weight.Output
				emi.RAM.UMA += e.Devices[0].Weight.ComputeOverridden
			}
		}

		// NonUMA.
		emi.RAM.NonUMA = GGUFBytesScalar(nonUMARamFootprint) + emi.RAM.UMA
	}

	// VRAMs.
	emi.VRAMs = make([]LLaMACppRunEstimateMemory, len(e.Devices)-1)
	{
		for i, d := range e.Devices[1:] {
			fp := d.Footprint
			wg := d.Weight.Sum()
			kv := d.KVCache.Sum()
			cp := d.Computation.Sum()

			emi.VRAMs[i].HandleLayers = d.HandleLayers
			emi.VRAMs[i].HandleLastLayer = d.HandleLastLayer
			emi.VRAMs[i].HandleOutputLayer = d.HandleOutputLayer
			emi.VRAMs[i].Remote = d.Remote
			emi.VRAMs[i].Position = d.Position

			// UMA.
			emi.VRAMs[i].UMA = fp + wg + kv + /* cp */ 0
			if !e.NoMMap && mmap {
				emi.VRAMs[i].UMA -= wg
				if d.Remote || d.Position > 0 && d.HandleLastLayer >= 0 || e.Type == "projector" {
					emi.VRAMs[i].UMA += wg
				}
			}

			// NonUMA.
			emi.VRAMs[i].NonUMA = GGUFBytesScalar(nonUMAVramFootprint) + fp + wg + kv + cp
			if !d.Remote && d.Position > 0 && d.HandleLastLayer < 0 {
				emi.VRAMs[i].NonUMA -= wg + cp
			}
		}
	}

	// Add drafter's usage.
	if e.Drafter != nil {
		demi := e.Drafter.SummarizeItem(mmap, 0, 0)
		emi.RAM.UMA += demi.RAM.UMA
		emi.RAM.NonUMA += demi.RAM.NonUMA
		for i, v := range demi.VRAMs {
			emi.VRAMs[i].UMA += v.UMA
			emi.VRAMs[i].NonUMA += v.NonUMA
		}
	}

	// Add projector's usage.
	if e.Projector != nil {
		pemi := e.Projector.SummarizeItem(mmap, 0, 0)
		emi.RAM.UMA += pemi.RAM.UMA
		emi.RAM.NonUMA += pemi.RAM.NonUMA
		for i, v := range pemi.VRAMs {
			emi.VRAMs[i].UMA += v.UMA
			emi.VRAMs[i].NonUMA += v.NonUMA
		}
	}

	// Add adapters' usage.
	for i := range e.Adapters {
		aemi := e.Adapters[i].SummarizeItem(false, 0, 0)
		emi.RAM.UMA += aemi.RAM.UMA
		emi.RAM.NonUMA += aemi.RAM.NonUMA
		for j, v := range aemi.VRAMs {
			emi.VRAMs[j].UMA += v.UMA
			emi.VRAMs[j].NonUMA += v.NonUMA
		}
	}

	return emi
}

// Summarize returns the corresponding LLaMACppRunEstimateSummary with the given options.
func (e LLaMACppRunEstimate) Summarize(mmap bool, nonUMARamFootprint, nonUMAVramFootprint uint64) (es LLaMACppRunEstimateSummary) {
	// Items.
	es.Items = []LLaMACppRunEstimateSummaryItem{
		e.SummarizeItem(mmap, nonUMARamFootprint, nonUMAVramFootprint),
	}

	// Just copy from the original estimate.
	es.Type = e.Type
	es.Architecture = e.Architecture
	es.ClipProjectorType = e.ClipProjectorType
	es.AdapterType = e.AdapterType
	es.ContextSize = e.ContextSize
	es.FlashAttention = e.FlashAttention
	es.NoMMap = e.NoMMap
	es.EmbeddingOnly = e.EmbeddingOnly
	es.Reranking = e.Reranking
	es.LogicalBatchSize = e.LogicalBatchSize
	es.PhysicalBatchSize = e.PhysicalBatchSize
	es.Distributable = e.Distributable

	return es
}

func (u LLaMACppWeightMemoryUsage) Sum() GGUFBytesScalar {
	return u.Input + u.Compute + u.ComputeOverridden + u.Output
}

func (u LLaMACppKVCacheMemoryUsage) Sum() GGUFBytesScalar {
	return u.Key + u.Value
}

func (u LLaMACppComputationMemoryUsage) Sum() GGUFBytesScalar {
	return u.Footprint + u.Input + max(u.Compute, u.Output)
}

// ClipAligning returns the aligned value of x to the nearest multiple of n,
// see https://github.com/ggml-org/llama.cpp/blob/cdf94a18023c92f41808ec874ba577d914674717/tools/mtmd/clip-impl.h#L114-L115.
func ClipAligning(x, n uint64) uint64 {
	return ((x + n - 1) / n) * n
}
