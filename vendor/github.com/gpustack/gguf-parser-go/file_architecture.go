package gguf_parser

import (
	"regexp"
	"slices"
	"strings"
)

// Types for the architecture metadata of a GGUF file.
type (
	// GGUFArchitecture represents the architecture metadata of a GGUF file.
	GGUFArchitecture struct {
		/* Basic */

		// Type describes the type of the file,
		// default is "model".
		Type string `json:"type"`
		// Architecture describes what architecture this model implements.
		//
		// All lowercase ASCII.
		Architecture string `json:"architecture"`
		// MaximumContextLength(n_ctx_train) is the maximum context length of the model.
		//
		// For most architectures, this is the hard limit on the length of the input.
		// Architectures, like RWKV,
		// that are not reliant on transformer-style attention may be able to handle larger inputs,
		// but this is not guaranteed.
		MaximumContextLength uint64 `json:"maximumContextLength,omitempty"`
		// EmbeddingLength(n_embd) is the length of the embedding layer.
		EmbeddingLength uint64 `json:"embeddingLength,omitempty"`
		// BlockCount(n_layer) is the number of blocks of attention and feed-forward layers,
		// i.e. the bulk of the LLM.
		// This does not include the input or embedding layers.
		BlockCount uint64 `json:"blockCount,omitempty"`
		// FeedForwardLength(n_ff) stores the length of each feed-forward layer.
		FeedForwardLength []uint64 `json:"feedForwardLength,omitempty"`
		// ExpertFeedForwardLength(expert_feed_forward_length) is the length of the feed-forward layer in the expert model.
		ExpertFeedForwardLength uint64 `json:"expertFeedForwardLength,omitempty"`
		// ExpertSharedFeedForwardLength(expert_shared_feed_forward_length) is the length of the shared feed-forward layer in the expert model.
		ExpertSharedFeedForwardLength uint64 `json:"expertSharedFeedForwardLength,omitempty"`
		// ExpertCount(n_expert) is the number of experts in MoE models.
		ExpertCount uint32 `json:"expertCount,omitempty"`
		// ExpertUsedCount(n_expert_used) is the number of experts used during each token evaluation in MoE models.
		ExpertUsedCount uint32 `json:"expertUsedCount,omitempty"`
		// ExpertSharedCount(n_expert_shared) is the number of shared experts in MoE models.
		ExpertSharedCount uint32 `json:"expertSharedCount,omitempty"`
		// AttentionHeadCount(n_head) is the number of attention heads.
		AttentionHeadCount uint64 `json:"attentionHeadCount,omitempty"`
		// AttentionHeadCountKV(n_head_kv) is the number of attention heads per group used in Grouped-Query-Attention.
		//
		// If not provided or equal to AttentionHeadCount,
		// the model does not use Grouped-Query-Attention.
		AttentionHeadCountKV uint64 `json:"attentionHeadCountKV,omitempty"`
		// AttentionHeadCountKVs is the per-layer AttentionHeadCountKV,
		// when the file declares "<architecture>.attention.head_count_kv" as an array.
		//
		// A layer with zero KV heads holds no KV cache: it is a recurrent (or attention-free) layer,
		// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/lfm2.cpp#L9-L11.
		AttentionHeadCountKVs []uint64 `json:"attentionHeadCountKVs,omitempty"`
		// AttentionSlidingWindowPattern is the pattern used in the sliding window attention.
		//
		// 0 means all layers are Sliding Window Attention.
		// 1 means all layers are none Sliding Window Attention.
		// N means every Nth layer is none Sliding Window Attention.
		//
		// AttentionSlidingWindowLayers below carries the per-layer resolution,
		// including the dense-first phasing this period cannot express.
		AttentionSlidingWindowPattern uint32 `json:"attentionSlidingWindowPattern,omitempty"`
		// AttentionSlidingWindowLayers is the per-layer use of the sliding window attention.
		//
		// Resolved from "<architecture>.attention.sliding_window_pattern" when the file declares it,
		// and from the per-architecture period of llama.cpp otherwise, see swaPatternDefaults.
		// Empty means no layer uses the sliding window attention.
		AttentionSlidingWindowLayers []bool `json:"attentionSlidingWindowLayers,omitempty"`
		// AttentionSlidingWindow is the size of the sliding window used in the attention layer.
		AttentionSlidingWindow uint64 `json:"attentionSlidingWindow,omitempty"`
		// AttentionKeyLengthSWA(n_embd_head_k_swa) is the size of a key head on the sliding window layers.
		//
		// Defaults to AttentionKeyLength.
		AttentionKeyLengthSWA uint32 `json:"attentionKeyLengthSWA,omitempty"`
		// AttentionValueLengthSWA(n_embd_head_v_swa) is the size of a value head on the sliding window layers.
		//
		// Defaults to AttentionValueLength.
		AttentionValueLengthSWA uint32 `json:"attentionValueLengthSWA,omitempty"`
		// AttentionKVFromStartLayerCount(n_layer_kv_from_start) is the number of leading layers
		// holding a KV cache, the layers behind reuse those caches.
		//
		// 0 means every layer holds its own KV cache.
		AttentionKVFromStartLayerCount uint32 `json:"attentionKVFromStartLayerCount,omitempty"`
		// NextNPredictLayers(n_layer_nextn) is the number of trailing NextN/MTP blocks inside BlockCount.
		NextNPredictLayers uint32 `json:"nextNPredictLayers,omitempty"`
		// AttentionMaxALiBIBias is the maximum bias to use for ALiBI.
		AttentionMaxALiBIBias float32 `json:"attentionMaxALiBIBias,omitempty"`
		// AttentionClampKQV describes a value `C`,
		// which is used to clamp the values of the `Q`, `K` and `V` tensors between `[-C, C]`.
		AttentionClampKQV float32 `json:"attentionClampKQV,omitempty"`
		// AttentionLayerNormEpsilon is the epsilon value used in the LayerNorm(Layer Normalization).
		AttentionLayerNormEpsilon float32 `json:"attentionLayerNormEpsilon,omitempty"`
		// AttentionLayerNormRMSEpsilon is the epsilon value used in the RMSNorm(root Mean Square Layer Normalization),
		// which is a simplification of the original LayerNorm.
		AttentionLayerNormRMSEpsilon float32 `json:"attentionLayerNormRMSEpsilon,omitempty"`
		// AttentionQueryLORARank is the LORA rank of the query matrix.
		//
		// Zero means no LORA.
		AttentionQueryLORARank uint32 `json:"attentionQueryLORARank,omitempty"`
		// AttentionKeyValueLORARank is the LORA rank of the key/value matrix.
		//
		// Zero means no LORA.
		AttentionKeyValueLORARank uint32 `json:"attentionKeyValueLORARank,omitempty"`
		// AttentionKeyLength(n_embd_head_k) is the size of a key head.
		//
		// Defaults to `EmbeddingLength / AttentionHeadCount`.
		AttentionKeyLength uint32 `json:"attentionKeyLength,omitempty"`
		// AttentionKeyLengthMLA(n_embd_head_k_mla) is the size of a key head in MLA(Multi-Layer Attention).
		//
		// Zero means no MLA.
		AttentionKeyLengthMLA uint32 `json:"attentionKeyLengthMLA,omitempty"`
		// AttentionValueLength(n_embd_head_v) is the size of a value head.
		//
		// Defaults to `EmbeddingLength / AttentionHeadCount`.
		AttentionValueLength uint32 `json:"attentionValueLength,omitempty"`
		// AttentionValueLengthMLA(n_embd_head_v_mla) is the size of a value head in MLA(Multi-Layer Attention).
		//
		// Zero means no MLA.
		AttentionValueLengthMLA uint32 `json:"attentionValueLengthMLA,omitempty"`
		// AttentionCausal is true if the attention is causal.
		AttentionCausal bool `json:"attentionCausal,omitempty"`
		// AttentionRecurrent is true if the attention is recurrent.
		//
		// Used in Mamba, RWKV, and similar architectures.
		AttentionRecurrent bool `json:"attentionRecurrent,omitempty"`
		// AttentionHybrid is true if the attention is hybrid (causal (self-attention) + recurrent).
		//
		// Used in Jamba, Falcon-H1, Qwen3.5, and similar architectures.
		AttentionHybrid bool `json:"attentionHybrid,omitempty"`
		// FullAttentionInterval is the interval at which a hybrid architecture places
		// a full (self-)attention layer, the layers in between are recurrent.
		//
		// 0 means the architecture does not declare the interleaving.
		// N means every Nth layer is a full (self-)attention layer, counting the layers from 1,
		// unlike AttentionSlidingWindowPattern above counting them from 0.
		FullAttentionInterval uint32 `json:"fullAttentionInterval,omitempty"`
		// RoPEDimensionCount is the number of dimensions in the RoPE(Rotary Positional Encoding).
		RoPEDimensionCount uint64 `json:"ropeDimensionCount,omitempty"`
		// RoPEFrequencyBase is the base frequency of the RoPE.
		RoPEFrequencyBase float32 `json:"ropeFrequencyBase,omitempty"`
		// RoPEFrequencyScale is the scale frequency of the RoPE.
		RoPEFrequencyScale float32 `json:"ropeFrequencyScale,omitempty"`
		// RoPEFrequencyScale is the frequency scale of the RoPE.
		RoPEScalingType string `json:"ropeScalingType,omitempty"`
		// RoPEScalingFactor is the scaling factor of the RoPE.
		RoPEScalingFactor float32 `json:"ropeScalingFactor,omitempty"`
		// RoPEScalingOriginalContextLength is the original context length of the RoPE scaling.
		RoPEScalingOriginalContextLength uint64 `json:"ropeScalingOriginalContextLength,omitempty"`
		// RoPEScalingFinetuned is true if the RoPE scaling is fine-tuned.
		RoPEScalingFinetuned bool `json:"ropeScalingFinetuned,omitempty"`
		// PoolingType is the type of pooling used in the model.
		PoolingType uint32 `json:"poolingType,omitempty"`
		// DecisionType is the declared type of the decision head of the model,
		// e.g. "openjev", "laya".
		//
		// It comes from the "<architecture>.decision.type" metadata, which the
		// llama.cpp conversion tools write and llama-server reads to decide
		// whether the model can serve the "POST /v1/systemone" endpoint,
		// see https://github.com/ggml-org/llama.cpp/blob/master/gguf-py/gguf/constants.py
		// (Keys.Decision.TYPE) and common_get_decision_type.
		//
		// Empty means the model declares no decision head with a string value,
		// either the key is absent, mistyped, or holds an empty string. Marking
		// a model as decision is presence-based, see LLaMACppRunEstimate.Decision.
		// An unrecognized value still declares a decision model, one that the
		// current llama.cpp build cannot serve.
		DecisionType string `json:"decisionType,omitempty"`
		// SSMConvolutionKernel is the size of the convolution kernel used in the Selective State Space Model (SSM) and similar architectures.
		SSMConvolutionKernel uint32 `json:"ssmConvolutionKernel,omitempty"`
		// SSMInnerSize is the embedding size of the state in SSM and similar architectures.
		SSMInnerSize uint32 `json:"ssmInnerSize,omitempty"`
		// SSMStateSize is the size of the recurrent state in SSM and similar architectures.
		SSMStateSize uint32 `json:"ssmStateSize,omitempty"`
		// SSMTimeStepRank is the rank of the time steps in SSM and similar architectures.
		SSMTimeStepRank uint32 `json:"ssmTimeStepRank,omitempty"`
		// SSMGroupCount is the number of groups in the SSM and similar architectures.
		SSMGroupCount uint32 `json:"ssmGroupCount,omitempty"`
		// ShortConvLCache is the cache length of the short convolution used in LFM2 and similar architectures.
		ShortConvLCache uint32 `json:"shortConvLCache,omitempty"`
		// KDAHeadDim is the head dimension of the KDA(Kimi Delta Attention) used in Kimi-Linear and similar architectures.
		KDAHeadDim uint32 `json:"kdaHeadDim,omitempty"`
		// WKVHeadSize is the size of the head in RWKV and similar architectures.
		RWKVHeadSize uint32 `json:"rwkvHeadSize,omitempty"`
		// RWKVRescaleEveryNLayers is the number of layers after which the rescaling is applied in RWKV and similar architectures.
		RWKVRescaleEveryNLayers uint32 `json:"rwkvRescaleEveryNLayers,omitempty"`
		// RWKVTimeMixExtraDimension indicates whether the RWKV architecture has an extra dimension for time mixing.
		RWKVTimeMixExtraDimension uint32 `json:"rwkvTimeMixExtraDimension,omitempty"`
		// RWKVTimeDecayExtraDimension indicates whether the RWKV architecture has an extra dimension for time decay.
		RWKVTimeDecayExtraDimension uint32 `json:"rwkvTimeDecayExtraDimension,omitempty"`
		// TokenShiftCount is the number of token shifts used in RWKV and similar architectures.
		RWKVTokenShiftCount uint32 `json:"rwkvTokenShiftCount,omitempty"`
		// VocabularyLength is the size of the vocabulary.
		//
		// VocabularyLength is the same as the tokenizer's token size.
		VocabularyLength uint64 `json:"vocabularyLength,omitempty"`

		/* Appendix */

		// ClipProjectorType is the type of the projector used in the clip model.
		//
		// Only used when Architecture is "clip".
		ClipProjectorType string `json:"clipProjectorType,omitempty"`
		// ClipHasLLaVAProjector indicates whether the clip model has LLaVA projector or not.
		//
		// Only used when Architecture is "clip".
		//
		// Deprecated: use ClipProjectorType instead.
		ClipHasLLaVAProjector bool `json:"clipHasLLaVAProjector,omitempty"`
		// ClipHasMiniCPMVProjector indicates whether the clip model has MiniCPMV projector or not.
		//
		// Only used when Architecture is "clip".
		//
		// Deprecated: use ClipProjectorType instead.
		ClipHasMiniCPMVProjector bool `json:"clipHasMiniCPMVProject,omitempty"`
		// ClipMiniCPMVVersion is the version of the MiniCPMV projector.
		//
		// Only used when Architecture is "clip".
		ClipMiniCPMVVersion int32 `json:"clipMiniCPMVVersion,omitempty"`
		// ClipMiniCPMVQueryNum is the number of queries used in the MiniCPMV projector.
		//
		// Only used when Architecture is "clip".
		ClipMiniCPMVQueryNum int32 `json:"clipMiniCPMVQueryNum,omitempty"`
		// ClipHasGLMProjector indicates whether the clip model has GLM projector or not.
		//
		// Only used when Architecture is "clip".
		//
		// Deprecated: use ClipProjectorType instead.
		ClipHasGLMProjector bool `json:"clipHasGLMProjector,omitempty"`
		// ClipHasQwen2VLMerger indicates whether the clip model has Qwen2VL merger or not.
		//
		// Only used when Architecture is "clip".
		//
		// Deprecated: use ClipProjectorType instead.
		ClipHasQwen2VLMerger bool `json:"clipHasQwen2VLMerger,omitempty"`
		// ClipHasVisionEncoder indicates whether the clip model has vision encoder or not.
		//
		// Only used when Architecture is "clip".
		ClipHasVisionEncoder bool `json:"clipHasVisionEncoder,omitempty"`
		// ClipVisionEmbeddingLength indicates the embedding length of vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionEmbeddingLength uint64 `json:"clipVisionEmbeddingLength,omitempty"`
		// ClipVisionBlockCount indicates the number of blocks in the vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionBlockCount uint64 `json:"clipVisionBlockCount,omitempty"`
		// ClipVisionFeedForwardLength indicates the feed-forward length of the vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionFeedForwardLength []uint64 `json:"clipVisionFeedForwardLength,omitempty"`
		// ClipVisionAttentionHeadCount indicates the number of attention heads in the vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionAttentionHeadCount uint64 `json:"clipVisionAttentionHeadCount,omitempty"`
		// ClipVisionAttentionLayerNormRMSEpsilon indicates the epsilon value used in the RMSNorm of the vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionAttentionLayerNormRMSEpsilon float32 `json:"clipVisionAttentionLayerNormRMSEpsilon,omitempty"`
		// ClipVisionProjectionDim indicates the projection dimension of vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionProjectionDim uint32 `json:"clipVisionProjectionDim,omitempty"`
		// ClipVisionProjectorScaleFactor is the scale factor of the projector.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionProjectorScaleFactor uint32 `json:"clipVisionProjectorScaleFactor,omitempty"`
		// ClipVisionImageSize indicates the image size of vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionImageSize uint32 `json:"clipVisionImageSize,omitempty"`
		// ClipVisionPatchSize indicates the patch size of vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionPatchSize uint32 `json:"clipVisionPatchSize,omitempty"`
		// ClipVisionMMPatchMergeType indicates the merge type of the vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionMMPatchMergeType string `json:"clipVisionMMPatchMergeType,omitempty"`
		// ClipVisionSpatialMergeSize is the spatial merge size of the vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionSpatialMergeSize uint32 `json:"clipVisionSpatialMergeSize,omitempty"`
		// ClipVisionWindowAttentionPattern is the Window Attention pattern used in the vision encoder.
		//
		// Only used when Architecture is "clip" and ClipHasVisionEncoder is true.
		ClipVisionWindowAttentionPattern uint32 `json:"clipVisionWindowAttentionPattern,omitempty"`
		// ClipHasAudioEncoder indicates whether the clip model has audio encoder or not.
		//
		// Only used when Architecture is "clip".
		ClipHasAudioEncoder bool `json:"clipHasAudioEncoder,omitempty"`
		// ClipAudioEmbeddingLength indicates the embedding length of audio encoder.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioEmbeddingLength uint64 `json:"clipAudioEmbeddingLength,omitempty"`
		// ClipAudioBlockCount indicates the number of blocks in the audio encoder.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioBlockCount uint64 `json:"clipAudioBlockCount,omitempty"`
		// ClipAudioFeedForwardLength indicates the feed-forward length of the audio encoder.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioFeedForwardLength []uint64 `json:"clipAudioFeedForwardLength,omitempty"`
		// ClipAudioAttentionHeadCount indicates the number of attention heads in the audio encoder.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioAttentionHeadCount uint64 `json:"clipAudioAttentionHeadCount,omitempty"`
		// ClipAudioAttentionLayerNormRMSEpsilon indicates the epsilon value used in the RMSNorm of the audio encoder.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioAttentionLayerNormRMSEpsilon float32 `json:"clipAudioAttentionLayerNormRMSEpsilon,omitempty"`
		// ClipAudioProjectionDim indicates the projection dimension of audio encoder.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioProjectionDim uint32 `json:"clipAudioProjectionDim,omitempty"`
		// ClipAudioProjectorStackFactor is the scale factor of the projector.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioProjectorStackFactor uint32 `json:"clipAudioProjectorStackFactor,omitempty"`
		// ClipAudioNumMelBins is the number of mel bins used in the audio encoder.
		//
		// Only used when Architecture is "clip" and ClipHasAudioEncoder is true.
		ClipAudioNumMelBins uint32 `json:"clipAudioNumMelBins,omitempty"`

		// AdapterType is the type of the adapter.
		//
		// Only used when Architecture is "adapter".
		AdapterType string `json:"adapterType,omitempty"`
		// AdapterLoRAAlpha is the alpha value of the LoRA adapter.
		//
		// Only used when AdapterType is "lora".
		AdapterLoRAAlpha float32 `json:"adapterLoRAAlpha,omitempty"`
		// AdapterControlVectorLayerCount is the number of layers in the control vector.
		//
		// Only used when Architecture is "control_vector".
		AdapterControlVectorLayerCount uint32 `json:"adapterControlVectorLayerCount,omitempty"`

		// DiffusionArchitecture is the actual architecture of the diffusion model.
		//
		// Only used when Architecture is "diffusion".
		DiffusionArchitecture string `json:"diffusionArchitecture,omitempty"`
		// DiffusionTransformer indicates whether the diffusion model is a diffusion transformer or not.
		//
		DiffusionTransformer bool `json:"diffusionTransformer,omitempty"`
		// DiffusionConditioners is the list of diffusion conditioners.
		//
		// Only used when Architecture is "diffusion".
		DiffusionConditioners GGUFArchitectureDiffusionConditioners `json:"diffusionConditioners,omitempty"`
		// DiffusionAutoencoder represents the autoencoder of the diffusion model.
		//
		// Only used when Architecture is "diffusion".
		DiffusionAutoencoder *GGUFArchitectureDiffusionAutoencoder `json:"diffusionAutoencoder,omitempty"`
	}

	// GGUFArchitectureDiffusionConditioners is the list of GGUFArchitectureDiffusionConditioner.
	GGUFArchitectureDiffusionConditioners []GGUFArchitectureDiffusionConditioner

	// GGUFArchitectureDiffusionConditioner represents the conditioner metadata of the diffusion architecture.
	GGUFArchitectureDiffusionConditioner struct {
		// Architecture is the architecture of the diffusion conditioner.
		Architecture string `json:"architecture"`

		// FileType describes the type of the majority of the tensors in the GGUF file.
		FileType GGUFFileType `json:"fileType"`
	}

	// GGUFArchitectureDiffusionAutoencoder represents the autoencoder metadata of the diffusion architecture.
	GGUFArchitectureDiffusionAutoencoder struct {
		// Architecture is the architecture of the diffusion autoencoder.
		//
		// Currently, only "VAE" is supported.
		Architecture string `json:"architecture"`

		// FileType describes the type of the majority of the tensors in the GGUF file.
		FileType GGUFFileType `json:"fileType"`
	}
)

