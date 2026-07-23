package gguf_parser

import (
	"regexp"
	"slices"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
)

// GGUFMetadata represents the model metadata of a GGUF file.
type GGUFMetadata struct {
	/* Basic */

	// Type describes what type this GGUF file is,
	// default is "model".
	Type string `json:"type"`
	// Architecture describes what architecture this GGUF file implements.
	//
	// All lowercase ASCII.
	Architecture string `json:"architecture"`
	// QuantizationVersion describes the version of the quantization format.
	//
	// Not required if the model is not quantized (i.e. no tensors are quantized).
	// If any tensors are quantized, this must be present.
	// This is separate to the quantization scheme of the tensors itself,
	// the quantization version may change without changing the scheme's name,
	// e.g. the quantization scheme is Q5_K, and the QuantizationVersion is 4.
	QuantizationVersion uint32 `json:"quantizationVersion,omitempty"`
	// Alignment describes the alignment of the GGUF file.
	//
	// This can vary to allow for different alignment schemes, but it must be a multiple of 8.
	// Some writers may not write the alignment.
	//
	// Default is 32.
	Alignment uint32 `json:"alignment"`
	// Name to the model.
	//
	// This should be a human-readable name that can be used to identify the GGUF file.
	// It should be unique within the community that the model is defined in.
	Name string `json:"name,omitempty"`
	// Author to the model.
	Author string `json:"author,omitempty"`
	// URL to the model's homepage.
	//
	// This can be a GitHub repo, a paper, etc.
	URL string `json:"url,omitempty"`
	// Description to the model.
	Description string `json:"description,omitempty"`
	// License to the model.
	//
	// This is expressed as a SPDX license expression, e.g. "MIT OR Apache-2.0".
	License string `json:"license,omitempty"`
	// FileType describes the type of the majority of the tensors in the GGUF file.
	FileType GGUFFileType `json:"fileType"`
	// FileTypeDescriptor describes the type of the GGUF file according to the FileType and trait layer.
	//
	// This supplies the FileType with more detail.
	FileTypeDescriptor string `json:"fileTypeDetail"`

	/* Appendix */

	// LittleEndian is true if the GGUF file is little-endian,
	// and false for big-endian.
	LittleEndian bool `json:"littleEndian"`
	// FileSize is the size of the GGUF file in bytes.
	FileSize GGUFBytesScalar `json:"fileSize"`
	// Size is the model size.
	Size GGUFBytesScalar `json:"size"`
	// Parameters is the parameters of the GGUF file.
	Parameters GGUFParametersScalar `json:"parameters"`
	// BitsPerWeight is the bits per weight of the GGUF file.
	BitsPerWeight GGUFBitsPerWeightScalar `json:"bitsPerWeight"`
}

// GGUFFileType is a type of GGUF file,
// see https://github.com/ggml-org/llama.cpp/blob/fd1234cb468935ea087d6929b2487926c3afff4b/ggml/include/ggml.h#L419-L445,
// and https://github.com/huggingface/huggingface.js/blob/d67a464473ca07fee9811a129e5fac8cc7487098/packages/tasks/src/gguf.ts#L4-L52.
type GGUFFileType uint32

