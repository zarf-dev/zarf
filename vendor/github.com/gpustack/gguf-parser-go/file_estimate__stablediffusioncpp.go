package gguf_parser

import (
	"math"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/gpustack/gguf-parser-go/util/ptr"
	"github.com/gpustack/gguf-parser-go/util/stringx"
)

// Types for StableDiffusionCpp estimation.
type (
	// StableDiffusionCppRunEstimate represents the estimated result of loading the GGUF file in stable-diffusion.cpp.
	StableDiffusionCppRunEstimate struct {
		// Type describes what type this GGUF file is.
		Type string `json:"type"`
		// Architecture describes what architecture this GGUF file implements.
		//
		// All lowercase ASCII.
		Architecture string `json:"architecture"`
		// FlashAttention is the flag to indicate whether enable the flash attention,
		// true for enable.
		FlashAttention bool `json:"flashAttention"`
		// FullOffloaded is the flag to indicate whether the layers are fully offloaded,
		// false for partial offloaded or zero offloaded.
		FullOffloaded bool `json:"fullOffloaded"`
		// NoMMap is the flag to indicate whether support the mmap,
		// true for support.
		NoMMap bool `json:"noMMap"`
		// ImageOnly is the flag to indicate whether the model is used for generating image,
		// true for generating image only.
		ImageOnly bool `json:"imageOnly"`
		// Distributable is the flag to indicate whether the model is distributable,
		// true for distributable.
		Distributable bool `json:"distributable"`
		// Devices represents the usage for running the GGUF file,
		// the first device is the CPU, and the rest are GPUs.
		Devices []StableDiffusionCppRunDeviceUsage `json:"devices"`
		// Autoencoder is the estimated result of the autoencoder.
		Autoencoder *StableDiffusionCppRunEstimate `json:"autoencoder,omitempty"`
		// Conditioners is the estimated result of the conditioners.
		Conditioners []StableDiffusionCppRunEstimate `json:"conditioners,omitempty"`
		// Upscaler is the estimated result of the upscaler.
		Upscaler *StableDiffusionCppRunEstimate `json:"upscaler,omitempty"`
		// ControlNet is the estimated result of the control net.
		ControlNet *StableDiffusionCppRunEstimate `json:"controlNet,omitempty"`
	}

	// StableDiffusionCppRunDeviceUsage represents the usage for running the GGUF file in llama.cpp.
	StableDiffusionCppRunDeviceUsage struct {
		// Remote is the flag to indicate whether the device is remote,
		// true for remote.
		Remote bool `json:"remote"`
		// Position is the relative position of the device,
		// starts from 0.
		//
		// If Remote is true, Position is the position of the remote devices,
		// Otherwise, Position is the position of the device in the local devices.
		Position int `json:"position"`
		// Footprint is the memory footprint for bootstrapping.
		Footprint GGUFBytesScalar `json:"footprint"`
		// Parameter is the running parameters that the device processes.
		Parameter GGUFParametersScalar `json:"parameter"`
		// Weight is the memory usage of weights that the device loads.
		Weight GGUFBytesScalar `json:"weight"`
		// Computation is the memory usage of computation that the device processes.
		Computation GGUFBytesScalar `json:"computation"`
	}
)