// maxSaneBlockCount bounds the per-layer slices this package derives from metadata.
// llama.cpp refuses a file beyond LLAMA_MAX_LAYERS (512) layers, so a far larger block_count
// is a corrupt or hostile file; the bound keeps such a file from sizing an allocation,
// in the spirit of the header validation of https://github.com/gpustack/gguf-parser-go/pull/18.
const maxSaneBlockCount = 4096

// interleavesFullAttention returns true if the architecture is hybrid and declares how it
// interleaves its full (self-)attention layers with its recurrent layers.
func (ga GGUFArchitecture) interleavesFullAttention() bool {
	return ga.AttentionHybrid && ga.FullAttentionInterval > 1
}

// interleavesKVHeads returns true if the architecture is hybrid and declares which of its layers
// are recurrent with zero entries in its per-layer attention.head_count_kv,
// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/kimi-linear.cpp#L15-L19.
func (ga GGUFArchitecture) interleavesKVHeads() bool {
	return ga.AttentionHybrid && slices.Contains(ga.AttentionHeadCountKVs, 0)
}

// interleavesAttention returns true if the architecture declares which of its layers are
// full (self-)attention and which are recurrent.
func (ga GGUFArchitecture) interleavesAttention() bool {
	return ga.interleavesFullAttention() || ga.interleavesKVHeads()
}