// GGUFFileType constants.
//
// GGUFFileTypeMostlyQ4_2, GGUFFileTypeMostlyQ4_3 are deprecated.
// GGUFFileTypeMostlyQ4_0_4_4, GGUFFileTypeMostlyQ4_0_4_8, GGUFFileTypeMostlyQ4_0_8_8 are deprecated.
//
// GGUFFileTypeMostlyQ4_1_SOME_F16 is a special case where the majority of the tensors are Q4_1,
// but 'token_embd.weight' and 'output.weight' tensors are F16.
const (
	GGUFFileTypeMostlyF32           GGUFFileType = iota // MOSTLY_F32
	GGUFFileTypeMostlyF16                               // MOSTLY_F16
	GGUFFileTypeMostlyQ4_0                              // MOSTLY_Q4_0
	GGUFFileTypeMostlyQ4_1                              // MOSTLY_Q4_1
	GGUFFileTypeMostlyQ4_1_SOME_F16                     // MOSTLY_Q4_1_SOME_F16
	GGUFFileTypeMostlyQ4_2                              // MOSTLY_Q4_2
	GGUFFileTypeMostlyQ4_3                              // MOSTLY_Q4_3
	GGUFFileTypeMostlyQ8_0                              // MOSTLY_Q8_0
	GGUFFileTypeMostlyQ5_0                              // MOSTLY_Q5_0
	GGUFFileTypeMostlyQ5_1                              // MOSTLY_Q5_1
	GGUFFileTypeMostlyQ2_K                              // MOSTLY_Q2_K
	GGUFFileTypeMostlyQ3_K_S                            // MOSTLY_Q3_K_S
	GGUFFileTypeMostlyQ3_K_M                            // MOSTLY_Q3_K_M
	GGUFFileTypeMostlyQ3_K_L                            // MOSTLY_Q3_K_L
	GGUFFileTypeMostlyQ4_K_S                            // MOSTLY_Q4_K_S
	GGUFFileTypeMostlyQ4_K_M                            // MOSTLY_Q4_K_M
	GGUFFileTypeMostlyQ5_K_S                            // MOSTLY_Q5_K_S
	GGUFFileTypeMostlyQ5_K_M                            // MOSTLY_Q5_K_M
	GGUFFileTypeMostlyQ6_K                              // MOSTLY_Q6_K
	GGUFFileTypeMostlyIQ2_XXS                           // MOSTLY_IQ2_XXS
	GGUFFileTypeMostlyIQ2_XS                            // MOSTLY_IQ2_XS
	GGUFFileTypeMostlyQ2_K_S                            // MOSTLY_Q2_K_S
	GGUFFileTypeMostlyIQ3_XS                            // MOSTLY_IQ3_XS
	GGUFFileTypeMostlyIQ3_XXS                           // MOSTLY_IQ3_XXS
	GGUFFileTypeMostlyIQ1_S                             // MOSTLY_IQ1_S
	GGUFFileTypeMostlyIQ4_NL                            // MOSTLY_IQ4_NL
	GGUFFileTypeMostlyIQ3_S                             // MOSTLY_IQ3_S
	GGUFFileTypeMostlyIQ3_M                             // MOSTLY_IQ3_M
	GGUFFileTypeMostlyIQ2_S                             // MOSTLY_IQ2_S
	GGUFFileTypeMostlyIQ2_M                             // MOSTLY_IQ2_M
	GGUFFileTypeMostlyIQ4_XS                            // MOSTLY_IQ4_XS
	GGUFFileTypeMostlyIQ1_M                             // MOSTLY_IQ1_M
	GGUFFileTypeMostlyBF16                              // MOSTLY_BF16
	GGUFFileTypeMostlyQ4_0_4_4                          // MOSTLY_Q4_0_4_4
	GGUFFileTypeMostlyQ4_0_4_8                          // MOSTLY_Q4_0_4_8
	GGUFFileTypeMostlyQ4_0_8_8                          // MOSTLY_Q4_0_8_8
	GGUFFileTypeMostlyTQ1_0                             // MOSTLY_TQ1_0
	GGUFFileTypeMostlyTQ2_0                             // MOSTLY_TQ2_0
	GGUFFileTypeMostlyMXFP4                             // MOSTLY_MXFP4
	_GGUFFileTypeCount                                  // Unknown
)

// _GGUFPotentialDiffusionArchitectures holds a list representing the potential diffusion architectures.
//
// Since we will unify all diffusion architectures to "diffusion" during processing,
// we can use this list to match the value in explicit `general.architecture`.
var _GGUFPotentialDiffusionArchitectures = []string{
	"flux",
	"sd",
	"sd2.5",
	"sd3",
	"stable-diffusion",
}