// EstimateStableDiffusionCppRun estimates the usages of the GGUF file in stable-diffusion.cpp.
func (gf *GGUFFile) EstimateStableDiffusionCppRun(opts ...GGUFRunEstimateOption) (e StableDiffusionCppRunEstimate) {
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
	if o.SDCOffloadLayers == nil {
		o.SDCOffloadLayers = ptr.To[uint64](math.MaxUint64)
	}
	if o.SDCBatchCount == nil {
		o.SDCBatchCount = ptr.To[int32](1)
	}
	if o.SDCHeight == nil {
		o.SDCHeight = ptr.To[uint32](1024)
	}
	if o.SDCWidth == nil {
		o.SDCWidth = ptr.To[uint32](1024)
	}
	if o.SDCOffloadConditioner == nil {
		o.SDCOffloadConditioner = ptr.To(true)
	}
	if o.SDCOffloadAutoencoder == nil {
		o.SDCOffloadAutoencoder = ptr.To(true)
	}
	if o.SDCAutoencoderTiling == nil {
		o.SDCAutoencoderTiling = ptr.To(false)
	}
	if o.SDCFreeComputeMemoryImmediately == nil {
		o.SDCFreeComputeMemoryImmediately = ptr.To(false)
	}

	// Devices.
	initDevices := func(e *StableDiffusionCppRunEstimate) {
		for j := range e.Devices[1:] {
			e.Devices[j+1].Remote = j < len(o.RPCServers)
			if e.Devices[j+1].Remote {
				e.Devices[j+1].Position = j
			} else {
				e.Devices[j+1].Position = j - len(o.RPCServers)
			}
		}
	}
	e.Devices = make([]StableDiffusionCppRunDeviceUsage, len(o.TensorSplitFraction)+1)
	initDevices(&e)

	// Metadata.
	a := gf.Architecture()
	e.Type = a.Type
	e.Architecture = normalizeArchitecture(a.DiffusionArchitecture)

	// Flash attention.
	if o.FlashAttention && !strings.HasPrefix(a.DiffusionArchitecture, "Stable Diffusion 3") {
		// NB(thxCode): Stable Diffusion 3 doesn't support flash attention yet,
		// see https://github.com/leejet/stable-diffusion.cpp/pull/386.
		e.FlashAttention = true
	}

	// Distributable.
	e.Distributable = true

	// Offload.
	e.FullOffloaded = *o.SDCOffloadLayers > 0

	// NoMMap.
	e.NoMMap = true // TODO: Implement this.

	// ImageOnly.
	e.ImageOnly = true // TODO: Implement this.

	// Autoencoder.
	if a.DiffusionAutoencoder != nil {
		ae := &StableDiffusionCppRunEstimate{
			Type:           "model",
			Architecture:   e.Architecture + "_vae",
			FlashAttention: e.FlashAttention,
			Distributable:  e.Distributable,
			FullOffloaded:  e.FullOffloaded && *o.SDCOffloadAutoencoder,
			NoMMap:         e.NoMMap,
			Devices:        make([]StableDiffusionCppRunDeviceUsage, len(e.Devices)),
		}
		initDevices(ae)
		e.Autoencoder = ae
	}

	// Conditioners.
	if len(a.DiffusionConditioners) != 0 {
		e.Conditioners = make([]StableDiffusionCppRunEstimate, 0, len(a.DiffusionConditioners))
		for i := range a.DiffusionConditioners {
			cd := StableDiffusionCppRunEstimate{
				Type:           "model",
				Architecture:   normalizeArchitecture(a.DiffusionConditioners[i].Architecture),
				FlashAttention: e.FlashAttention,
				Distributable:  e.Distributable,
				FullOffloaded:  e.FullOffloaded && *o.SDCOffloadConditioner,
				NoMMap:         e.NoMMap,
				Devices:        make([]StableDiffusionCppRunDeviceUsage, len(e.Devices)),
			}
			initDevices(&cd)
			e.Conditioners = append(e.Conditioners, cd)
		}
	}

	// Footprint
	{
		// Bootstrap.
		e.Devices[0].Footprint = GGUFBytesScalar(10*1024*1024) /* model load */ + (gf.Size - gf.ModelSize) /* metadata */
	}

	var cdLs, aeLs, dmLs GGUFLayerTensorInfos
	{
		ls := gf.Layers()
		cdLs, aeLs, _ = ls.Cut([]string{
			"cond_stage_model.*",
		})
		aeLs, dmLs, _ = aeLs.Cut([]string{
			"first_stage_model.*",
		})
	}

	var cdDevIdx, aeDevIdx, dmDevIdx int
	{
		if *o.SDCOffloadConditioner && *o.SDCOffloadLayers > 0 {
			cdDevIdx = 1
		}
		if *o.SDCOffloadAutoencoder && *o.SDCOffloadLayers > 0 {
			aeDevIdx = 1
			if len(e.Devices) > 3 {
				aeDevIdx = 2
			}
		}
		if *o.SDCOffloadLayers > 0 {
			dmDevIdx = 1
			switch {
			case len(e.Devices) > 3:
				dmDevIdx = 3
			case len(e.Devices) > 2:
				dmDevIdx = 2
			}
		}
	}

	// Weight & Parameter.
	{
		// Conditioners.
		for i := range cdLs {
			e.Conditioners[i].Devices[cdDevIdx].Weight = GGUFBytesScalar(cdLs[i].Bytes())
			e.Conditioners[i].Devices[cdDevIdx].Parameter = GGUFParametersScalar(cdLs[i].Elements())
		}

		// Autoencoder.
		if len(aeLs) != 0 {
			e.Autoencoder.Devices[aeDevIdx].Weight = GGUFBytesScalar(aeLs.Bytes())
			e.Autoencoder.Devices[aeDevIdx].Parameter = GGUFParametersScalar(aeLs.Elements())
		}

		// Model.
		e.Devices[dmDevIdx].Weight = GGUFBytesScalar(dmLs.Bytes())
		e.Devices[dmDevIdx].Parameter = GGUFParametersScalar(dmLs.Elements())
	}

	// Computation.
	{
		// See https://github.com/leejet/stable-diffusion.cpp/blob/10c6501bd05a697e014f1bee3a84e5664290c489/ggml_extend.hpp#L1058C9-L1058C23.
		var maxNodes uint64 = 32768

		// Bootstrap, compute metadata.
		cm := GGMLTensorOverhead()*maxNodes + GGMLComputationGraphOverhead(maxNodes, false)
		e.Devices[0].Computation = GGUFBytesScalar(cm)

		// Work context,
		// see https://github.com/leejet/stable-diffusion.cpp/blob/4570715727f35e5a07a76796d823824c8f42206c/stable-diffusion.cpp#L1467-L1481,
		//     https://github.com/leejet/stable-diffusion.cpp/blob/4570715727f35e5a07a76796d823824c8f42206c/stable-diffusion.cpp#L1572-L1586,
		//     https://github.com/leejet/stable-diffusion.cpp/blob/4570715727f35e5a07a76796d823824c8f42206c/stable-diffusion.cpp#L1675-L1679.
		//
		{
			zChannels := uint64(4)
			if a.DiffusionTransformer {
				zChannels = 16
			}
			// See https://github.com/thxCode/stable-diffusion.cpp/blob/1ae97f8a8ca3615bdaf9c1fd32c13562e2471833/stable-diffusion.cpp#L2682-L2691.
			usage := uint64(128 * 1024 * 1024) /* 128MiB, LLaMA Box */
			usage += uint64(*o.SDCWidth) * uint64(*o.SDCHeight) * 3 /* output channels */ * 4 /* sizeof(float) */ * zChannels
			e.Devices[0].Computation += GGUFBytesScalar(usage * uint64(ptr.Deref(o.ParallelSize, 1)) /* max batch */)
		}

		// Encode usage,
		// see https://github.com/leejet/stable-diffusion.cpp/blob/4570715727f35e5a07a76796d823824c8f42206c/conditioner.hpp#L388-L391,
		//     https://github.com/leejet/stable-diffusion.cpp/blob/4570715727f35e5a07a76796d823824c8f42206c/conditioner.hpp#L758-L766,
		//     https://github.com/leejet/stable-diffusion.cpp/blob/4570715727f35e5a07a76796d823824c8f42206c/conditioner.hpp#L1083-L1085.
		{
			var tes [][]uint64
			switch {
			case strings.HasPrefix(a.DiffusionArchitecture, "FLUX"): // FLUX.1
				tes = [][]uint64{
					{768, 77},
					{4096, 256},
				}
			case strings.HasPrefix(a.DiffusionArchitecture, "Stable Diffusion 3"): // SD 3.x
				tes = [][]uint64{
					{768, 77},
					{1280, 77},
					{4096, 77},
				}
			case strings.HasPrefix(a.DiffusionArchitecture, "Stable Diffusion XL"): // SD XL/XL Refiner
				if strings.HasSuffix(a.DiffusionArchitecture, "Refiner") {
					tes = [][]uint64{
						{1280, 77},
					}
				} else {
					tes = [][]uint64{
						{768, 77},
						{1280, 77},
					}
				}
			default: // SD 1.x/2.x
				tes = [][]uint64{
					{768, 77},
				}
			}
			for i := range cdLs {
				usage := GGMLTypeF32.RowSizeOf(tes[i]) * 2 /* include conditioner */
				e.Conditioners[i].Devices[cdDevIdx].Computation += GGUFBytesScalar(usage)
			}

			// TODO VAE Encode
		}

		// Diffusing usage.
		if !*o.SDCFreeComputeMemoryImmediately {
			var usage uint64
			switch {
			case strings.HasPrefix(a.DiffusionArchitecture, "FLUX"): // FLUX.1
				usage = GuessFLUXDiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
			case strings.HasPrefix(a.DiffusionArchitecture, "Stable Diffusion 3"): // SD 3.x
				const (
					sd3MediumKey  = "model.diffusion_model.joint_blocks.23.x_block.attn.proj.weight" // SD 3 Medium
					sd35MediumKey = "model.diffusion_model.joint_blocks.23.x_block.attn.ln_k.weight" // SD 3.5 Medium
					sd35LargeKey  = "model.diffusion_model.joint_blocks.37.x_block.attn.ln_k.weight" // SD 3.5 Large
				)
				m, _ := dmLs.Index([]string{sd3MediumKey, sd35MediumKey, sd35LargeKey})
				switch {
				case m[sd35LargeKey].Name != "":
					usage = GuessSD35LargeDiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
				case m[sd35MediumKey].Name != "":
					usage = GuessSD35MediumDiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
				default:
					usage = GuessSD3MediumDiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
				}
			case strings.HasPrefix(a.DiffusionArchitecture, "Stable Diffusion XL"): // SD XL/XL Refiner
				const (
					sdXlKey        = "model.diffusion_model.output_blocks.5.1.transformer_blocks.1.attn1.to_v.weight" // SD XL
					sdXlRefinerKey = "model.diffusion_model.output_blocks.8.1.transformer_blocks.1.attn1.to_v.weight" // SD XL Refiner
				)
				m, _ := dmLs.Index([]string{sdXlKey, sdXlRefinerKey})
				if m[sdXlRefinerKey].Name != "" {
					usage = GuessSDXLRefinerDiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
				} else {
					usage = GuessSDXLDiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
				}
			case strings.HasPrefix(a.DiffusionArchitecture, "Stable Diffusion 2"): // SD 2.x
				usage = GuessSD2DiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
			default: // SD 1.x
				usage = GuessSD1DiffusionModelMemoryUsage(*o.SDCWidth, *o.SDCHeight, e.FlashAttention)
			}
			e.Devices[dmDevIdx].Computation += GGUFBytesScalar(usage)
		}

		// Decode usage.
		if len(aeLs) != 0 && !*o.SDCFreeComputeMemoryImmediately {
			// Bootstrap.
			e.Autoencoder.Devices[aeDevIdx].Footprint += GGUFBytesScalar(100 * 1024 * 1024) /*100 MiB.*/

			var convDim uint64
			{
				m, _ := aeLs.Index([]string{
					"first_stage_model.decoder.conv_in.weight",
					"decoder.conv_in.weight",
				})
				tis := maps.Values(m)
				if len(tis) != 0 && tis[0].NDimensions > 3 {
					convDim = max(tis[0].Dimensions[0], tis[0].Dimensions[3])
				}
			}

			var usage uint64
			if !*o.SDCAutoencoderTiling {
				usage = uint64(*o.SDCWidth) * uint64(*o.SDCHeight) * (3 /* output channels */ *4 /* sizeof(float) */ + 1) * convDim
			} else {
				usage = 512 * 512 * (3 /* output channels */ *4 /* sizeof(float) */ + 1) * convDim
			}
			e.Autoencoder.Devices[aeDevIdx].Computation += GGUFBytesScalar(usage)
		}
	}

	return e
}