// usesSlidingWindowAttention returns true if any layer uses the sliding window attention.
func (ga GGUFArchitecture) usesSlidingWindowAttention() bool {
	return slices.Contains(ga.AttentionSlidingWindowLayers, true)
}

// isSWALayer returns true if the layer at the given index uses the sliding window attention.
func (ga GGUFArchitecture) isSWALayer(i uint64) bool {
	return i < uint64(len(ga.AttentionSlidingWindowLayers)) && ga.AttentionSlidingWindowLayers[i]
}

// memoryKindOfLayer returns whether the layer at the given index holds a full (self-)attention KV cache,
// and whether it holds a recurrent (conv + ssm) state.
//
// A hybrid architecture declaring its interleaving holds exactly one of the two per layer,
// as llama.cpp allocates them from the complementary `filter_attn` and `filter_recr` predicates,
// see https://github.com/ggml-org/llama.cpp/blob/272700b360944e40816a7ea13da8cd723119000a/src/llama-model.cpp#L2176-L2182.
func (ga GGUFArchitecture) memoryKindOfLayer(i uint64) (fullAttention, recurrent bool) {
	switch {
	case ga.interleavesKVHeads():
		fullAttention = i < uint64(len(ga.AttentionHeadCountKVs)) && ga.AttentionHeadCountKVs[i] > 0
		return fullAttention, !fullAttention
	case ga.interleavesFullAttention():
		fullAttention = (i+1)%uint64(ga.FullAttentionInterval) == 0
		return fullAttention, !fullAttention
	case ga.AttentionHybrid:
		// Both on every layer, as before, when the interleaving is not declared.
		return true, true
	case ga.AttentionRecurrent:
		return false, true
	}
	return true, false
}

// DiffusionHasConditioners returns true if the diffusion model has conditioners.
func (ga GGUFArchitecture) DiffusionHasConditioners() bool {
	return len(ga.DiffusionConditioners) > 0
}

// DiffusionHasAutoencoder returns true if the diffusion model has an autoencoder.
func (ga GGUFArchitecture) DiffusionHasAutoencoder() bool {
	return ga.DiffusionAutoencoder != nil && ga.DiffusionAutoencoder.Architecture != ""
}

func (gacs GGUFArchitectureDiffusionConditioners) String() string {
	var sb strings.Builder
	for i, gac := range gacs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(gac.String())
	}
	return sb.String()
}

func (gac GGUFArchitectureDiffusionConditioner) String() string {
	return gac.Architecture + " (" + gac.FileType.String() + ")"
}

func (gaa GGUFArchitectureDiffusionAutoencoder) String() string {
	return gaa.Architecture + " (" + gaa.FileType.String() + ")"
}

// Architecture returns the architecture metadata of the GGUF file.
func (gf *GGUFFile) Architecture() (ga GGUFArchitecture) {
	for _, re := range _GGUFPotentialDiffusionArchitectureTensorsRegexes {
		if gf.TensorInfos.Match(re) {
			return gf.diffuserArchitecture()
		}
	}
	var (
		generalTypeKey         = "general.type"
		generalArchitectureKey = "general.architecture"

		controlVectorModelHintKey = "controlvector.model_hint"
	)
	m, _ := gf.Header.MetadataKV.Index([]string{
		generalTypeKey,
		generalArchitectureKey,
		controlVectorModelHintKey,
	})

	typ, arch := "model", "llama" // nolint: goconst
	{
		if v, ok := m[generalTypeKey]; ok {
			typ = v.ValueString()
		}
		if v, ok := m[generalArchitectureKey]; ok {
			arch = v.ValueString()
		}
	}

	switch {
	case arch == "clip":
		return gf.clipArchitecture()
	case arch == "controlvector":
		arch = "llama"
		if v, ok := m[controlVectorModelHintKey]; ok {
			arch = v.ValueString()
		}
		return gf.adapterArchitecture(arch)
	case typ == "adapter":
		return gf.adapterArchitecture(arch)
	case typ == "imatrix":
		return gf.imatrixArchitecture(arch)
	}
	return gf.transformerArchitecture(arch)
}

