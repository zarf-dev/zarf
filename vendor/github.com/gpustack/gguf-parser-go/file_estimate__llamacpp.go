package gguf_parser

import (
	"math"
	"regexp"
	"slices"
	"strconv"
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
		// Decision is the flag to indicate whether the model is used for decision,
		// i.e. serving the "POST /v1/systemone" endpoint in llama-server,
		// true for decision.
		//
		// It is presence-based: any string-typed "<architecture>.decision.type"
		// metadata marks the model, mirroring common_get_decision_type in
		// llama.cpp. It outranks Reranking: a declared decision head carries
		// classifier tensors and rank pooling that would otherwise read as
		// reranking.
		Decision bool `json:"decision"`
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

// llama.cpp runs a host layer's heavy nodes on a device once the physical batch
// reaches this size, see ggml_backend_cuda_device_offload_op.
const _LMCOffloadOpMinBatchSize = 32

// The measured rows. Each constant is a count of embedding-wide rows read from
// one GGML_ALLOCATOR_DEBUG replay of the file named beside it. None of them
// derives from the graph builder, so a change to that family's graph in
// llama.cpp silently invalidates it. Re-measure the family when its graph
// changes, and replace the constant with a derivation when one is found.
const (
	// rwkv7-g1c-13.3b: the rows alive at the peak, a dozen embedding-wide
	// rows and the six-way fused projection.
	_LMCMeasuredRWKV7PeakRows = 19
	// arwkv-qwen-r1-1b5: the qwen FFN rides beside the rwkv rows.
	_LMCMeasuredARWKV7PeakRows = 24
	// v6-Finch-7B: a similar but narrower set than rwkv7.
	_LMCMeasuredRWKV6PeakRows = 15
	// RWKV6QwQ-32B.
	_LMCMeasuredRWKV6Qwen2PeakRows = 20
	// granite-switch-4.1-3b-preview: the switch router keeps three extra rows
	// at the full-offload peak.
	_LMCMeasuredGraniteSwitchRouterRows = 3
	// moondream2 (phi2): the parallel residual holds one more row at both
	// moments.
	_LMCMeasuredPhi2ResidualRows = 1
	// Qwen3-VL-8B: the deepstack embedding rides beside the residual through
	// the whole graph, three rows wide.
	_LMCMeasuredQwen3VLDeepstackRows = 3
	// gemma-3n-E2B: the altup block holds the selected per-layer inputs in
	// nine rows before the attention node.
	_LMCMeasuredGemma3nAltupRows = 9
	// gemma-3n-E2B: the altup input count that stands in when the file
	// carries no altup_predict_coef tensor to read it from.
	_LMCMeasuredGemma3nAltupInputs = 4
	// NVIDIA-Nemotron-Nano-12B-v2 and plamo-2-translate: half of one zx row
	// strands beside the zx row at the recurrent peak, so the share divides
	// the row by this.
	_LMCMeasuredSSMFragmentedZxDivisor = 2
	// OLMoE-1B-7B and granite-3.1-3b-a800m: a host boundary stages four
	// weight and cache slots, the K and V cache, the attention weight, and
	// the gate and up weights, and each frees a slot that strands under the
	// activation stack by at most one activation row. The two FFN slots price
	// as the largest FFN weight.
	_LMCMeasuredStagedFFNSlots = 2
)

// clampInt32 narrows a declared count to the int32 range the run options hold.
// llama.cpp reads the same keys as uint32, so a file it loads declares at most
// 2^32 - 1, and the clamp keeps the estimate at the largest context the options
// can name rather than at a negative one.
func clampInt32(v uint64) int32 {
	return int32(min(v, math.MaxInt32))
}

var (
	// One transformer layer's attention weights, as llama.cpp names them.
	_LMCAttentionWeightRegex = regexp.MustCompile(`.*\.\d+\.attn_(q|k|v|output|qkv|q_a|q_b|kv_a_mqa|kv_b)\.weight`)
	// One transformer layer's feed-forward weights, dense or expert.
	_LMCFeedForwardWeightRegex = regexp.MustCompile(`.*\.\d+\.ffn_(gate|up|down)(_exps|_shexp)?\.weight`)
	// The expert up projection, shaped [n_embd, n_ff_exp, n_expert].
	_LMCExpertUpWeightRegex = regexp.MustCompile(`.*\.\d+\.ffn_up_exps\.weight`)
	// The dense gate projection; its absence marks a gateless MLP.
	_LMCGateWeightRegex = regexp.MustCompile(`.*\.\d+\.ffn_gate\.weight`)
	// The dense up projection; its output width is the activation row.
	_LMCUpWeightRegex = regexp.MustCompile(`.*\.\d+\.ffn_up\.weight`)
	// The compressed KV projection that marks a latent-attention graph.
	_LMCLatentAttentionRegex = regexp.MustCompile(`.*\.\d+\.attn_kv_a_mqa\.weight`)
	// Any transformer-layer weight tensor.
	_LMCLayerWeightRegex = regexp.MustCompile(`.*\.\d+\..*\.weight`)
	// The gemma3n altup prediction coefficients; their first dimension is n_altup.
	_LMCAltupPredictCoefRegex = regexp.MustCompile(`.*\.\d+\.altup_predict_coef\.weight`)
	// The relative attention bias that marks a KQ-biased graph (t5).
	_LMCRelativeBiasWeightRegex = regexp.MustCompile(`.*\.\d+\.attn_rel_b\.weight`)
	// The block index inside a tensor name.
	_LMCBlockIndexRegex = regexp.MustCompile(`\.(\d+)\.`)
	// The weights whose outputs the non-flash attention moment holds.
	_LMCAttentionNodeWeightRegex = regexp.MustCompile(`.*\.\d+\.attn_(norm|q|qkv|q_b)\.weight`)
	// The weights whose outputs the non-flash FFN moment holds.
	_LMCFeedForwardNodeWeightRegex = regexp.MustCompile(`.*\.\d+\.(attn_norm|ffn_norm|ffn_gate|ffn_up)\.weight`)
)

// estimateLLaMACppRecurrentComputeFloor floors a recurrent graph's device
// compute buffer at the rows the allocator holds at once, read from
// GGML_ALLOCATOR_DEBUG replays: rwkv7 keeps a dozen embedding-wide rows and
// its six-way fused projection, rwkv6 a similar but narrower set, and mamba2
// the fused zx projection with two convolution-width rows. Every recurrent
// graph strands two residual rows, and a host boundary stages one layer's
// largest weight.
func estimateLLaMACppRecurrentComputeFloor(
	a *GGUFArchitecture, tfLs GGUFLayerTensorInfos, nTokens, cp uint64, boundary bool,
) uint64 {
	embdRow := GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, nTokens})
	floor := uint64(0)
	switch a.Architecture {
	case "rwkv7":
		floor = _LMCMeasuredRWKV7PeakRows * embdRow
	case "arwkv7":
		floor = _LMCMeasuredARWKV7PeakRows * embdRow
	case "rwkv6":
		floor = _LMCMeasuredRWKV6PeakRows * embdRow
	case "rwkv6qwen2":
		floor = _LMCMeasuredRWKV6Qwen2PeakRows * embdRow
	}
	if a.SSMInnerSize > 0 {
		zx, conv := a.ssmProjectionRows(nTokens)
		floor = max(floor, zx+
			2*conv+ // the two convolution-width rows
			2*embdRow+ // the stranded residual rows
			zx/_LMCMeasuredSSMFragmentedZxDivisor)
	}
	cp = max(cp, floor) + 2*embdRow // every recurrent graph strands two residual rows
	if boundary {
		var w uint64
		for _, l := range tfLs[0].Search(_LMCLayerWeightRegex) {
			w = max(w, l.Bytes())
		}
		cp += w
	}
	return cp
}