// Types for StableDiffusionCpp estimated summary.
type (
	// StableDiffusionCppRunEstimateSummary represents the estimated summary of loading the GGUF file in stable-diffusion.cpp.
	StableDiffusionCppRunEstimateSummary struct {
		/* Basic */

		// Items
		Items []StableDiffusionCppRunEstimateSummaryItem `json:"items"`

		/* Appendix */

		// Type describes what type this GGUF file is.
		Type string `json:"type"`
		// Architecture describes what architecture this GGUF file implements.
		//
		// All lowercase ASCII.
		Architecture string `json:"architecture"`
		// FlashAttention is the flag to indicate whether enable the flash attention,
		// true for enable.
		FlashAttention bool `json:"flashAttention"`
		// NoMMap is the flag to indicate whether the file must be loaded without mmap,
		// true for total loaded.
		NoMMap bool `json:"noMMap"`
		// ImageOnly is the flag to indicate whether the model is used for generating image,
		// true for embedding only.
		ImageOnly bool `json:"imageOnly"`
		// Distributable is the flag to indicate whether the model is distributable,
		// true for distributable.
		Distributable bool `json:"distributable"`
	}

	// StableDiffusionCppRunEstimateSummaryItem represents the estimated summary item of loading the GGUF file in stable-diffusion.cpp.
	StableDiffusionCppRunEstimateSummaryItem struct {
		// FullOffloaded is the flag to indicate whether the layers are fully offloaded,
		// false for partial offloaded or zero offloaded.
		FullOffloaded bool `json:"fullOffloaded"`
		// RAM is the memory usage for loading the GGUF file in RAM.
		RAM StableDiffusionCppRunEstimateMemory `json:"ram"`
		// VRAMs is the memory usage for loading the GGUF file in VRAM per device.
		VRAMs []StableDiffusionCppRunEstimateMemory `json:"vrams"`
	}

	// StableDiffusionCppRunEstimateMemory represents the memory usage for loading the GGUF file in llama.cpp.
	StableDiffusionCppRunEstimateMemory struct {
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
func (e StableDiffusionCppRunEstimate) SummarizeItem(
	mmap bool,
	nonUMARamFootprint, nonUMAVramFootprint uint64,
) (emi StableDiffusionCppRunEstimateSummaryItem) {
	emi.FullOffloaded = e.FullOffloaded

	// RAM.
	{
		fp := e.Devices[0].Footprint
		wg := e.Devices[0].Weight
		cp := e.Devices[0].Computation

		// UMA.
		emi.RAM.UMA = fp + wg + cp

		// NonUMA.
		emi.RAM.NonUMA = GGUFBytesScalar(nonUMARamFootprint) + emi.RAM.UMA
	}

	// VRAMs.
	emi.VRAMs = make([]StableDiffusionCppRunEstimateMemory, len(e.Devices)-1)
	{
		for i, d := range e.Devices[1:] {
			fp := d.Footprint
			wg := d.Weight
			cp := d.Computation

			emi.VRAMs[i].Remote = d.Remote
			emi.VRAMs[i].Position = d.Position

			// UMA.
			emi.VRAMs[i].UMA = fp + wg + /* cp */ 0
			if d.Remote {
				emi.VRAMs[i].UMA += cp
			}

			// NonUMA.
			emi.VRAMs[i].NonUMA = GGUFBytesScalar(nonUMAVramFootprint) + fp + wg + cp
		}
	}

	// Add antoencoder's usage.
	if e.Autoencoder != nil {
		aemi := e.Autoencoder.SummarizeItem(mmap, 0, 0)
		emi.RAM.UMA += aemi.RAM.UMA
		emi.RAM.NonUMA += aemi.RAM.NonUMA
		for i, v := range aemi.VRAMs {
			emi.VRAMs[i].UMA += v.UMA
			emi.VRAMs[i].NonUMA += v.NonUMA
		}
	}

	// Add conditioners' usage.
	for i := range e.Conditioners {
		cemi := e.Conditioners[i].SummarizeItem(mmap, 0, 0)
		emi.RAM.UMA += cemi.RAM.UMA
		emi.RAM.NonUMA += cemi.RAM.NonUMA
		for i, v := range cemi.VRAMs {
			emi.VRAMs[i].UMA += v.UMA
			emi.VRAMs[i].NonUMA += v.NonUMA
		}
	}

	// Add upscaler's usage.
	if e.Upscaler != nil {
		uemi := e.Upscaler.SummarizeItem(mmap, 0, 0)
		emi.RAM.UMA += uemi.RAM.UMA
		emi.RAM.NonUMA += uemi.RAM.NonUMA
		// NB(thxCode): all VRAMs should offload to the first device at present.
		var vramUMA, vramNonUMA GGUFBytesScalar
		for _, v := range uemi.VRAMs {
			vramUMA += v.UMA
			vramNonUMA += v.NonUMA
		}
		if e.Upscaler.FullOffloaded {
			emi.VRAMs[0].UMA += vramUMA
			emi.VRAMs[0].NonUMA += vramNonUMA
		} else {
			emi.RAM.UMA += vramUMA
			emi.RAM.NonUMA += vramNonUMA
		}
	}

	// Add control net's usage.
	if e.ControlNet != nil {
		cnemi := e.ControlNet.SummarizeItem(mmap, 0, 0)
		emi.RAM.UMA += cnemi.RAM.UMA
		emi.RAM.NonUMA += cnemi.RAM.NonUMA
		// NB(thxCode): all VRAMs should offload to the first device at present.
		var vramUMA, vramNonUMA GGUFBytesScalar
		for _, v := range cnemi.VRAMs {
			vramUMA += v.UMA
			vramNonUMA += v.NonUMA
		}
		if e.ControlNet.FullOffloaded {
			emi.VRAMs[0].UMA += vramUMA
			emi.VRAMs[0].NonUMA += vramNonUMA
		} else {
			emi.RAM.UMA += vramUMA
			emi.RAM.NonUMA += vramNonUMA
		}
	}

	return emi
}

// Summarize returns the corresponding StableDiffusionCppRunEstimate with the given options.
func (e StableDiffusionCppRunEstimate) Summarize(
	mmap bool,
	nonUMARamFootprint, nonUMAVramFootprint uint64,
) (es StableDiffusionCppRunEstimateSummary) {
	// Items.
	es.Items = []StableDiffusionCppRunEstimateSummaryItem{
		e.SummarizeItem(mmap, nonUMARamFootprint, nonUMAVramFootprint),
	}

	// Just copy from the original estimate.
	es.Type = e.Type
	es.Architecture = e.Architecture
	es.FlashAttention = e.FlashAttention
	es.NoMMap = e.NoMMap
	es.ImageOnly = e.ImageOnly
	es.Distributable = e.Distributable

	return es
}

func normalizeArchitecture(arch string) string {
	return stringx.ReplaceAllFunc(arch, func(r rune) rune {
		switch r {
		case ' ', '.', '-', '/', ':':
			return '_' // Replace with underscore.
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A' // Lowercase.
		}
		return r
	})
}