func (gf *GGUFFile) diffuserArchitecture() (ga GGUFArchitecture) {
	const (
		// Diffusion

		sdKey                = "model.diffusion_model.output_blocks.11.1.transformer_blocks.0.attn2.to_v.weight" // SD 1.x/2.x
		sdKey2               = "output_blocks.11.1.transformer_blocks.0.attn2.to_v.weight"
		sdXlKey              = "model.diffusion_model.output_blocks.5.1.transformer_blocks.1.attn1.to_v.weight" // SD XL
		sdXlKey2             = "output_blocks.5.1.transformer_blocks.1.attn1.to_v.weight"
		sdXlRefinerKey       = "model.diffusion_model.output_blocks.8.1.transformer_blocks.1.attn1.to_v.weight" // SD XL Refiner
		sdXlRefinerKey2      = "output_blocks.8.1.transformer_blocks.1.attn1.to_v.weight"
		sd3Key               = "model.diffusion_model.joint_blocks.23.x_block.attn.proj.weight" // SD 3.x
		sd3Key2              = "joint_blocks.23.x_block.attn.proj.weight"
		sdInPaintFeatureKey  = "model.diffusion_model.input_blocks.0.0.weight" // SD in-paint feature
		sdInPaintFeatureKey2 = "input_blocks.0.0.weight"

		fluxKey             = "model.diffusion_model.double_blocks.0.txt_attn.proj.weight" // FLUX.1
		fluxKey2            = "double_blocks.0.txt_attn.proj.weight"
		fluxFillFeatureKey  = "model.diffusion_model.img_in.weight" // FLUX.1 Fill feature
		fluxFillFeatureKey2 = "img_in.weight"

		// Conditioner

		openAiClipVitL14Key  = "cond_stage_model.transformer.text_model.encoder.layers.11.self_attn.k_proj.weight" // OpenAI CLIP ViT-L/14
		openAiClipVitL14Key2 = "text_model.encoder.layers.11.self_attn.k_proj.weight"
		openClipVitH14Key    = "cond_stage_model.transformer.text_model.encoder.layers.22.self_attn.k_proj.weight" // OpenCLIP ViT-H/14
		openClipVitH14Key2   = "text_model.encoder.layers.22.self_attn.k_proj.weight"
		openClipVitG14Key    = "cond_stage_model.1.transformer.text_model.encoder.layers.31.self_attn.k_proj.weight" // OpenCLIP ViT-G/14
		openClipVitG14Key2   = "text_model.encoder.layers.31.self_attn.k_proj.weight"
		t5xxlKey             = "cond_stage_model.1.transformer.encoder.block.23.layer.0.SelfAttention.k.weight" // Google T5-xxl
		t5xxlKey2            = "cond_stage_model.2.transformer.encoder.block.23.layer.0.SelfAttention.k.weight"
		t5xxlKey3            = "encoder.block.23.layer.0.SelfAttention.k.weight"
	)

	tis, _ := gf.TensorInfos.Index([]string{
		sdKey,
		sdKey2,
		sdXlKey,
		sdXlKey2,
		sdXlRefinerKey,
		sdXlRefinerKey2,
		sd3Key,
		sd3Key2,
		sdInPaintFeatureKey,
		sdInPaintFeatureKey2,

		fluxKey,
		fluxKey2,
		fluxFillFeatureKey,
		fluxFillFeatureKey2,

		openAiClipVitL14Key,
		openAiClipVitL14Key2,
		openClipVitH14Key,
		openClipVitH14Key2,
		openClipVitG14Key,
		openClipVitG14Key2,
		t5xxlKey,
		t5xxlKey2,
		t5xxlKey3,
	})

	ga.Type = "model"
	ga.Architecture = "diffusion"

	if ti, ok := tis[sdKey]; ok {
		ga.DiffusionArchitecture = "Stable Diffusion 1.x"
		if ti.Dimensions[0] == 1024 {
			ga.DiffusionArchitecture = "Stable Diffusion 2.x"
		}
		if ti, ok := tis[sdInPaintFeatureKey]; ok && ti.Dimensions[2] == 9 {
			ga.DiffusionArchitecture += " InPaint"
		}
	} else if _, ok := tis[sdKey2]; ok {
		ga.DiffusionArchitecture = "Stable Diffusion 1.x"
		if ti.Dimensions[0] == 1024 {
			ga.DiffusionArchitecture = "Stable Diffusion 2.x"
		}
		if ti, ok := tis[sdInPaintFeatureKey2]; ok && ti.Dimensions[2] == 9 {
			ga.DiffusionArchitecture += " InPaint"
		}
	} else if _, ok := tis[sdXlKey]; ok {
		ga.DiffusionArchitecture = "Stable Diffusion XL"
		if _, ok = tis[sdXlRefinerKey]; ok {
			ga.DiffusionArchitecture = "Stable Diffusion XL Refiner"
		}
		if ti, ok := tis[sdInPaintFeatureKey]; ok && ti.Dimensions[2] == 9 {
			ga.DiffusionArchitecture += " InPaint"
		}
	} else if _, ok := tis[sdXlKey2]; ok {
		ga.DiffusionArchitecture = "Stable Diffusion XL"
		if _, ok = tis[sdXlRefinerKey2]; ok {
			ga.DiffusionArchitecture = "Stable Diffusion XL Refiner"
		}
		if ti, ok := tis[sdInPaintFeatureKey2]; ok && ti.Dimensions[2] == 9 {
			ga.DiffusionArchitecture += " InPaint"
		}
	} else if _, ok := tis[sd3Key]; ok {
		ga.DiffusionArchitecture = "Stable Diffusion 3.x"
		ga.DiffusionTransformer = true
	} else if _, ok := tis[sd3Key2]; ok {
		ga.DiffusionArchitecture = "Stable Diffusion 3.x"
		ga.DiffusionTransformer = true
	}
	if _, ok := tis[fluxKey]; ok {
		ga.DiffusionArchitecture = "FLUX.1"
		ga.DiffusionTransformer = true
		if ti, ok := tis[fluxFillFeatureKey]; ok && ti.Dimensions[0] == 384 {
			ga.DiffusionArchitecture += " Fill"
		}
	} else if _, ok := tis[fluxKey2]; ok {
		ga.DiffusionArchitecture = "FLUX.1"
		ga.DiffusionTransformer = true
		if ti, ok := tis[fluxFillFeatureKey2]; ok && ti.Dimensions[0] == 384 {
			ga.DiffusionArchitecture += " Fill"
		}
	}

	if ti, ok := tis[openAiClipVitL14Key]; ok {
		cond := GGUFArchitectureDiffusionConditioner{
			Architecture: "OpenAI CLIP ViT-L/14",
			FileType:     ti.GetFileType(),
		}
		if ti, ok = tis[openClipVitH14Key]; ok {
			cond = GGUFArchitectureDiffusionConditioner{
				Architecture: "OpenCLIP ViT-H/14",
				FileType:     ti.GetFileType(),
			}
		}
		ga.DiffusionConditioners = append(ga.DiffusionConditioners, cond)
	} else if ti, ok := tis[openAiClipVitL14Key2]; ok {
		cond := GGUFArchitectureDiffusionConditioner{
			Architecture: "OpenAI CLIP ViT-L/14",
			FileType:     ti.GetFileType(),
		}
		if ti, ok = tis[openClipVitH14Key2]; ok {
			cond = GGUFArchitectureDiffusionConditioner{
				Architecture: "OpenCLIP ViT-H/14",
				FileType:     ti.GetFileType(),
			}
		}
		ga.DiffusionConditioners = append(ga.DiffusionConditioners, cond)
	}
	if ti, ok := tis[openClipVitG14Key]; ok {
		ga.DiffusionConditioners = append(ga.DiffusionConditioners, GGUFArchitectureDiffusionConditioner{
			Architecture: "OpenCLIP ViT-G/14",
			FileType:     ti.GetFileType(),
		})
	} else if ti, ok = tis[openClipVitG14Key2]; ok {
		ga.DiffusionConditioners = append(ga.DiffusionConditioners, GGUFArchitectureDiffusionConditioner{
			Architecture: "OpenCLIP ViT-G/14",
			FileType:     ti.GetFileType(),
		})
	}
	if ti, ok := tis[t5xxlKey]; ok {
		ga.DiffusionConditioners = append(ga.DiffusionConditioners, GGUFArchitectureDiffusionConditioner{
			Architecture: "Google T5-xxl",
			FileType:     ti.GetFileType(),
		})
	} else if ti, ok = tis[t5xxlKey2]; ok {
		ga.DiffusionConditioners = append(ga.DiffusionConditioners, GGUFArchitectureDiffusionConditioner{
			Architecture: "Google T5-xxl",
			FileType:     ti.GetFileType(),
		})
	} else if ti, ok = tis[t5xxlKey3]; ok {
		ga.DiffusionConditioners = append(ga.DiffusionConditioners, GGUFArchitectureDiffusionConditioner{
			Architecture: "Google T5-xxl",
			FileType:     ti.GetFileType(),
		})
	}

	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`^first_stage_model\..*`),
		regexp.MustCompile(`^decoder\.conv_in\..*`),
	} {
		if tis := gf.TensorInfos.Search(re); len(tis) != 0 {
			ga.DiffusionAutoencoder = &GGUFArchitectureDiffusionAutoencoder{
				Architecture: ga.DiffusionArchitecture + " VAE",
				FileType:     GGUFTensorInfos(tis).GetFileType(),
			}
			break
		}
	}

	return ga
}

// _GGUFClipProjectorTypeKeys are the metadata keys that can name a projector's type,
// in the order llama.cpp reads them: the default key first, then the key of the
// modality it is loading, see
// https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/tools/mtmd/clip.cpp#L1255-L1272.
var _GGUFClipProjectorTypeKeys = []string{
	"clip.projector_type",
	"clip.vision.projector_type",
	"clip.audio.projector_type",
	"clip.gen.audio.projector_type",
}

// _GGUFClipVisionRequiredKeys are the sizes llama.cpp reads as required keys of a
// vision projector, see
// https://github.com/ggml-org/llama.cpp/blob/c841aeeb8/tools/mtmd/clip.cpp#L1308-L1309.
var _GGUFClipVisionRequiredKeys = []string{
	"clip.vision.image_size",
	"clip.vision.patch_size",
}

// _GGUFClipWarmupImage is how llama.cpp sizes the one square image it reserves a
// dynamic-resolution projector's graph for. TokensPerSide is the square root of
// the warm-up token count llama.cpp carries for the projector type, and
// MergeDefault is the patch merge it assumes when the file declares none.
type _GGUFClipWarmupImage struct {
	TokensPerSide uint64
	MergeDefault  uint64
}

// _GGUFClipWarmupImages carries llama.cpp's own warm-up token counts, which no
// metadata key holds, see
// https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/tools/mtmd/clip.cpp#L1513-L1523
// for pixtral and
// https://github.com/ggml-org/llama.cpp/blob/b3c3b96a139d4ef1bdec926ac17aa040981cfc5d/tools/mtmd/clip.cpp#L1623-L1641
// for the Qwen vision family. Every other projector type reserves for the image
// size it declares, which is already the default.
var _GGUFClipWarmupImages = map[string]_GGUFClipWarmupImage{
	"qwen2vl_merger":   {TokensPerSide: 46, MergeDefault: 2},
	"qwen2.5vl_merger": {TokensPerSide: 46, MergeDefault: 2},
	"qwen3vl_merger":   {TokensPerSide: 46, MergeDefault: 2},
	"pixtral":          {TokensPerSide: 16, MergeDefault: 1},
}