// _LMCBackendUnholdableArchitectures lists the architectures the CUDA backend
// has no kernels for. llama.cpp assigns every layer of these to the CPU
// whatever the offload asks, then the batch offload path copies each layer's
// weights into the device compute buffer, and the graph allocator holds close
// to the whole model at once. Checked against llama.cpp c841aeeb8; refresh
// this set as llama.cpp gains kernels.
var _LMCBackendUnholdableArchitectures = []string{"nanbeige"}

// searchLastBlock returns the regex's matches for the highest-numbered block
// among the given layers. A per-block layout keeps one block per tf layer, so
// the last layer's matches come back as they are. A prefixed family like t5
// folds every enc block into one tf layer; there a search of the last matching
// layer spans every block, so only the last block's tensors are kept.
func searchLastBlock(tfLs GGUFLayerTensorInfos, re *regexp.Regexp) []GGUFTensorInfo {
	for i := len(tfLs) - 1; i >= 0; i-- {
		matches := tfLs[i].Search(re)
		if len(matches) == 0 {
			continue
		}
		maxBlk, blkOf := -1, make([]int, len(matches))
		for j := range matches {
			blkOf[j] = -1
			if sm := _LMCBlockIndexRegex.FindStringSubmatch(matches[j].Name); sm != nil {
				blkOf[j], _ = strconv.Atoi(sm[1])
			}
			maxBlk = max(maxBlk, blkOf[j])
		}
		out := matches[:0]
		for j := range matches {
			if blkOf[j] == maxBlk {
				out = append(out, matches[j])
			}
		}
		return out
	}
	return nil
}

// estimateLLaMACppFlashComputeMoments sizes the device compute buffer of a
// flash attention graph. The buffer peaks at one of a few moments of a single
// layer: its FLASH_ATTN_EXT node, its FFN block, or, on a hybrid, its
// recurrent block. With boundary set, the device also stages the host layer's
// K and V cache and its largest weights, one layer at a time, so the buffer is
// the same at every partial depth.
// _LMCFlashStagedSlots holds what a host boundary stages on the device for
// one layer: its K and V cache rows, its K and V rows staged a second time,
// and its largest attention and FFN weights. Without a boundary the weight
// slots stay zero.
type _LMCFlashStagedSlots struct {
	boundary            bool
	loadAttnInc         uint64
	stagedKV            uint64
	wAttnSlot           uint64
	wFfnSlot, wFfnSlot2 uint64
}

// strandedUnder is what the staged slots leave stranded under an activation
// stack of the given row: at most one activation row per slot, see
// _LMCMeasuredStagedFFNSlots.
func (s _LMCFlashStagedSlots) strandedUnder(act uint64) uint64 {
	return min(s.loadAttnInc, act) + min(s.wAttnSlot, act) + _LMCMeasuredStagedFFNSlots*min(s.wFfnSlot, act)
}

// estimateLLaMACppFlashFFNActivations sizes the widest run of FFN activations
// in one flash attention layer, beside the slots a host boundary stages. It
// also returns the dense hole a mixture strands under its peak, and one
// activation row for the hybrid moment.
func estimateLLaMACppFlashFFNActivations(
	a *GGUFArchitecture, hostLayers, allLayers []IGGUFTensorInfos,
	nTokens, embdRow uint64, slots _LMCFlashStagedSlots, gated bool,
) (acts, strand, lastAct uint64) {
	if a.ExpertCount > 0 || a.ExpertUsedCount > 0 {
		acts += GGMLTypeF32.RowSizeOf([]uint64{uint64(a.ExpertCount), a.EmbeddingLength}) // ffn_gate_input
		acts += GGMLTypeF32.RowSizeOf([]uint64{uint64(a.ExpertCount), nTokens})           // ffn_moe_logits
		nFFExp := a.ExpertFeedForwardLength
		if nFFExp == 0 {
			// Some files do not declare <arch>.expert_feed_forward_length.
			// Every layer counts: an interleaved mixture's block zero can be
			// dense, and a full offload references only that block here.
			for _, hl := range allLayers {
				for _, l := range hl.Search(_LMCExpertUpWeightRegex) {
					if l.NDimensions > 1 {
						nFFExp = l.Dimensions[1]
						break
					}
				}
			}
		}
		act := GGMLTypeF32.RowSizeOf([]uint64{nFFExp, uint64(a.ExpertUsedCount), nTokens})
		down := GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, uint64(a.ExpertUsedCount), nTokens}) // ffn_moe_down
		lastAct = act
		stack := 3 * act
		if a.ExpertSharedFeedForwardLength > 0 {
			// The shared expert runs beside the routed ones.
			stack += 3 * GGMLTypeF32.RowSizeOf([]uint64{a.ExpertSharedFeedForwardLength, nTokens})
		}
		if down > 2*act {
			// The down projection cannot reuse the hole the gate and up
			// activations free, so all four stages hold slots at once.
			stack += down
		}
		if slots.boundary {
			actsPhase := stack + slots.strandedUnder(act)
			downMoment := act + // the swiglu product
				slots.wFfnSlot + // the staged expert weight
				slots.wAttnSlot + // the staged attention weight
				down + // ffn_moe_down
				slots.stagedKV // the host layer's K and V rows
			acts += max(actsPhase, downMoment)
		} else {
			acts += stack
		}
		// A mixture whose graph also holds dense blocks frees a dense-sized
		// hole between layers. When that hole is smaller than the mixture's
		// own activations, the live residual fragments it and the peak keeps
		// it as dead space.
		denseFF := uint64(0)
		if len(a.FeedForwardLength) > 0 {
			denseFF = a.FeedForwardLength[0]
		}
		if denseFF > 0 && denseFF != nFFExp {
			for _, hl := range hostLayers {
				if len(hl.Search(_LMCExpertUpWeightRegex)) == 0 {
					strand = 3 * GGMLTypeF32.RowSizeOf([]uint64{denseFF, nTokens})
					break
				}
			}
		}
	} else {
		var nFFMeta uint64
		for _, v := range a.FeedForwardLength {
			nFFMeta = max(nFFMeta, v)
		}
		nFF := nFFMeta
		for _, tl := range allLayers {
			for _, l := range tl.Search(_LMCUpWeightRegex) {
				nFF = max(nFF, l.Dimensions[l.NDimensions-1])
			}
		}
		act := GGMLTypeF32.RowSizeOf([]uint64{nFF, nTokens})
		actStack := act + embdRow // a gateless MLP: the activation reuses the up slot, the down output rides beside it
		switch {
		case gated:
			actStack = 3 * act // gate, up and the GLU product hold rows at once
		case nFFMeta > 0 && nFF >= 2*nFFMeta:
			actStack = act + act/2 + embdRow // fused GLU: the product holds half a row beside the up row
		}
		lastAct = act
		if slots.boundary {
			actsPhase := actStack + slots.strandedUnder(act)
			downMoment := act + // the up activation
				slots.wFfnSlot + // the staged FFN weight
				slots.wAttnSlot + // the staged attention weight
				embdRow + // ffn_out
				slots.stagedKV // the host layer's K and V rows
			if !gated {
				// Only the small down output follows, so the freed up-weight
				// hole strands under the peak.
				downMoment += slots.wFfnSlot2
			}
			acts += max(actsPhase, downMoment)
		} else {
			acts += actStack
		}
	}
	return acts, strand, lastAct
}