// _GGUFPotentialDiffusionArchitectureTensorsRegexes holds a list of regexes to match the potential diffusion architecture tensors.
//
// This is used to detect if the GGUF file is a diffusion model,
// when the `general.architecture` is not set to a known diffusion architecture.
var _GGUFPotentialDiffusionArchitectureTensorsRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^model\.diffusion_model\..*`),
	regexp.MustCompile(`^double_blocks\..*`),
	regexp.MustCompile(`^joint_blocks\..*`),
	regexp.MustCompile(`^decoder\..*`),
	regexp.MustCompile(`^encoder\..*`),
	regexp.MustCompile(`^text_model\..*`),
}

// Metadata returns the metadata of the GGUF file.
func (gf *GGUFFile) Metadata() (gm GGUFMetadata) {
	const (
		typeKey         = "general.type"
		architectureKey = "general.architecture"
		quantizationKey = "general.quantization_version"
		alignmentKey    = "general.alignment"
		nameKey         = "general.name"
		authorKey       = "general.author"
		urlKey          = "general.url"
		descriptionKey  = "general.description"
		licenseKey      = "general.license"

		controlVectorModelHintKey = "controlvector.model_hint"
	)

	m, _ := gf.Header.MetadataKV.Index([]string{
		typeKey,
		architectureKey,
		quantizationKey,
		alignmentKey,
		nameKey,
		authorKey,
		urlKey,
		descriptionKey,
		licenseKey,
		controlVectorModelHintKey,
	})

	if v, ok := m[typeKey]; ok {
		gm.Type = v.ValueString()
	} else if _, ok = m[controlVectorModelHintKey]; ok {
		gm.Type = "adapter"
	} else {
		gm.Type = "model"
	}
	if v, ok := m[controlVectorModelHintKey]; ok {
		gm.Architecture = v.ValueString()
	} else if v, ok = m[architectureKey]; ok && !slices.Contains(_GGUFPotentialDiffusionArchitectures, v.ValueString()) {
		gm.Architecture = v.ValueString()
		if gm.Architecture == "clip" {
			gm.Type = "projector"
		}
	} else if gm.Type == "imatrix" {
		gm.Architecture = "imatrix" // Default to imatrix.
	} else {
		gm.Architecture = "llama" // Default to llama.
		for _, re := range _GGUFPotentialDiffusionArchitectureTensorsRegexes {
			if gf.TensorInfos.Match(re) {
				gm.Architecture = "diffusion"
				break
			}
		}
	}
	if v, ok := m[quantizationKey]; ok {
		gm.QuantizationVersion = ValueNumeric[uint32](v)
	}
	if v, ok := m[alignmentKey]; ok {
		gm.Alignment = ValueNumeric[uint32](v)
	} else {
		gm.Alignment = 32
	}
	if v, ok := m[nameKey]; ok {
		gm.Name = v.ValueString()
	}
	if v, ok := m[authorKey]; ok {
		gm.Author = v.ValueString()
	}
	if v, ok := m[urlKey]; ok {
		gm.URL = v.ValueString()
	}
	if v, ok := m[descriptionKey]; ok {
		gm.Description = v.ValueString()
	}
	if v, ok := m[licenseKey]; ok {
		gm.License = v.ValueString()
	}
	gm.FileType, gm.FileTypeDescriptor = gf.extractFileType(gm.Architecture)

	gm.LittleEndian = gf.Header.Version < GGUFVersionV3 || gf.Header.Magic == GGUFMagicGGUFLe
	gm.FileSize = gf.Size
	gm.Size = gf.ModelSize
	gm.Parameters = gf.ModelParameters
	gm.BitsPerWeight = gf.ModelBitsPerWeight

	return gm
}

// GGMLType returns the GGMLType of the GGUFFileType,
// which is inspired by
// https://github.com/ggerganov/ggml/blob/a10a8b880c059b3b29356eb9a9f8df72f03cdb6a/src/ggml.c#L2730-L2763.
func (t GGUFFileType) GGMLType() GGMLType {
	switch t {
	case GGUFFileTypeMostlyF32:
		return GGMLTypeF32
	case GGUFFileTypeMostlyF16:
		return GGMLTypeF16
	case GGUFFileTypeMostlyQ4_0:
		return GGMLTypeQ4_0
	case GGUFFileTypeMostlyQ4_1:
		return GGMLTypeQ4_1
	case GGUFFileTypeMostlyQ4_1_SOME_F16:
		return GGMLTypeQ4_1
	case GGUFFileTypeMostlyQ4_2:
		return GGMLTypeQ4_2
	case GGUFFileTypeMostlyQ4_3:
		return GGMLTypeQ4_3
	case GGUFFileTypeMostlyQ8_0:
		return GGMLTypeQ8_0
	case GGUFFileTypeMostlyQ5_0:
		return GGMLTypeQ5_0
	case GGUFFileTypeMostlyQ5_1:
		return GGMLTypeQ5_1
	case GGUFFileTypeMostlyQ2_K:
		return GGMLTypeQ2_K
	case GGUFFileTypeMostlyQ3_K_S:
		return GGMLTypeQ3_K
	case GGUFFileTypeMostlyQ3_K_M:
		return GGMLTypeQ4_K
	case GGUFFileTypeMostlyQ3_K_L:
		return GGMLTypeQ5_K
	case GGUFFileTypeMostlyQ4_K_S:
		return GGMLTypeQ6_K
	case GGUFFileTypeMostlyQ4_K_M:
		return GGMLTypeQ4_K
	case GGUFFileTypeMostlyQ5_K_S:
		return GGMLTypeQ5_K
	case GGUFFileTypeMostlyQ5_K_M:
		return GGMLTypeQ5_K
	case GGUFFileTypeMostlyQ6_K:
		return GGMLTypeQ6_K
	case GGUFFileTypeMostlyIQ2_XXS:
		return GGMLTypeIQ2_XXS
	case GGUFFileTypeMostlyIQ2_XS:
		return GGMLTypeIQ2_XS
	case GGUFFileTypeMostlyQ2_K_S:
		return GGMLTypeQ2_K
	case GGUFFileTypeMostlyIQ3_XS:
		return GGMLTypeIQ3_S
	case GGUFFileTypeMostlyIQ3_XXS:
		return GGMLTypeIQ3_XXS
	case GGUFFileTypeMostlyIQ1_S:
		return GGMLTypeIQ1_S
	case GGUFFileTypeMostlyIQ4_NL:
		return GGMLTypeIQ4_NL
	case GGUFFileTypeMostlyIQ3_S:
		return GGMLTypeIQ3_S
	case GGUFFileTypeMostlyIQ3_M:
		return GGMLTypeIQ3_S
	case GGUFFileTypeMostlyIQ2_S:
		return GGMLTypeIQ2_XS
	case GGUFFileTypeMostlyIQ2_M:
		return GGMLTypeIQ2_S
	case GGUFFileTypeMostlyIQ4_XS:
		return GGMLTypeIQ4_XS
	case GGUFFileTypeMostlyIQ1_M:
		return GGMLTypeIQ1_M
	case GGUFFileTypeMostlyBF16:
		return GGMLTypeBF16
	case GGUFFileTypeMostlyQ4_0_4_4:
		return GGMLTypeQ4_0_4_4
	case GGUFFileTypeMostlyQ4_0_4_8:
		return GGMLTypeQ4_0_4_8
	case GGUFFileTypeMostlyQ4_0_8_8:
		return GGMLTypeQ4_0_8_8
	case GGUFFileTypeMostlyTQ1_0:
		return GGMLTypeTQ1_0
	case GGUFFileTypeMostlyTQ2_0:
		return GGMLTypeTQ2_0
	case GGUFFileTypeMostlyMXFP4:
		return GGMLTypeMXFP4
	default:
	}
	return _GGMLTypeCount
}

// extractFileType extracts the GGUF file type from the metadata,
// it tries to return the descriptor of the file type.
func (gf *GGUFFile) extractFileType(arch string) (fileType GGUFFileType, fileTypeDescriptor string) {
	fileType, fileTypeDescriptor = _GGUFFileTypeCount, "Unknown"

	const fileTypeKey = "general.file_type"
	m, _ := gf.Header.MetadataKV.Index([]string{
		fileTypeKey,
	})
	if v, ok := m[fileTypeKey]; ok {
		fileType = GGUFFileType(ValueNumeric[uint32](v))
	}

	if fileType == _GGUFFileTypeCount {
		// Guess.
		if len(gf.TensorInfos) != 0 {
			cm := make(map[GGMLType]int)
			for i := range gf.TensorInfos {
				switch {
				case arch != "diffusion" &&
					!strings.HasPrefix(gf.TensorInfos[i].Name, "token_embd") &&
					!strings.HasPrefix(gf.TensorInfos[i].Name, "blk.") &&
					!strings.Contains(gf.TensorInfos[i].Name, "_norm") &&
					!strings.HasSuffix(gf.TensorInfos[i].Name, ".weight"):
					continue
				case arch == "diffusion" &&
					!strings.HasSuffix(gf.TensorInfos[i].Name, ".weight"):
					continue
				}
				cm[gf.TensorInfos[i].Type]++
			}
			fileType = GetFileType(cm)
		}
	}
	if fileType == _GGUFFileTypeCount {
		return fileType, fileTypeDescriptor
	}

	fileTypeDescriptor = strings.TrimPrefix(fileType.String(), "MOSTLY_")

	const tokenEmbedWeightTensorName = "token_embd.weight"

	switch fileType {
	case GGUFFileTypeMostlyQ4_0:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 || v.Type == GGMLTypeQ5_0 || v.Type == GGMLTypeQ5_1 {
				fileTypeDescriptor = "Q4_0_L"
			}
		}
	case GGUFFileTypeMostlyQ4_1:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 || v.Type == GGMLTypeQ5_0 || v.Type == GGMLTypeQ5_1 {
				fileTypeDescriptor = "Q4_1_L"
			}
		}
	case GGUFFileTypeMostlyQ5_0:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 {
				fileTypeDescriptor = "Q5_0_L"
			}
		}
	case GGUFFileTypeMostlyQ5_1:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 {
				fileTypeDescriptor = "Q5_1_L"
			}
		}
	case GGUFFileTypeMostlyQ2_K:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 || v.Type == GGMLTypeQ4_K {
				fileTypeDescriptor = "Q2_K_L"
			}
		}
	case GGUFFileTypeMostlyQ3_K_M:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 {
				fileTypeDescriptor = "Q3_K_L"
			}
		}
	case GGUFFileTypeMostlyQ4_K_M:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 {
				fileTypeDescriptor = "Q4_K_L"
			}
		}
	case GGUFFileTypeMostlyQ5_K_M:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 {
				fileTypeDescriptor = "Q5_K_L"
			}
		}
	case GGUFFileTypeMostlyQ6_K:
		tis, _ := gf.TensorInfos.Index([]string{tokenEmbedWeightTensorName})
		if v, ok := tis[tokenEmbedWeightTensorName]; ok {
			if v.Type == GGMLTypeQ8_0 {
				fileTypeDescriptor = "Q6_K_L"
			}
		}
	}

	return fileType, fileTypeDescriptor
}

// GetFileType returns the GGUFFileType represented the mostly GGMLType of the given tensors counter.
//
// The input `cm` is a map of GGMLType to the count of tensors of that type.
func GetFileType(cm map[GGMLType]int) GGUFFileType {
	if len(cm) == 0 {
		return _GGUFFileTypeCount
	}

	// Sort.
	ts := maps.Keys(cm)
	sort.Slice(ts, func(i, j int) bool {
		return cm[ts[i]] > cm[ts[j]]
	})

	// Guess.
	if ts[0] == GGMLTypeF32 {
		if len(ts) == 1 {
			return GGUFFileTypeMostlyF32
		}
		ts[0] = ts[1]
	}
	switch ts[0] {
	case GGMLTypeF16:
		return GGUFFileTypeMostlyF16
	case GGMLTypeQ4_0:
		return GGUFFileTypeMostlyQ4_0
	case GGMLTypeQ4_1:
		return GGUFFileTypeMostlyQ4_1
	case GGMLTypeQ4_2:
		return GGUFFileTypeMostlyQ4_2
	case GGMLTypeQ4_3:
		return GGUFFileTypeMostlyQ4_3
	case GGMLTypeQ5_0:
		return GGUFFileTypeMostlyQ5_0
	case GGMLTypeQ5_1:
		return GGUFFileTypeMostlyQ5_1
	case GGMLTypeQ8_0:
		return GGUFFileTypeMostlyQ8_0
	case GGMLTypeQ2_K:
		if ts[len(ts)-1] == GGMLTypeQ5_K {
			return GGUFFileTypeMostlyQ2_K_S
		}
		return GGUFFileTypeMostlyQ2_K
	case GGMLTypeQ3_K:
		if cm[GGMLTypeQ8_0] > 0 ||
			(cm[GGMLTypeQ5_K] > 1 && cm[GGMLTypeQ4_K] == 0) {
			return GGUFFileTypeMostlyQ3_K_L
		}
		if cm[GGMLTypeQ4_K] > 1 {
			return GGUFFileTypeMostlyQ3_K_M
		}
		return GGUFFileTypeMostlyQ3_K_S
	case GGMLTypeQ4_K:
		if cm[GGMLTypeQ6_K] > 1 {
			return GGUFFileTypeMostlyQ4_K_M
		}
		if cm[GGMLTypeQ3_K] > 1 {
			return GGUFFileTypeMostlyQ3_K_M
		}
		return GGUFFileTypeMostlyQ4_K_S
	case GGMLTypeQ5_K:
		if cm[GGMLTypeQ6_K] > 1 {
			return GGUFFileTypeMostlyQ5_K_M
		}
		return GGUFFileTypeMostlyQ5_K_S
	case GGMLTypeQ6_K:
		return GGUFFileTypeMostlyQ6_K
	case GGMLTypeIQ2_XXS:
		return GGUFFileTypeMostlyIQ2_XXS
	case GGMLTypeIQ2_XS:
		if cm[GGMLTypeIQ4_XS] > 1 {
			return GGUFFileTypeMostlyIQ2_S
		}
		return GGUFFileTypeMostlyIQ2_XS
	case GGMLTypeIQ2_S:
		return GGUFFileTypeMostlyIQ2_M
	case GGMLTypeIQ3_XXS:
		return GGUFFileTypeMostlyIQ3_XXS
	case GGMLTypeIQ3_S:
		if cm[GGMLTypeIQ3_XXS] > 1 {
			return GGUFFileTypeMostlyIQ3_XS
		}
		return GGUFFileTypeMostlyIQ3_S
	case GGMLTypeIQ1_S:
		return GGUFFileTypeMostlyIQ1_S
	case GGMLTypeIQ4_NL:
		return GGUFFileTypeMostlyIQ4_NL
	case GGMLTypeIQ4_XS:
		return GGUFFileTypeMostlyIQ4_XS
	case GGMLTypeIQ1_M:
		return GGUFFileTypeMostlyIQ1_M
	case GGMLTypeBF16:
		return GGUFFileTypeMostlyBF16
	case GGMLTypeQ4_0_4_4:
		return GGUFFileTypeMostlyQ4_0_4_4
	case GGMLTypeQ4_0_4_8:
		return GGUFFileTypeMostlyQ4_0_4_8
	case GGMLTypeQ4_0_8_8:
		return GGUFFileTypeMostlyQ4_0_8_8
	case GGMLTypeTQ1_0:
		return GGUFFileTypeMostlyTQ1_0
	case GGMLTypeTQ2_0:
		return GGUFFileTypeMostlyTQ2_0
	case GGMLTypeMXFP4:
		return GGUFFileTypeMostlyMXFP4
	default:
	}
	return _GGUFFileTypeCount
}