func (gf *GGUFFile) clipArchitecture() (ga GGUFArchitecture) {
	const (
		hasLLaVAProjectorKey = "clip.has_llava_projector"
		hasMiniCPMVProjector = "clip.has_minicpmv_projector"
		miniCPMVVersionKey   = "clip.minicpmv_version"
		miniCPMVQueryNumKey  = "clip.minicpmv_query_num"
		hasGLMProjectorKey   = "clip.has_glm_projector"
		hasQwen2VLMergerKey  = "clip.has_qwen2vl_merger"

		hasVisionEncoderKey                   = "clip.has_vision_encoder"
		visionEmbeddingLengthKey              = "clip.vision.embedding_length"
		visionBlockCountKey                   = "clip.vision.block_count"
		visionFeedForwardLengthKey            = "clip.vision.feed_forward_length"
		visionAttentionHeadCountKey           = "clip.vision.attention.head_count"
		visionAttentionLayerNormRMSEpsilonKey = "clip.vision.attention.layer_norm_epsilon"
		visionProjectionDimKey                = "clip.vision.projection_dim"
		visionProjectorScaleFactorKey         = "clip.vision.projector.scale_factor"
		visionImageSizeKey                    = "clip.vision.image_size"
		visionPatchSizeKey                    = "clip.vision.patch_size"
		visionMMPatchMergeTypeKey             = "clip.vision.mm_patch_merge_type"
		visioSpatialMergeSizeKey              = "clip.vision.spatial_merge_size"
		visionWindowAttentionPatternKey       = "clip.vision.n_wa_pattern"

		hasAudioEncoderKey                   = "clip.has_audio_encoder"
		audioEmbeddingLengthKey              = "clip.audio.embedding_length"
		audioBlockCountKey                   = "clip.audio.block_count"
		audioFeedForwardLengthKey            = "clip.audio.feed_forward_length"
		audioAttentionHeadCountKey           = "clip.audio.attention.head_count"
		audioAttentionLayerNormRMSEpsilonKey = "clip.audio.attention.layer_norm_epsilon"
		audioProjectionDimKey                = "clip.audio.projection_dim"
		audioProjectorStackFactorKey         = "clip.audio.projector.stack_factor"
		audioNumMelBinsKey                   = "clip.audio.num_mel_bins"
	)

	ga.Type = "projector"
	ga.Architecture = "clip"

	m, _ := gf.Header.MetadataKV.Index(append([]string{
		hasLLaVAProjectorKey,
		hasMiniCPMVProjector,
		miniCPMVVersionKey,
		miniCPMVQueryNumKey,
		hasGLMProjectorKey,
		hasQwen2VLMergerKey,
		// Vision
		hasVisionEncoderKey,
		visionEmbeddingLengthKey,
		visionBlockCountKey,
		visionFeedForwardLengthKey,
		visionAttentionHeadCountKey,
		visionAttentionLayerNormRMSEpsilonKey,
		visionProjectionDimKey,
		visionProjectorScaleFactorKey,
		visionImageSizeKey,
		visionPatchSizeKey,
		visionMMPatchMergeTypeKey,
		visioSpatialMergeSizeKey,
		visionWindowAttentionPatternKey,
		// Audio
		hasAudioEncoderKey,
		audioEmbeddingLengthKey,
		audioBlockCountKey,
		audioFeedForwardLengthKey,
		audioAttentionHeadCountKey,
		audioAttentionLayerNormRMSEpsilonKey,
		audioProjectionDimKey,
		audioProjectorStackFactorKey,
		audioNumMelBinsKey,
	}, _GGUFClipProjectorTypeKeys...))

	// A projector that names no type is refused while the file is parsed, so no
	// type is invented here.
	for _, k := range _GGUFClipProjectorTypeKeys {
		if v, ok := m[k]; ok {
			ga.ClipProjectorType = v.ValueString()
			break
		}
	}
	if v, ok := m[hasLLaVAProjectorKey]; ok {
		ga.ClipHasLLaVAProjector = v.ValueBool()
	}
	if v, ok := m[hasMiniCPMVProjector]; ok {
		ga.ClipHasMiniCPMVProjector = v.ValueBool()
	}
	if v, ok := m[miniCPMVVersionKey]; ok {
		ga.ClipMiniCPMVVersion = ValueNumeric[int32](v)
	}
	if v, ok := m[miniCPMVQueryNumKey]; ok {
		ga.ClipMiniCPMVQueryNum = ValueNumeric[int32](v)
	}
	if v, ok := m[hasGLMProjectorKey]; ok {
		ga.ClipHasGLMProjector = v.ValueBool()
	}
	if v, ok := m[hasQwen2VLMergerKey]; ok {
		ga.ClipHasQwen2VLMerger = v.ValueBool()
	}
	// Vision
	if v, ok := m[hasVisionEncoderKey]; ok {
		ga.ClipHasVisionEncoder = v.ValueBool()
	}
	if v, ok := m[visionEmbeddingLengthKey]; ok {
		ga.ClipVisionEmbeddingLength = ValueNumeric[uint64](v)
	}
	if v, ok := m[visionBlockCountKey]; ok {
		ga.ClipVisionBlockCount = ValueNumeric[uint64](v)
	}
	if v, ok := m[visionFeedForwardLengthKey]; ok {
		if v.ValueType == GGUFMetadataValueTypeArray {
			ga.ClipVisionFeedForwardLength = ValuesNumeric[uint64](v.ValueArray())
		} else {
			vx := ValueNumeric[uint64](v)
			ga.ClipVisionFeedForwardLength = make([]uint64, ga.ClipVisionBlockCount)
			for i := range ga.ClipVisionFeedForwardLength {
				ga.ClipVisionFeedForwardLength[i] = vx
			}
		}
	}
	if v, ok := m[visionAttentionHeadCountKey]; ok {
		ga.ClipVisionAttentionHeadCount = ValueNumeric[uint64](v)
	}
	if v, ok := m[visionAttentionLayerNormRMSEpsilonKey]; ok {
		ga.ClipVisionAttentionLayerNormRMSEpsilon = ValueNumeric[float32](v)
	}
	if v, ok := m[visionImageSizeKey]; ok {
		ga.ClipVisionImageSize = ValueNumeric[uint32](v)
	}
	if v, ok := m[visionProjectionDimKey]; ok {
		ga.ClipVisionProjectionDim = ValueNumeric[uint32](v)
	}
	ga.ClipVisionProjectorScaleFactor = 1
	if ga.ClipProjectorType == "gemma3" {
		ga.ClipVisionProjectorScaleFactor = 4
	}
	if v, ok := m[visionProjectorScaleFactorKey]; ok {
		ga.ClipVisionProjectorScaleFactor = ValueNumeric[uint32](v)
	}
	// llama.cpp requires the key for a vision projector, and validateMetadata
	// refuses a file without it, so the default stands in only for a file built
	// in memory.
	ga.ClipVisionPatchSize = 1
	if v, ok := m[visionPatchSizeKey]; ok {
		ga.ClipVisionPatchSize = ValueNumeric[uint32](v)
	}
	ga.ClipVisionMMPatchMergeType = "flat"
	if v, ok := m[visionMMPatchMergeTypeKey]; ok {
		ga.ClipVisionMMPatchMergeType = v.ValueString()
	}
	if v, ok := m[visioSpatialMergeSizeKey]; ok {
		ga.ClipVisionSpatialMergeSize = ValueNumeric[uint32](v)
	}
	if v, ok := m[visionWindowAttentionPatternKey]; ok {
		ga.ClipVisionWindowAttentionPattern = ValueNumeric[uint32](v)
	}
	// Audio
	if v, ok := m[hasAudioEncoderKey]; ok {
		ga.ClipHasAudioEncoder = v.ValueBool()
	}
	if v, ok := m[audioEmbeddingLengthKey]; ok {
		ga.ClipAudioEmbeddingLength = ValueNumeric[uint64](v)
	}
	if v, ok := m[audioBlockCountKey]; ok {
		ga.ClipAudioBlockCount = ValueNumeric[uint64](v)
	}
	if v, ok := m[audioFeedForwardLengthKey]; ok {
		if v.ValueType == GGUFMetadataValueTypeArray {
			ga.ClipAudioFeedForwardLength = ValuesNumeric[uint64](v.ValueArray())
		} else {
			vx := ValueNumeric[uint64](v)
			ga.ClipAudioFeedForwardLength = make([]uint64, ga.ClipAudioBlockCount)
			for i := range ga.ClipAudioFeedForwardLength {
				ga.ClipAudioFeedForwardLength[i] = vx
			}
		}
	}
	if v, ok := m[audioAttentionHeadCountKey]; ok {
		ga.ClipAudioAttentionHeadCount = ValueNumeric[uint64](v)
	}
	if v, ok := m[audioAttentionLayerNormRMSEpsilonKey]; ok {
		ga.ClipAudioAttentionLayerNormRMSEpsilon = ValueNumeric[float32](v)
	}
	if v, ok := m[audioProjectionDimKey]; ok {
		ga.ClipAudioProjectionDim = ValueNumeric[uint32](v)
	}
	ga.ClipAudioProjectorStackFactor = 1
	if v, ok := m[audioProjectorStackFactorKey]; ok {
		ga.ClipAudioProjectorStackFactor = ValueNumeric[uint32](v)
	}
	if v, ok := m[audioNumMelBinsKey]; ok {
		ga.ClipAudioNumMelBins = ValueNumeric[uint32](v)
	}

	ga.AttentionHeadCountKV = ga.AttentionHeadCount

	return ga
}