func estimateLLaMACppFlashComputeMoments(
	o *_GGUFRunEstimateOptions, a *GGUFArchitecture,
	hostLayers, allLayers []IGGUFTensorInfos,
	nKV, nTokens, loadAttnInc uint64,
	boundary, gated bool,
) (attnInc, ffnInc uint64) {
	// One layer's working set around the attention node.
	embdRow := GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, nTokens})
	attnQ := GGMLTypeF32.RowSizeOf([]uint64{uint64(a.AttentionKeyLength) * a.AttentionHeadCount, nTokens})
	attnK := GGMLTypeF32.RowSizeOf([]uint64{uint64(a.AttentionKeyLength) * a.AttentionHeadCountKV, nTokens})
	attnV := GGMLTypeF32.RowSizeOf([]uint64{uint64(a.AttentionValueLength) * a.AttentionHeadCountKV, nTokens})
	kqMask := GGMLTypeF16.RowSizeOf([]uint64{nKV, nTokens})
	if a.AttentionSlidingWindow > 0 {
		// A sliding-window graph builds a windowed mask next to the full one.
		kqMask *= 2
	}

	// A latent-attention graph decompresses K and V through low-rank rows that
	// ride beside every moment.
	archRows := uint64(0)
	for _, tl := range allLayers {
		if len(tl.Search(_LMCLatentAttentionRegex)) != 0 {
			archRows = 2 * embdRow
			break
		}
	}

	// The largest weights of any host layer, staged on the device as split
	// inputs, one layer at a time. Every host layer counts: a mixture with
	// leading dense blocks keeps its expert tensors past block zero.
	slots := _LMCFlashStagedSlots{boundary: boundary, loadAttnInc: loadAttnInc, stagedKV: attnK + attnV}
	if a.buildsMemorylessGraph() {
		// A memoryless graph has no cache tensors to stage; only weights cross.
		slots.stagedKV = 0
	}
	if boundary {
		for _, hl := range hostLayers {
			for _, l := range hl.Search(_LMCAttentionWeightRegex) {
				slots.wAttnSlot = max(slots.wAttnSlot, l.Bytes())
			}
			for _, l := range hl.Search(_LMCFeedForwardWeightRegex) {
				if b := l.Bytes(); b > slots.wFfnSlot {
					slots.wFfnSlot, slots.wFfnSlot2 = b, slots.wFfnSlot
				} else {
					slots.wFfnSlot2 = max(slots.wFfnSlot2, b)
				}
			}
		}
	}

	// The FLASH_ATTN_EXT allocation. On CUDA and ROCm it is the f32 output plus
	// room to convert one layer's K and V to f16 when the cache is quantized,
	// see ggml_cuda_flash_attn_ext_get_alloc_size.
	faAlloc := GGMLTypeF32.RowSizeOf([]uint64{uint64(a.AttentionValueLength) * a.AttentionHeadCount, nTokens})
	if *o.LMCCacheKeyType != GGMLTypeF16 {
		faAlloc += GGMLTypeF16.RowSizeOf([]uint64{uint64(a.AttentionKeyLength), nKV, a.AttentionHeadCountKV})
	}
	if *o.LMCCacheValueType != GGMLTypeF16 {
		faAlloc += GGMLTypeF16.RowSizeOf([]uint64{uint64(a.AttentionValueLength), nKV, a.AttentionHeadCountKV})
	}

	// The attention moment: the rows alive at the FLASH_ATTN_EXT node.
	attnInc = archRows +
		embdRow + // l_out, the residual
		embdRow + // the norm output
		embdRow + // attn_out
		attnQ + // Qcur
		attnK + // Kcur
		attnV + // Vcur
		kqMask +
		faAlloc
	if boundary {
		attnInc += loadAttnInc + // the host layer's K and V cache
			slots.wAttnSlot + // its largest attention weight
			slots.stagedKV // its K and V rows, staged a second time to cross the boundary
	}

	// The FFN moment: the slots the attention node stranded, the mask, the
	// FFN input, and the widest run of FFN activations.
	ffnInc = archRows +
		embdRow + // l_out, the residual
		attnQ + // the stranded Qcur slot
		embdRow + // ffn_inp
		embdRow + // the stranded norm slot
		embdRow + // the stranded attn_out slot
		kqMask
	acts, strand, lastAct := estimateLLaMACppFlashFFNActivations(
		a, hostLayers, allLayers, nTokens, embdRow, slots, gated)
	ffnInc += acts
	strand = min(strand, faAlloc)
	attnInc += strand
	if boundary {
		ffnInc += strand
	}
	attnMeasured, ffnMeasured, ssmMeasured := estimateLLaMACppMeasuredFamilyRows(a, embdRow, lastAct, slots)
	attnInc += attnMeasured
	ffnInc += ffnMeasured
	if a.AttentionSlidingWindow > 0 {
		// An interleaved graph keeps the global layer's FLASH_ATTN_EXT scratch
		// alive under the windowed layer's FFN peak.
		ffnInc += faAlloc
	}
	// A hybrid's recurrent blocks hold their own moment: the fused zx
	// projection, two convolution-width rows, the residuals and a
	// fragmentation share, sized from the mamba2 graph.
	if a.SSMInnerSize > 0 {
		zx, conv := a.ssmProjectionRows(nTokens)
		ssm := zx +
			2*conv + // the two convolution-width rows
			4*embdRow + // the residual rows the block holds
			zx/_LMCMeasuredSSMFragmentedZxDivisor +
			kqMask +
			ssmMeasured
		if boundary {
			ssm += loadAttnInc + slots.wAttnSlot
		}
		ffnInc = max(ffnInc, ssm)
	}
	return attnInc, ffnInc
}