func (gf *GGUFFile) adapterArchitecture(arch string) (ga GGUFArchitecture) {
	var (
		typeKey = "adapter.type"

		loraAlphaKey = "adapter.lora.alpha"

		controlVectorLayerCountKey  = "adapter.control_vector.layer_count"
		controlVectorLayerCountKey2 = "control_vector.layer_count"
	)

	ga.Type = "adapter"
	ga.Architecture = arch

	m, _ := gf.Header.MetadataKV.Index([]string{
		typeKey,
		loraAlphaKey,
		controlVectorLayerCountKey,
		controlVectorLayerCountKey2,
	})

	if v, ok := m[typeKey]; ok {
		ga.AdapterType = v.ValueString()
	}
	if v, ok := m[loraAlphaKey]; ok {
		ga.AdapterLoRAAlpha = ValueNumeric[float32](v)
	}
	if v, ok := m[controlVectorLayerCountKey]; ok {
		ga.AdapterControlVectorLayerCount = ValueNumeric[uint32](v)
	} else if v, ok := m[controlVectorLayerCountKey2]; ok {
		ga.AdapterControlVectorLayerCount = ValueNumeric[uint32](v)
	}

	return ga
}

func (gf *GGUFFile) imatrixArchitecture(_ string) (ga GGUFArchitecture) {
	ga.Type = "imatrix"
	ga.Architecture = "imatrix"

	return ga
}

// swaPatternDefaults carries the per-architecture sliding window layout of llama.cpp:
// the period between full attention layers, whether the full layer leads each group
// (dense first, full at layer%period == 0) or closes it (full at layer%period == period-1),
// and the default window size for the architectures hardcoding one.
//
// A GGUF rarely declares "<architecture>.attention.sliding_window_pattern"
// (convert_hf_to_gguf.py does not write it), so these mirror
// https://github.com/ggml-org/llama.cpp/tree/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models.
// A period applies when the sliding window resolves above zero; a zero period windows every layer.
// DeepSeek-V4 windows every layer too, but its MLA + indexer caches diverge too far to model here.
var swaPatternDefaults = map[string]struct {
	period     uint32
	denseFirst bool
	window     uint64
}{
	"afmoe":           {period: 4},
	"cohere2":         {period: 4},
	"cohere2moe":      {period: 4, denseFirst: true},
	"dflash":          {period: 0},
	"exaone-moe":      {period: 4, window: 128},
	"gemma-embedding": {period: 6},
	"gemma2":          {period: 2, window: 4096},
	"gemma3":          {period: 6},
	"gemma3n":         {period: 5},
	"gpt-oss":         {period: 2},
	"laguna":          {period: 4, denseFirst: true},
	"llama4":          {period: 4, window: 8192},
	"mellum":          {period: 4},
	"modern-bert":     {period: 3, denseFirst: true},
	"muse-glimmer":    {period: 4},
	"olmo2":           {period: 4},
	"plamo3":          {period: 8},
	"smallthinker":    {period: 4, denseFirst: true, window: 4096},
}

func (gf *GGUFFile) transformerArchitecture(arch string) (ga GGUFArchitecture) {
	var (
		contextLengthKey     = arch + ".context_length"
		embeddingLengthKey   = arch + ".embedding_length"
		blockCountKey        = arch + ".block_count"
		feedForwardLengthKey = arch + ".feed_forward_length"

		expertFeedForwardLengthKey       = arch + ".expert_feed_forward_length"
		expertSharedFeedForwardLengthKey = arch + ".expert_shared_feed_forward_length"
		expertCountKey                   = arch + ".expert_count"
		expertUsedCountKey               = arch + ".expert_used_count"
		expertSharedCountKey             = arch + ".expert_shared_count"

		attentionHeadCountKey            = arch + ".attention.head_count"
		attentionHeadCountKVKey          = arch + ".attention.head_count_kv"
		attentionSlidingWindowKey        = arch + ".attention.sliding_window"
		attentionSlidingWindowPatternKey = arch + ".attention.sliding_window_pattern"
		attentionKeyLengthSWAKey         = arch + ".attention.key_length_swa"
		attentionValueLengthSWAKey       = arch + ".attention.value_length_swa"
		attentionSharedKVLayersKey       = arch + ".attention.shared_kv_layers"
		nextNPredictLayersKey            = arch + ".nextn_predict_layers"
		attentionMaxALiBIBiasKey         = arch + ".attention.max_alibi_bias"
		attentionMaxALiBIBiasKey2        = arch + ".attention.alibi_bias_max"
		attentionClampKQVKey             = arch + ".attention.clamp_kqv"
		attentionClampKQVKey2            = arch + ".attention.clip_kqv"
		attentionLayerNormEpsilonKey     = arch + ".attention.layer_norm_epsilon"
		attentionLayerNormRMSEpsilonKey  = arch + ".attention.layer_norm_rms_epsilon"
		attentionQueryLORARankKey        = arch + ".attention.q_lora_rank"
		attentionKeyValueLORARankKey     = arch + ".attention.kv_lora_rank"
		attentionKeyLengthKey            = arch + ".attention.key_length"
		attentionKeyLengthMLAKey         = arch + ".attention.key_length_mla"
		attentionValueLengthKey          = arch + ".attention.value_length"
		attentionValueLengthMLAKey       = arch + ".attention.value_length_mla"
		attentionCausalKey               = arch + ".attention.causal"

		ropeDimensionCountKey         = arch + ".rope.dimension_count"
		ropeFrequencyBaseKey          = arch + ".rope.freq_base"
		ropeFrequencyScaleKey         = arch + ".rope.freq_scale"
		ropeScaleLinearKey            = arch + ".rope.scale_linear"
		ropeScalingTypeKey            = arch + ".rope.scaling.type"
		ropeScalingFactorKey          = arch + ".rope.scaling.factor"
		ropeScalingOriginalContextKey = arch + ".rope.scaling.original_context_length" // uint32 maybe
		ropeScalingFinetunedKey       = arch + ".rope.scaling.finetuned"

		poolingTypeKey = arch + ".pooling_type"

		decisionTypeKey = arch + ".decision.type"

		fullAttentionIntervalKey = arch + ".full_attention_interval"

		ssmConvolutionKernelKey = arch + ".ssm.conv_kernel"
		ssmInnerSizeKey         = arch + ".ssm.inner_size"
		ssmStateSizeKey         = arch + ".ssm.state_size"
		ssmTimeStepRankKey      = arch + ".ssm.time_step_rank"
		ssmGroupCountKey        = arch + ".ssm.group_count"

		shortConvLCacheKey = arch + ".shortconv.l_cache"
		kdaHeadDimKey      = arch + ".kda.head_dim"

		rwkvHeadSizeKey                = arch + ".wkv.head_size"
		rwkvRescaleEveryNLayersKey     = arch + ".rescale_every_n_layers"
		rwkvTimeMixExtraDimensionKey   = arch + ".time_mix_extra_dim"
		rwkvTimeDecayExtraDimensionKey = arch + ".time_decay_extra_dim"
		rwkvTokenShiftCountKey         = arch + ".token_shift_count"

		vocabularyLengthKey    = arch + ".vocab_size"
		tokenizerGGMLTokensKey = "tokenizer.ggml.tokens"
	)

	ga.Type = "model"
	ga.Architecture = arch

	m, _ := gf.Header.MetadataKV.Index([]string{
		contextLengthKey,
		embeddingLengthKey,
		blockCountKey,
		feedForwardLengthKey,
		expertFeedForwardLengthKey,
		expertSharedFeedForwardLengthKey,
		expertCountKey,
		expertUsedCountKey,
		expertSharedCountKey,
		attentionHeadCountKey,
		attentionHeadCountKVKey,
		attentionSlidingWindowKey,
		attentionSlidingWindowPatternKey,
		attentionKeyLengthSWAKey,
		attentionValueLengthSWAKey,
		attentionSharedKVLayersKey,
		nextNPredictLayersKey,
		attentionMaxALiBIBiasKey,
		attentionMaxALiBIBiasKey2,
		attentionClampKQVKey,
		attentionClampKQVKey2,
		attentionLayerNormEpsilonKey,
		attentionLayerNormRMSEpsilonKey,
		attentionQueryLORARankKey,
		attentionKeyValueLORARankKey,
		attentionKeyLengthKey,
		attentionKeyLengthMLAKey,
		attentionValueLengthKey,
		attentionValueLengthMLAKey,
		attentionCausalKey,
		ropeDimensionCountKey,
		ropeFrequencyBaseKey,
		ropeFrequencyScaleKey,
		ropeScaleLinearKey,
		ropeScalingTypeKey,
		ropeScalingFactorKey,
		ropeScalingOriginalContextKey,
		ropeScalingFinetunedKey,
		poolingTypeKey,
		decisionTypeKey,
		fullAttentionIntervalKey,
		ssmConvolutionKernelKey,
		ssmInnerSizeKey,
		ssmStateSizeKey,
		ssmTimeStepRankKey,
		ssmGroupCountKey,
		shortConvLCacheKey,
		kdaHeadDimKey,
		rwkvHeadSizeKey,
		rwkvRescaleEveryNLayersKey,
		rwkvTimeMixExtraDimensionKey,
		rwkvTimeDecayExtraDimensionKey,
		rwkvTokenShiftCountKey,
		vocabularyLengthKey,
		tokenizerGGMLTokensKey,
	})

	if v, ok := m[contextLengthKey]; ok {
		ga.MaximumContextLength = ValueNumeric[uint64](v)
	}
	if v, ok := m[embeddingLengthKey]; ok {
		ga.EmbeddingLength = ValueNumeric[uint64](v)
	}
	if v, ok := m[blockCountKey]; ok {
		ga.BlockCount = ValueNumeric[uint64](v)
	}
	if v, ok := m[feedForwardLengthKey]; ok {
		if v.ValueType == GGUFMetadataValueTypeArray {
			ga.FeedForwardLength = ValuesNumeric[uint64](v.ValueArray())
		} else {
			vx := ValueNumeric[uint64](v)
			ga.FeedForwardLength = make([]uint64, ga.BlockCount)
			for i := range ga.FeedForwardLength {
				ga.FeedForwardLength[i] = vx
			}
		}
	}

	if v, ok := m[expertCountKey]; ok {
		ga.ExpertCount = ValueNumeric[uint32](v)
	}
	if v, ok := m[expertUsedCountKey]; ok {
		ga.ExpertUsedCount = ValueNumeric[uint32](v)
	}
	if v, ok := m[expertSharedCountKey]; ok {
		ga.ExpertSharedCount = ValueNumeric[uint32](v)
	}
	if v, ok := m[expertFeedForwardLengthKey]; ok {
		ga.ExpertFeedForwardLength = ValueNumeric[uint64](v)
	}
	if v, ok := m[expertSharedFeedForwardLengthKey]; ok {
		ga.ExpertSharedFeedForwardLength = ValueNumeric[uint64](v)
	}

	if v, ok := m[attentionHeadCountKey]; ok {
		if v.ValueType == GGUFMetadataValueTypeArray {
			ga.AttentionHeadCount = ValuesNumeric[uint64](v.ValueArray())[0]
		} else {
			ga.AttentionHeadCount = ValueNumeric[uint64](v)
		}
	}
	if v, ok := m[attentionHeadCountKVKey]; ok {
		if v.ValueType == GGUFMetadataValueTypeArray {
			// Keep the per-layer values: a hybrid architecture declares its recurrent layers
			// with zero KV heads, and a variable-GQA architecture varies the width per layer.
			ga.AttentionHeadCountKVs = ValuesNumeric[uint64](v.ValueArray())
			if len(ga.AttentionHeadCountKVs) > 0 {
				ga.AttentionHeadCountKV = slices.Max(ga.AttentionHeadCountKVs)
			}
		} else {
			ga.AttentionHeadCountKV = ValueNumeric[uint64](v)
		}
	} else {
		ga.AttentionHeadCountKV = ga.AttentionHeadCount
	}
	ga.AttentionSlidingWindowPattern = 1
	if v, ok := m[attentionSlidingWindowKey]; ok {
		if v.ValueType == GGUFMetadataValueTypeArray {
			ga.AttentionSlidingWindow = ValuesNumeric[uint64](v.ValueArray())[0]
		} else {
			ga.AttentionSlidingWindow = ValueNumeric[uint64](v)
		}
	}
	if v, ok := m[nextNPredictLayersKey]; ok {
		ga.NextNPredictLayers = ValueNumeric[uint32](v)
	}
	// Sliding window layout, from the per-architecture period table swaPatternDefaults.
	swaPeriod, swaDenseFirst := uint32(1), false
	if d, ok := swaPatternDefaults[arch]; ok {
		swaDenseFirst = d.denseFirst
		if ga.AttentionSlidingWindow == 0 {
			ga.AttentionSlidingWindow = d.window
		}
		if ga.AttentionSlidingWindow > 0 {
			swaPeriod = d.period
		}
	}
	switch arch {
	case "phi3":
		// See https://github.com/ggml-org/llama.cpp/pull/13676
		ga.AttentionSlidingWindow = 0
	case "exaone4":
		// Only the 64-layer EXAONE 4.0 interleaves windows; the count excludes the
		// NextN blocks, as llama.cpp's n_layer() does,
		// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/exaone4.cpp#L4-L14.
		if ga.BlockCount > uint64(ga.NextNPredictLayers) && ga.BlockCount-uint64(ga.NextNPredictLayers) == 64 {
			if ga.AttentionSlidingWindow == 0 {
				ga.AttentionSlidingWindow = 4096
			}
			swaPeriod = 4
		}
	case "gemma3n":
		// The first 20 layers hold a KV cache, the layers behind reuse those,
		// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/gemma3n.cpp#L9.
		ga.AttentionKVFromStartLayerCount = 20
	}
	if v, ok := m[attentionSlidingWindowPatternKey]; ok {
		// The declared pattern is either the period as a scalar, or the per-layer use as a bool
		// array (Gemma 4), see llama_model_loader::get_key_or_arr.
		if v.ValueType == GGUFMetadataValueTypeArray {
			av := v.ValueArray()
			ga.AttentionSlidingWindowLayers = make([]bool, min(av.Len, maxSaneBlockCount))
			for i := range ga.AttentionSlidingWindowLayers {
				b, _ := av.Array[i].(bool)
				ga.AttentionSlidingWindowLayers[i] = b
			}
		} else {
			swaPeriod = ValueNumeric[uint32](v)
		}
	}
	if len(ga.AttentionSlidingWindowLayers) == 0 && swaPeriod != 1 && ga.AttentionSlidingWindow > 0 &&
		ga.BlockCount > 0 && ga.BlockCount <= maxSaneBlockCount {
		ga.AttentionSlidingWindowLayers = make([]bool, ga.BlockCount)
		for i := range ga.AttentionSlidingWindowLayers {
			if swaDenseFirst {
				ga.AttentionSlidingWindowLayers[i] = swaPeriod == 0 || uint32(i)%swaPeriod != 0
			} else {
				ga.AttentionSlidingWindowLayers[i] = swaPeriod == 0 || uint32(i)%swaPeriod < swaPeriod-1
			}
		}
	}
	// LFM2 windows its attention layers when it declares a window; its recurrent layers hold no KV,
	// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/lfm2.cpp#L24-L29.
	if (arch == "lfm2" || arch == "lfm2moe") && ga.AttentionSlidingWindow > 0 {
		ga.AttentionSlidingWindowLayers = make([]bool, min(uint64(len(ga.AttentionHeadCountKVs)), maxSaneBlockCount))
		for i := range ga.AttentionSlidingWindowLayers {
			ga.AttentionSlidingWindowLayers[i] = ga.AttentionHeadCountKVs[i] > 0
		}
	}
	ga.AttentionSlidingWindowPattern = swaPeriod
	if v, ok := m[attentionKeyLengthSWAKey]; ok {
		ga.AttentionKeyLengthSWA = ValueNumeric[uint32](v)
	}
	if v, ok := m[attentionValueLengthSWAKey]; ok {
		ga.AttentionValueLengthSWA = ValueNumeric[uint32](v)
	}
	if v, ok := m[attentionSharedKVLayersKey]; ok && ga.BlockCount > 0 {
		// Gemma 4 hangs its trailing layers off the leading caches,
		// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/gemma4.cpp#L7-L10.
		if shared := ValueNumeric[uint64](v); shared > 0 && shared <= ga.BlockCount {
			ga.AttentionKVFromStartLayerCount = uint32(ga.BlockCount - shared)
		}
	}
	if v, ok := m[attentionMaxALiBIBiasKey]; ok {
		ga.AttentionMaxALiBIBias = ValueNumeric[float32](v)
	} else if v, ok := m[attentionMaxALiBIBiasKey2]; ok {
		ga.AttentionMaxALiBIBias = ValueNumeric[float32](v)
	}
	if v, ok := m[attentionClampKQVKey]; ok {
		ga.AttentionClampKQV = ValueNumeric[float32](v)
	} else if v, ok := m[attentionClampKQVKey2]; ok {
		ga.AttentionClampKQV = ValueNumeric[float32](v)
	}
	if v, ok := m[attentionLayerNormEpsilonKey]; ok {
		ga.AttentionLayerNormEpsilon = ValueNumeric[float32](v)
	}
	if v, ok := m[attentionLayerNormRMSEpsilonKey]; ok {
		ga.AttentionLayerNormRMSEpsilon = ValueNumeric[float32](v)
	}
	if v, ok := m[attentionQueryLORARankKey]; ok {
		ga.AttentionQueryLORARank = ValueNumeric[uint32](v)
	}
	if v, ok := m[attentionKeyValueLORARankKey]; ok {
		ga.AttentionKeyValueLORARank = ValueNumeric[uint32](v)
	}
	if v, ok := m[attentionKeyLengthKey]; ok {
		ga.AttentionKeyLength = ValueNumeric[uint32](v)
	} else if ga.AttentionHeadCount != 0 {
		ga.AttentionKeyLength = uint32(ga.EmbeddingLength / ga.AttentionHeadCount)
	}
	if v, ok := m[attentionKeyLengthMLAKey]; ok {
		ga.AttentionKeyLengthMLA = ValueNumeric[uint32](v)
	}
	if v, ok := m[attentionValueLengthKey]; ok {
		ga.AttentionValueLength = ValueNumeric[uint32](v)
	} else if ga.AttentionHeadCount != 0 {
		ga.AttentionValueLength = uint32(ga.EmbeddingLength / ga.AttentionHeadCount)
	}
	if v, ok := m[attentionValueLengthMLAKey]; ok {
		ga.AttentionValueLengthMLA = ValueNumeric[uint32](v)
	}
	if v, ok := m[attentionCausalKey]; ok {
		ga.AttentionCausal = v.ValueBool()
	} else {
		// An encoder-only architecture attends bidirectionally even when the
		// file does not declare attention.causal,
		// see llama_context::encode in llama.cpp.
		ga.AttentionCausal = ga.Architecture != "t5encoder"
	}
	// See https://github.com/ggml-org/llama.cpp/blob/6491d6e4f1caf0ad2221865b4249ae6938a6308c/src/llama-arch.cpp#L1913-L1924.
	ga.AttentionRecurrent = slices.Contains([]string{ // TODO(thxCode): calculate this from the metadata.
		"mamba",
		"mamba2",
		"rwkv6",
		"rwkv6qwen2",
		"rwkv7",
		"arwkv7",
	}, ga.Architecture)
	// A hybrid architecture interleaves recurrent layers with full (self-)attention layers.
	//
	// Prefer the metadata: only the hybrid QWEN3NEXT/QWEN35/QWEN35MOE read
	// `<architecture>.full_attention_interval`, and its value says which layer is which,
	// so neither the architecture nor its layer split has to be hardcoded,
	// see https://github.com/ggml-org/llama.cpp/blob/272700b360944e40816a7ea13da8cd723119000a/src/models/qwen35moe.cpp#L22-L30.
	if v, ok := m[fullAttentionIntervalKey]; ok {
		ga.FullAttentionInterval = ValueNumeric[uint32](v)
	}
	// Fall back to the hardcoded list for the hybrid architectures declaring no interval,
	// see https://github.com/ggml-org/llama.cpp/blob/a57d1bcb3c0165ac87b1f0dbb429839b0da69689/src/llama-arch.cpp#L2029-L2038.
	// Jamba and Granite-Hybrid declare their interleaving per layer in
	// `<architecture>.attention.head_count_kv`, and so does LFM2, which reaches this list
	// through that array rather than by name, see interleavesKVHeads and
	// https://github.com/ggml-org/llama.cpp/blob/272700b360944e40816a7ea13da8cd723119000a/src/models/lfm2.cpp#L9-L11.
	// Falcon-H1 declares none, it runs attention and recurrence in parallel on every layer.
	ga.AttentionHybrid = ga.FullAttentionInterval > 1 || slices.Contains([]string{
		"jamba",
		"falcon-h1",
		"granitehybrid",
	}, ga.Architecture)
	ga.AttentionRecurrent = ga.AttentionHybrid || ga.AttentionRecurrent

	if v, ok := m[ropeDimensionCountKey]; ok {
		ga.RoPEDimensionCount = ValueNumeric[uint64](v)
	}
	ga.RoPEFrequencyBase = 10000.0
	if v, ok := m[ropeFrequencyBaseKey]; ok {
		ga.RoPEFrequencyBase = ValueNumeric[float32](v)
	}
	ga.RoPEFrequencyScale = 1.0
	if v, ok := m[ropeFrequencyScaleKey]; ok {
		ga.RoPEFrequencyScale = ValueNumeric[float32](v)
	}
	if v, ok := m[ropeScalingTypeKey]; ok {
		ga.RoPEScalingType = v.ValueString()
	}
	if v, ok := m[ropeScaleLinearKey]; ok {
		ga.RoPEScalingType = "linear"
		ga.RoPEScalingFactor = ValueNumeric[float32](v)
		if ga.RoPEScalingFactor != 0 {
			ga.RoPEFrequencyScale = 1.0 / ga.RoPEScalingFactor
		}
	}
	if v, ok := m[ropeScalingFactorKey]; ok {
		ga.RoPEScalingFactor = ValueNumeric[float32](v)
		if ga.RoPEScalingFactor != 0 {
			ga.RoPEFrequencyScale = 1.0 / ga.RoPEScalingFactor
		}
	}
	if v, ok := m[ropeScalingOriginalContextKey]; ok {
		ga.RoPEScalingOriginalContextLength = ValueNumeric[uint64](v)
	}
	if v, ok := m[ropeScalingFinetunedKey]; ok {
		ga.RoPEScalingFinetuned = v.ValueBool()
	}

	if v, ok := m[poolingTypeKey]; ok {
		ga.PoolingType = v.ValueUint32()
		if ga.AttentionCausal && ga.PoolingType > 2 {
			ga.AttentionCausal = false
		}
	}

	// The presence of the "<architecture>.decision.type" metadata marks
	// a decision model, see common_get_decision_type in llama.cpp's
	// common/common.cpp. The value type is guarded because the key comes
	// straight from the untrusted file, a mistyped key reads as no decision head.
	if v, ok := m[decisionTypeKey]; ok && v.ValueType == GGUFMetadataValueTypeString {
		ga.DecisionType = v.ValueString()
	}

	if v, ok := m[ssmConvolutionKernelKey]; ok {
		ga.SSMConvolutionKernel = ValueNumeric[uint32](v)
	}
	if v, ok := m[ssmInnerSizeKey]; ok {
		ga.SSMInnerSize = ValueNumeric[uint32](v)
	}
	if v, ok := m[ssmStateSizeKey]; ok {
		ga.SSMStateSize = ValueNumeric[uint32](v)
	}
	if v, ok := m[ssmTimeStepRankKey]; ok {
		ga.SSMTimeStepRank = ValueNumeric[uint32](v)
	}
	if v, ok := m[ssmGroupCountKey]; ok {
		ga.SSMGroupCount = ValueNumeric[uint32](v)
	}

	if v, ok := m[shortConvLCacheKey]; ok {
		ga.ShortConvLCache = ValueNumeric[uint32](v)
	}
	if v, ok := m[kdaHeadDimKey]; ok {
		ga.KDAHeadDim = ValueNumeric[uint32](v)
	}

	if v, ok := m[rwkvHeadSizeKey]; ok {
		ga.RWKVHeadSize = ValueNumeric[uint32](v)
	}
	if v, ok := m[rwkvRescaleEveryNLayersKey]; ok {
		ga.RWKVRescaleEveryNLayers = ValueNumeric[uint32](v)
	}
	if v, ok := m[rwkvTimeMixExtraDimensionKey]; ok {
		ga.RWKVTimeMixExtraDimension = ValueNumeric[uint32](v)
	}
	if v, ok := m[rwkvTimeDecayExtraDimensionKey]; ok {
		ga.RWKVTimeDecayExtraDimension = ValueNumeric[uint32](v)
	}
	if v, ok := m[rwkvTokenShiftCountKey]; ok {
		ga.RWKVTokenShiftCount = ValueNumeric[uint32](v)
	} else if ga.AttentionRecurrent {
		ga.RWKVTokenShiftCount = 2
	}

	// A per-layer head_count_kv with zero entries declares the interleaving as well:
	// the zero layers hold the recurrent state, the other layers a full (self-)attention KV cache,
	// see https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/lfm2.cpp#L9-L11
	// and https://github.com/ggml-org/llama.cpp/blob/16d222fc5f2c0fdc3d0180e0b772516ec6e2eddd/src/models/kimi-linear.cpp#L15-L19.
	// The zero layers of a stateless architecture (Deci-style dummy blocks) are attention-free
	// instead, so becoming hybrid requires a recurrent state.
	if hasRecurrentState := ga.SSMConvolutionKernel > 0 || ga.SSMStateSize > 0 ||
		ga.ShortConvLCache > 0 || ga.KDAHeadDim > 0 || ga.RWKVHeadSize > 0; hasRecurrentState &&
		!ga.AttentionRecurrent && slices.Contains(ga.AttentionHeadCountKVs, 0) {
		ga.AttentionHybrid = true
		ga.AttentionRecurrent = true
	}

	if v, ok := m[vocabularyLengthKey]; ok {
		ga.VocabularyLength = ValueNumeric[uint64](v)
	} else if v, ok := m[tokenizerGGMLTokensKey]; ok {
		ga.VocabularyLength = v.ValueArray().Len
	}

	return ga
}