// estimateLLaMACppMeasuredFamilyRows adds what one replay of one file showed
// beyond the derived moments, for the families that carry such a constant.
// See the measured rows above for what each one is and where it came from.
func estimateLLaMACppMeasuredFamilyRows(
	a *GGUFArchitecture, embdRow, lastAct uint64, slots _LMCFlashStagedSlots,
) (attnInc, ffnInc, ssmInc uint64) {
	switch a.Architecture {
	case "qwen3vl", "qwen3vlmoe":
		// The deepstack embedding rides beside the residual through the
		// whole graph, at both moments.
		attnInc = _LMCMeasuredQwen3VLDeepstackRows * embdRow
		ffnInc = _LMCMeasuredQwen3VLDeepstackRows * embdRow
	case "plamo2", "nemotron_h":
		// These blocks hold the ssm and FFN working sets together.
		ssmInc = lastAct + embdRow
	case "graniteswitch":
		ffnInc = _LMCMeasuredGraniteSwitchRouterRows * embdRow
	case "mistral4":
		// Mistral-Small-4: two staged expert tensors overlap at a host boundary.
		if slots.boundary {
			ffnInc = slots.wFfnSlot2
		}
	case "nemotron_h_moe":
		// Nemotron-3.5-Lightning: the copies of two host layers' expert
		// tensors overlap at a host boundary.
		if slots.boundary {
			ffnInc = slots.wFfnSlot + slots.wFfnSlot2
		}
	case "phi2":
		attnInc = _LMCMeasuredPhi2ResidualRows * embdRow
		ffnInc = _LMCMeasuredPhi2ResidualRows * embdRow
	}
	return attnInc, ffnInc, ssmInc
}