// emitsLogitsAtEveryPosition reports whether this architecture produces logits
// for every position in the batch rather than for the last one only.
//
// A normal decoder is reserved with n_outputs = 1, so its logits buffer is one
// row and it lives on the host. A speculator or draft head scores every position
// it proposes, so the output projection is a full vocabulary by batch tensor on
// the device that holds the head.
func (ga GGUFArchitecture) emitsLogitsAtEveryPosition() bool {
	return slices.Contains([]string{"eagle3"}, ga.Architecture)
}

// ssmProjectionRows are the rows a mamba2 block holds per token: the fused zx
// projection, twice the inner width plus the group state, and one
// convolution-width row, the inner width plus the group state, both f32, see
// build_mamba2_layer in llama.cpp. Every factor widens to uint64 before the
// product, as the fields are uint32.
func (ga GGUFArchitecture) ssmProjectionRows(nTokens uint64) (zx, conv uint64) {
	groupState := 2 * uint64(ga.SSMGroupCount) * uint64(ga.SSMStateSize)
	zx = GGMLTypeF32.RowSizeOf([]uint64{2*uint64(ga.SSMInnerSize) + groupState, nTokens})
	conv = GGMLTypeF32.RowSizeOf([]uint64{uint64(ga.SSMInnerSize) + groupState, nTokens})
	return zx, conv
}

// ssmConvolutionStateSize is the element count of one sequence's mamba
// convolution state: the kernel width less one, times the inner and group state
// widths, and zero for a zero kernel, see llama_hparams::n_embd_r at
// https://github.com/ggml-org/llama.cpp/blob/c841aeeb8/src/llama-hparams.cpp#L203.
// Every factor widens to uint64 before the product, as the fields are uint32.
func (ga GGUFArchitecture) ssmConvolutionStateSize() uint64 {
	if ga.SSMConvolutionKernel == 0 {
		return 0
	}
	return (uint64(ga.SSMConvolutionKernel) - 1) * (uint64(ga.SSMInnerSize) + 2*uint64(ga.SSMGroupCount)*uint64(ga.SSMStateSize))
}

// buildsMemorylessGraph reports whether llama.cpp serves this architecture
// without a memory module, so its graph attends over the ubatch only.
//
// llama_model::create_memory returns no memory for these architectures, see
// https://github.com/ggml-org/llama.cpp/blob/c841aeeb8/src/llama-model.cpp#L2206-L2228.
// t5encoder builds its whole graph from the same no-cache attention input.
func (ga GGUFArchitecture) buildsMemorylessGraph() bool {
	return slices.Contains([]string{
		"bert", "jina-bert-v2", "jina-bert-v3", "nomic-bert", "nomic-bert-moe",
		"neo-bert", "eurobert", "modern-bert", "gemma-embedding",
		"dream", "llada", "llada-moe", "rnd1",
		"t5encoder", "wavtokenizer-dec",
	}, ga.Architecture)
}