// estimateLLaMACppGemma3nAltupMoment sizes the moment before the attention
// node of a gemma3n layer, where the altup block holds the selected per-layer
// inputs, one [n_embd, n_altup] row per altup projection, two masks, and at a
// host boundary the staged cache and FFN weight. Measured on gemma-3n-E2B.
func estimateLLaMACppGemma3nAltupMoment(
	a *GGUFArchitecture, ipLs, tfLs GGUFLayerTensorInfos,
	nKV, nTokens, loadAttnInc uint64, boundary bool,
) uint64 {
	embdRow := GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, nTokens})
	am := _LMCMeasuredGemma3nAltupRows * embdRow
	if l, ok := ipLs.Get("per_layer_model_proj.weight"); ok {
		am += 2 * GGMLTypeF32.RowSizeOf([]uint64{l.Dimensions[l.NDimensions-1], nTokens})
	}
	var nAltup uint64
	for _, l := range tfLs[0].Search(_LMCAltupPredictCoefRegex) {
		nAltup = l.Dimensions[0]
	}
	if nAltup == 0 {
		nAltup = _LMCMeasuredGemma3nAltupInputs
	}
	am += 4 * nAltup * embdRow
	am += 2 * GGMLTypeF16.RowSizeOf([]uint64{nKV, nTokens})
	if boundary {
		var wFfn uint64
		for _, l := range tfLs[0].Search(_LMCFeedForwardWeightRegex) {
			wFfn = max(wFfn, l.Bytes())
		}
		am += loadAttnInc + wFfn
	}
	return am
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
	usingSWA := a.usesSlidingWindowAttention() && !o.LMCFullSizeSWACache

	// Full offload: nLoadLayers == 0 && isOffloadOutputLayer
	// Zero offload: nOffloadLayers == 0
	// Partial offload: !Full offload && !Zero offload
	var (
		nOffloadLayers       uint64
		nActualOffloadLayers uint64
		nLoadLayers          = a.BlockCount
		idxOutputDevice      int

		fullOffload, zeroOffload bool
		// Whether the output layer sits on an offload device. llama.cpp counts it
		// inside n_gpu_layers and fills from the end, so it is offloaded before
		// any block is, not after all of them.
		isOffloadOutputLayer bool

		// The device that holds each layer, as e.Devices is indexed:
		// 0 for the host, 1+N for the N-th offload device.
		// Bounded so a hostile block_count cannot size the allocation; the layers beyond
		// the bound fall back to the host device, see maxSaneBlockCount.
		layerDevices = make([]int, min(a.BlockCount, maxSaneBlockCount))
	)
	{
		switch v := o.LMCOffloadLayers; {
		case v == nil:
			o.LMCOffloadLayers = ptr.To(a.BlockCount + 1)
			nOffloadLayers = a.BlockCount
			isOffloadOutputLayer = true
		case *v != 0:
			// i_gpu_start = max(n_layer_all + 1 - n_gpu_layers, 0), so the count
			// spans the blocks plus the output layer and the output is the first
			// thing to land on the card. --gpu-layers 28 on a 28 block model puts
			// blocks 1..27 and the output on the device and leaves block 0 on the
			// host, which llama.cpp reports as "offloaded 28/29 layers".
			isOffloadOutputLayer = true
			nOffloadLayers = min(*v-1, a.BlockCount)
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
				if usingSWA && a.isSWALayer(i) {
					e.Devices[0].HandleSWALayers += 1
				}
			case i >= offloadStart:
				x := float64(i-offloadStart) / float64(nActualOffloadLayers)
				j = slicex.UpperBound(o.TensorSplitFraction, x)
				if i < uint64(len(layerDevices)) {
					layerDevices[i] = j + 1
				}
				e.Devices[j+1].HandleLayers += 1
				e.Devices[j+1].HandleLastLayer = int(i)
				if usingSWA && a.isSWALayer(i) {
					e.Devices[j+1].HandleSWALayers += 1
				}
				if isOffloadOutputLayer && i == a.BlockCount-1 {
					idxOutputDevice = j + 1
				}
			}
		}

		// An offload that reaches the output layer but no block leaves the loop
		// above with nothing to key off, so name the first offload device here.
		if isOffloadOutputLayer && idxOutputDevice == 0 && len(e.Devices) > 1 {
			idxOutputDevice = 1
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

	// Decision, see common_get_decision_type in llama.cpp's common/common.cpp:
	// a string-typed "<arch>.decision.type" metadata marks a decision model
	// serving "POST /v1/systemone". The detection is presence-only and
	// independent of the attention direction: causal decision models read
	// logits, non-causal ones read embeddings.
	{
		v, found := gf.Header.MetadataKV.Get(a.Architecture + ".decision.type")
		e.Decision = found && v.ValueType == GGUFMetadataValueTypeString
	}

	// Embedding.
	if !a.AttentionCausal {
		ropeFrequencyBase := ptr.Deref(o.LMCRoPEFrequencyBase, a.RoPEFrequencyBase)
		ropeFrequencyScale := ptr.Deref(o.LMCRoPEFrequencyScale, a.RoPEFrequencyScale)
		ropeScalingType := ptr.Deref(o.LMCRoPEScalingType, a.RoPEScalingType)
		ropeScalingOriginalContextSize := ptr.Deref(o.LMCRoPEScalingOriginalContextSize, clampInt32(a.RoPEScalingOriginalContextLength))
		isRoPECustomized := ropeFrequencyBase != a.RoPEFrequencyBase ||
			ropeFrequencyScale != a.RoPEFrequencyScale ||
			ropeScalingType != a.RoPEScalingType ||
			(ropeScalingType == "yarn" && ropeScalingOriginalContextSize != clampInt32(a.RoPEScalingOriginalContextLength))

		e.EmbeddingOnly = true
		o.LMCContextSize = ptr.To(ptr.Deref(o.LMCContextSize, clampInt32(a.MaximumContextLength)))
		// Cap the context size to the training context size.
		if !isRoPECustomized {
			o.LMCContextSize = ptr.To(min(clampInt32(a.MaximumContextLength), *o.LMCContextSize))
		}
		// The batch sizes stay as configured: llama.cpp encodes in n_ubatch
		// slices whatever the context is, and sched_reserve prices the worst
		// graph over min(n_ctx, n_ubatch) tokens, see
		// https://github.com/ggml-org/llama.cpp/blob/c841aeeb8/src/llama-context.cpp#L595.
		// Reranking is outranked by a declared decision head: general backbones
		// (e.g. modern-bert) carry classifier tensors and rank pooling while
		// serving decisions.
		if !e.Decision {
			if _, found := gf.TensorInfos.Index([]string{"cls.bias", "cls.weight"}); found > 0 {
				e.Reranking = true
			}
			if !e.Reranking && a.PoolingType == 4 { // 0: None, 1: Mean, 2: Cls, 3: Last, 4: Rank
				e.Reranking = true
			}
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
		if a.buildsMemorylessGraph() {
			// A memoryless graph attends over the ubatch only: there is no KV
			// cache, and build_attn_inp_no_cache shapes the attention mask as
			// [n_tokens, n_tokens], see
			// https://github.com/ggml-org/llama.cpp/blob/c841aeeb8/src/llama-graph.cpp#L2676-L2698.
			nKV = nTokens
		}

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
		//
		// This is host memory whatever the offload. llama_context::output_reserve
		// allocates it from ggml_backend_cpu_buffer_type, or from the output
		// device's *host* buffer type when it has one, which is pinned system
		// memory chosen "for faster transfer to system memory" rather than VRAM:
		// https://github.com/ggml-org/llama.cpp/blob/master/src/llama-context.cpp
		// (llama_context::output_reserve, the buf_output allocation).
		// Charging it to the output device instead reported ~700 MiB of VRAM that
		// no card ever holds, which is enough to reject a model that fits.
		ob := a.EmbeddingLength * nOutputs * 4 /* float32 size */
		if a.AttentionCausal {
			ob += a.VocabularyLength * nOutputs * 4 /* float32 size */
		}
		// A speculator head scores every position it proposes, so llama.cpp reserves
		// the full vocabulary by batch tensor on the device that holds the head. The
		// host placement above is correct only for a decoder reserved with
		// n_outputs = 1, which is every other architecture.
		if fullOffload && a.emitsLogitsAtEveryPosition() {
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
		if idxOutputDevice != 0 {
			e.Devices[idxOutputDevice].Weight.Output = wg
			e.Devices[idxOutputDevice].Parameter.Output = ps
		} else {
			e.Devices[0].Parameter.Output = ps
		}
	}

	// KV cache.
	if a.AttentionCausal {
		// Recurrent state per layer, context independent,
		// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/llama-hparams.cpp#L183-L222.
		var r, s uint64
		switch {
		case a.RWKVHeadSize > 0:
			r = uint64(a.RWKVTokenShiftCount) * a.EmbeddingLength
			s = uint64(a.RWKVHeadSize) * a.EmbeddingLength
		case a.ShortConvLCache > 0:
			r = (uint64(a.ShortConvLCache) - 1) * a.EmbeddingLength
		case a.KDAHeadDim > 0:
			dConv := uint64(3)
			if a.SSMConvolutionKernel > 0 {
				dConv = uint64(a.SSMConvolutionKernel)
			}
			r = 3 * (dConv - 1) * a.AttentionHeadCount * uint64(a.KDAHeadDim)
			s = uint64(a.KDAHeadDim) * uint64(a.KDAHeadDim) * a.AttentionHeadCount
		default:
			r = a.ssmConvolutionStateSize()
			s = uint64(a.SSMStateSize) * uint64(a.SSMInnerSize)
		}
		rps, sps := r*nSeq, s*nSeq
		rrs, srs := GGMLTypeF32.RowSizeOf([]uint64{rps}), GGMLTypeF32.RowSizeOf([]uint64{sps})

		// Attention KV width per layer,
		// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/llama-hparams.cpp#L131-L141.
		akl, avl := uint64(a.AttentionKeyLength), uint64(a.AttentionValueLength)
		if a.AttentionKeyLengthMLA > 0 && a.AttentionValueLengthMLA > 0 {
			akl, avl = uint64(a.AttentionKeyLengthMLA), uint64(a.AttentionValueLengthMLA)
		}
		headsKVOfLayer := func(i uint64) uint64 {
			if i < uint64(len(a.AttentionHeadCountKVs)) {
				return a.AttentionHeadCountKVs[i]
			}
			return a.AttentionHeadCountKV
		}

		// The sliding window layers may size their heads separately (Gemma 4).
		swaAkl, swaAvl := akl, avl
		if a.AttentionKeyLengthSWA > 0 {
			swaAkl = uint64(a.AttentionKeyLengthSWA)
		}
		if a.AttentionValueLengthSWA > 0 {
			swaAvl = uint64(a.AttentionValueLengthSWA)
		}

		// Sliding window attention size,
		// see https://github.com/ggml-org/llama.cpp/blob/3079e9ac8e04ef6eddeb0c164d72edb6b6fd2df5/src/llama-kv-cache.cpp#L1640-L1642.
		swas := min(nKV, GGMLPadding(a.AttentionSlidingWindow*nSeq+uint64(*o.LMCLogicalBatchSize), paddingAlign))

		// The NextN/MTP blocks of the hybrid family are filtered out of the main context cache,
		// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/llama-model.cpp#L2290-L2296
		// and https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/llama-model.cpp#L2360-L2368;
		// GLM/DeepSeek-style NextN blocks stay in it.
		nextNWithoutKV := a.NextNPredictLayers > 0 && (a.interleavesAttention() ||
			slices.Contains([]string{"step35", "hy-v3", "glm-dsa", "mimo2", "deepseek32"}, a.Architecture))

		charge := func(device int, key, value, parameter uint64) {
			e.Devices[device].KVCache.Key += GGUFBytesScalar(key)
			e.Devices[device].KVCache.Value += GGUFBytesScalar(value)
			e.Devices[device].Parameter.KVCache += GGUFParametersScalar(parameter)
		}
		for i := uint64(0); i < a.BlockCount; i++ {
			if a.AttentionKVFromStartLayerCount > 0 && i >= uint64(a.AttentionKVFromStartLayerCount) {
				// The layer reuses the cache of a leading layer (Gemma3n/Gemma4),
				// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/llama-model.cpp#L2343-L2352.
				continue
			}
			if nextNWithoutKV && i+uint64(a.NextNPredictLayers) >= a.BlockCount {
				continue
			}
			device := 0
			if i < uint64(len(layerDevices)) {
				device = layerDevices[i]
			}
			if device > 0 && !*o.LMCOffloadKVCache {
				// The host holds the cache of every offloaded layer without KV offloading.
				device = 0
			}
			fullAttention, recurrent := a.memoryKindOfLayer(i)
			if recurrent {
				charge(device, rrs, srs, rrs+srs)
			}
			if !fullAttention {
				continue
			}
			kGQA, vGQA := akl*headsKVOfLayer(i), avl*headsKVOfLayer(i)
			n := nKV
			if usingSWA && a.isSWALayer(i) {
				kGQA, vGQA = swaAkl*headsKVOfLayer(i), swaAvl*headsKVOfLayer(i)
				n = swas
			}
			if kGQA == 0 && vGQA == 0 {
				// Attention-free layer, a Deci-style dummy block.
				continue
			}
			kps, vps := kGQA*n, vGQA*n
			charge(device,
				o.LMCCacheKeyType.RowSizeOf([]uint64{kps}),
				o.LMCCacheValueType.RowSizeOf([]uint64{vps}),
				kps+vps)
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
		switch {
		// A hybrid architecture holding a KV cache above needs the inputs of its full (self-)attention
		// layers here as well, otherwise it silently loses `inpPos + inpKQMask`.
		//
		// TODO(thxCode): so does a hybrid architecture declaring no interleaving, like Falcon-H1,
		// but charging it here changes the estimate of architectures shipping today.
		case a.interleavesAttention():
			e.Devices[0].Computation.Input = GGUFBytesScalar(inpTokens + inpEmbd + inpPos + inpKQMask + 2*inpSMask + inpSSeq + inpOutIds)
		case a.AttentionRecurrent:
			e.Devices[0].Computation.Input = GGUFBytesScalar(inpTokens + inpEmbd + 2*inpSMask + inpSSeq + inpOutIds)
		default:
			e.Devices[0].Computation.Input = GGUFBytesScalar(inpTokens + inpEmbd + inpPos + inpKQMask + inpOutIds)
		}
		{
			var v GGUFBytesScalar
			switch {
			case a.interleavesAttention():
				v = GGUFBytesScalar(inpEmbd + inpPos + inpKQMask + inpSMask + inpSSeq)
			case a.AttentionRecurrent:
				v = GGUFBytesScalar(inpEmbd + inpSMask + inpSSeq)
			default:
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
		switch {
		case len(tfLs) == 0:
			// A file with no transformer block holds nothing to size per layer.
			// llama.cpp refuses to load it for its missing blk.0 tensors, so the
			// device compute stays zero rather than reading past the layer list.
		case a.AttentionRecurrent && !a.AttentionHybrid:
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
						// RWKV6 reshapes time_mix_w2 to [ne0, ne1, 1, 5] and multiplies the
						// five-way lerp stack, so the node is 4-D and its last dimension is
						// that fan-out, see
						// https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/src/models/rwkv6-base.cpp#L59-L65.
						// RWKV7 gives the same tensor name to a plain decay projection whose
						// node is [n_embd, n_tokens], see
						// https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/src/models/rwkv7-base.cpp#L67-L70.
						// Its last dimension is the embedding length, so the 4-D shape charges
						// the node n_embd times over.
						var rs uint64
						switch a.Architecture {
						case "rwkv7", "arwkv7":
							rs = GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, nTokens})
						default:
							rs = GGMLTypeF32.RowSizeOf([]uint64{a.EmbeddingLength, 1, nTokens, l.Dimensions[l.NDimensions-1]})
						}
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
				cp = GGUFBytesScalar(estimateLLaMACppRecurrentComputeFloor(
					a, tfLs, nTokens, uint64(cp), !fullOffload && nTokens >= _LMCOffloadOpMinBatchSize))
				for i := range e.Devices[1:] {
					e.Devices[i+1].Computation.Compute = cp
				}
			} else {
				r := a.ssmConvolutionStateSize()
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
				cp = GGUFBytesScalar(estimateLLaMACppRecurrentComputeFloor(
					a, tfLs, nTokens, uint64(cp), !fullOffload && nTokens >= _LMCOffloadOpMinBatchSize))
				for i := range e.Devices[1:] {
					e.Devices[i+1].Computation.Compute = cp
				}
			}
		default:
			loadAttnInc, offloadAttnInc := uint64(0), uint64(0)
			// A memoryless graph has no KV cache rows to stage across a host
			// boundary.
			if !a.buildsMemorylessGraph() {
				rs := o.LMCCacheKeyType.RowSizeOf([]uint64{uint64(a.AttentionKeyLength), nKV, a.AttentionHeadCountKV})
				loadAttnInc = rs // k-?
				rs = o.LMCCacheValueType.RowSizeOf([]uint64{uint64(a.AttentionValueLength), nKV, a.AttentionHeadCountKV})
				loadAttnInc += rs // v-?
			}
			// A graph that adds a relative attention bias prices the quadratic
			// attention whatever the flag says: build_attn_mha declines the
			// flash path unless kq_b is null, see
			// https://github.com/ggml-org/llama.cpp/blob/c841aeeb8/src/llama-graph.cpp#L2564.
			kqBias := false
			for _, tl := range tfLs {
				if len(tl.Search(_LMCRelativeBiasWeightRegex)) != 0 {
					kqBias = true
					break
				}
			}
			flashPricing := o.FlashAttention && !kqBias
			// The non-flash path keeps sizing the offload device from ffnInc below.
			offloadFfnInc := uint64(0)
			if flashPricing {
				boundary := !fullOffload && nTokens >= _LMCOffloadOpMinBatchSize
				hostLayers := tfLs[:min(nLoadLayers, uint64(len(tfLs)))]
				if len(hostLayers) == 0 {
					// Every block is on the device, so block zero stands in for
					// the shapes; only the output layer can still hold a boundary.
					hostLayers = tfLs[:1]
				}
				// Gating is a property of the architecture's FFN; a hybrid's
				// first block can be a recurrent one with no FFN at all.
				gated := false
				for _, tl := range tfLs {
					if len(tl.Search(_LMCGateWeightRegex)) != 0 {
						gated = true
						break
					}
				}
				offloadAttnInc, offloadFfnInc = estimateLLaMACppFlashComputeMoments(
					o, a, hostLayers, tfLs, nKV, nTokens, loadAttnInc, boundary, gated)
				if a.Architecture == "gemma3n" {
					offloadAttnInc = max(offloadAttnInc,
						estimateLLaMACppGemma3nAltupMoment(a, ipLs, tfLs, nKV, nTokens, loadAttnInc, boundary))
				}
			} else {
				offloadAttnInc = uint64(0)
				for _, l := range searchLastBlock(tfLs, _LMCAttentionNodeWeightRegex) {
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
				if kqBias {
					// build_pos_bias holds three kq-sized rows beside the kq
					// node: the gathered bias rows, their contiguous
					// permutation, and the biased kq sum.
					rs := GGMLTypeF32.RowSizeOf([]uint64{nKV, nTokens, a.AttentionHeadCount})
					offloadAttnInc += 3 * rs
				}
			}
			ffnInc := uint64(0)
			for _, l := range searchLastBlock(tfLs, _LMCFeedForwardNodeWeightRegex) {
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
				if flashPricing {
					// The flash path stages the boundary copies inside its own moments.
					cp = GGUFBytesScalar(max(offloadAttnInc, offloadFfnInc))
				}
				if slices.Contains(_LMCBackendUnholdableArchitectures, a.Architecture) &&
					nTokens >= _LMCOffloadOpMinBatchSize {
					// Every layer stays on the CPU, and the batch offload path
					// streams each layer's weights through the device compute
					// buffer. The allocator holds close to the whole model, so
					// the buffer floors at the summed layer weights beside the
					// moments above. Measured on Nanbeige4.2-3B-Q4_K_M: the
					// buffer is 1854.23 MiB at every offload depth.
					var ws uint64
					for _, tl := range tfLs {
						for _, l := range tl.Search(_LMCLayerWeightRegex) {
							ws += l.Bytes()
						}
					}
					cp += GGUFBytesScalar(ws)
				}
				for i := range e.Devices[1:] {
					e.Devices[i+1].Computation.Compute = cp
				}
				if !flashPricing && nLoadLayers > 1 {
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
			// Host, for the same reason as the output buffer above: what the output
			// projection produces is logits, and llama_context::output_reserve keeps
			// them in system memory whatever the offload. Charging them to the output
			// device counted the same bytes a second time, on a card that never holds
			// them.
			//
			// A speculator head is the exception. llama.cpp reserves the graph with
			// n_outputs = 1 for a normal decoder, so the logits never reach the card,
			// but a draft head emits logits at every position and the projection runs
			// on the device that holds it.
			outDev := 0
			if a.emitsLogitsAtEveryPosition() {
				outDev = idxOutputDevice
			}
			e.Devices[outDev].Computation.Output += GGUFBytesScalar(outInc)
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

	// Flash attention.
	e.FlashAttention = o.FlashAttention

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
			// nPatchesMerged is how many patches the projector merges into one output token.
			//
			// The vision encoder attends over the patches before the merge,
			// so nPatches counts the projector's output tokens while the encoder's positions
			// are nPatches scaled back up by this factor.
			nPatchesMerged uint64 = 1
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
		// A dynamic-resolution projector reserves its graph for one square warm-up
		// image whose side llama.cpp derives from a token count it carries itself,
		// not from anything the file declares: sqrt(tokens) * patch_size * n_merge,
		// see https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/tools/mtmd/clip-model.h#L204-L209.
		// That one image is what the graph is reserved from, see
		// https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/tools/mtmd/clip.cpp#L3618-L3639.
		// A caller's own maximum still wins, the way "--image-max-tokens" does.
		warmupSized := false
		if w, ok := _GGUFClipWarmupImages[a.ClipProjectorType]; ok && o.LMCVisualMaxImageSize == nil && nPatchSize > 0 {
			merge := w.MergeDefault
			if a.ClipVisionSpatialMergeSize > 0 {
				merge = uint64(a.ClipVisionSpatialMergeSize)
			}
			heightMaxSize = w.TokensPerSide * nPatchSize * merge
			widthMaxSize = heightMaxSize
			warmupSized = true
		}
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
			nSizePatch := 2 * uint64(a.ClipVisionPatchSize)
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
			// The average pooling runs after the vision transformer, so nPatches counts
			// the projector's output tokens while the encoder still attends over the
			// whole patch grid, see
			// https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/tools/mtmd/clip.cpp#L936-L943.
			if sf := uint64(a.ClipVisionProjectorScaleFactor); sf > 1 {
				nPatchesMerged = sf * sf
			}
			if ti, ok := gf.TensorInfos.Get("mm.input_projection.weight"); ok {
				projectionDim = ti.Dimensions[0]
			}
		case "idefics3", "llama4":
			nPatches /= uint64(a.ClipVisionProjectorScaleFactor) * uint64(a.ClipVisionProjectorScaleFactor)
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
			nPatches /= uint64(a.ClipVisionProjectorScaleFactor) * uint64(a.ClipVisionProjectorScaleFactor)
			if ti, ok := gf.TensorInfos.Get("mm.model.mlp.3.weight"); ok {
				projectionDim = ti.Dimensions[1]
			}
		case "gemma3nv":
			// MobileNetV5 is a convolutional encoder that always outputs a fixed 16x16 token grid
			// regardless of the input size; its patch size is a convolution stride,
			// not a transformer patch. image_size/patch_size is llama.cpp's own expression for
			// the TOTAL token count of that grid (768/3 = 256 = 16x16), not a per-side count,
			// see https://github.com/ggml-org/llama.cpp/blob/e3546c7948e3af463d0b401e6421d5a4c2faf565/tools/mtmd/clip.cpp#L3389-L3394.
			nPatches = uint64(a.ClipVisionImageSize) / nPatchSize
			projectionDim = uint64(a.ClipVisionProjectionDim)
		default:
			if !a.ClipHasQwen2VLMerger && !a.ClipHasLLaVAProjector && !a.ClipHasMiniCPMVProjector {
				// Approximate a projector type without a special case above from what the GGUF declares,
				// so that new llama.cpp projector types (lightonocr, paddleocr, dots_ocr, kimivl, ...)
				// degrade to a rough estimate rather than an inflated one,
				// see https://github.com/gpustack/gguf-parser-go/issues/21.
				//
				// Cap the image size like the known dynamic-resolution projector types,
				// without inflating a smaller native image size by default.
				// A projector declaring no image size at all is dynamic resolution
				// (dots_ocr does this); assume the default cap rather than zero-pixel images,
				// which would charge nothing for the encoder.
				if !warmupSized {
					ms := uint64(ptr.Deref(o.LMCVisualMaxImageSize, 1024))
					if o.LMCVisualMaxImageSize == nil && heightMaxSize > 0 && heightMaxSize < ms {
						ms = heightMaxSize
					}
					heightMaxSize = ms
					widthMaxSize = ms
				}
				heightPatchSize := (heightMaxSize + nPatchSize - 1) / nPatchSize
				widthPatchSize := (widthMaxSize + nPatchSize - 1) / nPatchSize
				// Honor the patch reduction the metadata declares,
				// like llama.cpp does for every projector type carrying it,
				// rounding up so the estimate never undercounts.
				//
				// This reduces the projector's output tokens, not the encoder's positions:
				// the merge happens after the vision transformer.
				merge := uint64(1)
				if a.ClipVisionSpatialMergeSize > 1 {
					merge = uint64(a.ClipVisionSpatialMergeSize)
				} else if a.ClipVisionProjectorScaleFactor > 1 {
					merge = uint64(a.ClipVisionProjectorScaleFactor)
				}
				if merge > 1 {
					heightPatchSize = (heightPatchSize + merge - 1) / merge
					widthPatchSize = (widthPatchSize + merge - 1) / merge
					nPatchesMerged = merge * merge
				}
				nPatches = heightPatchSize * widthPatchSize
				if _, ok := gf.TensorInfos.Get("v.token_embd.img_break"); ok && heightPatchSize > 0 {
					nPatches += heightPatchSize - 1 /* [IMG_BREAK] per row */
				}
				projectionDim = uint64(a.ClipVisionProjectionDim)
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
				} else if nPatchesMerged > 1 {
					// The merged patch count is what the projector emits;
					// the encoder attends over the patches before the merge.
					// The class embedding, if any, is a single position and is not merged.
					nPositions = nPatches * nPatchesMerged
					if hasClassEmbd {
						nPositions += 1
					}
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
				if o.FlashAttention && nHead > 0 {
					// The attention score matrix is not materialized with flash attention,
					// the largest quadratic tensor left is the F16-cast mask,
					// see https://github.com/ggml-org/llama.cpp/blob/e3546c7948e3af463d0b401e6421d5a4c2faf565/tools/mtmd/clip.cpp#L690-L700.
					//
					// A projector declaring no attention head count is not charged for attention at all,
					// so flash attention must not add a buffer here either.
					compKQcur = GGMLTypeF16.RowSizeOf([]uint64{nPositions, nPositions})
				}
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
				// The encoder attends over one audio chunk, not the position table's whole
				// capacity; the two coincide for the whisper family (30s chunks, 1500-row
				// tables) but not for conformer projectors with 1-second chunks,
				// see https://github.com/ggml-org/llama.cpp/blob/e3546c7948e3af463d0b401e6421d5a4c2faf565/tools/mtmd/clip.cpp#L1557,
				//     https://github.com/ggml-org/llama.cpp/blob/e3546c7948e3af463d0b401e6421d5a4c2faf565/tools/mtmd/clip.cpp#L1611-L1618,
				//     https://github.com/gpustack/gguf-parser-go/issues/26.
				audioChunkSeconds := uint64(30)
				if a.ClipProjectorType == "lfm2a" {
					audioChunkSeconds = 1
				}
				// One second of 16 kHz audio at hop length 160 is 100 mel frames.
				if chunkPositions := audioChunkSeconds * 100; nPositions > chunkPositions {
					nPositions = chunkPositions
				}
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
				if o.FlashAttention && nHead > 0 {
					// The audio encoder shares the clip attention graph,
					// see https://github.com/ggml-org/llama.cpp/blob/e3546c7948e3af463d0b401e6421d5a4c2faf565/tools/mtmd/clip.cpp#L690-L700.
					compKQcur = GGMLTypeF16.RowSizeOf([]uint64{nPositions, nPositions})
				}
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
		// Decision is the flag to indicate whether the model is used for decision,
		// i.e. serving the "POST /v1/systemone" endpoint in llama-server,
		// true for decision.
		//
		// It is presence-based: any string-typed "<architecture>.decision.type"
		// metadata marks the model, mirroring common_get_decision_type in
		// llama.cpp. It outranks Reranking: a declared decision head carries
		// classifier tensors and rank pooling that would otherwise read as
		// reranking.
		Decision bool `json:"decision"`
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
	es.Decision = e.Decision
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
