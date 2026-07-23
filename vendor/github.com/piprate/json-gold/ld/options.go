// Copyright 2015-2017 Piprate Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ld

type Embed string

const (
	JsonLd_1_0       = "json-ld-1.0"              //nolint:stylecheck
	JsonLd_1_1       = "json-ld-1.1"              //nolint:stylecheck
	JsonLd_1_1_Frame = "json-ld-1.1-expand-frame" //nolint:stylecheck

	EmbedLast   = "@last"
	EmbedAlways = "@always"
	EmbedNever  = "@never"
)

// JsonLdOptions type as specified in the JSON-LD-API specification:
// http://www.w3.org/TR/json-ld-api/#the-jsonldoptions-type
type JsonLdOptions struct { //nolint:stylecheck

	// Base options: http://www.w3.org/TR/json-ld-api/#idl-def-JsonLdOptions

	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-base
	Base string
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-compactArrays
	CompactArrays bool
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-expandContext
	ExpandContext interface{}
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-processingMode
	ProcessingMode string
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-documentLoader
	DocumentLoader DocumentLoader

	// Frame options: http://json-ld.org/spec/latest/json-ld-framing/

	Embed        Embed
	Explicit     bool
	RequireAll   bool
	FrameDefault bool
	OmitDefault  bool
	OmitGraph    bool

	// RDF conversion options: http://www.w3.org/TR/json-ld-api/#serialize-rdf-as-json-ld-algorithm

	UseRdfType            bool
	UseNativeTypes        bool
	ProduceGeneralizedRdf bool

	// The following properties aren't in the spec

	InputFormat   string
	Format        string
	Algorithm     string
	UseNamespaces bool
	OutputForm    string
	SafeMode      bool
}

// NewJsonLdOptions creates and returns new instance of JsonLdOptions with the given base.
func NewJsonLdOptions(base string) *JsonLdOptions { //nolint:stylecheck
	return &JsonLdOptions{
		Base:                  base,
		CompactArrays:         true,
		ProcessingMode:        JsonLd_1_1,
		DocumentLoader:        NewDefaultDocumentLoader(nil),
		Embed:                 EmbedLast,
		Explicit:              false,
		RequireAll:            true,
		FrameDefault:          false,
		OmitDefault:           false,
		OmitGraph:             false,
		UseRdfType:            false,
		UseNativeTypes:        false,
		ProduceGeneralizedRdf: false,
		InputFormat:           "",
		Format:                "",
		Algorithm:             AlgorithmURGNA2012,
		UseNamespaces:         false,
		OutputForm:            "",
		SafeMode:              false,
	}
}

// Copy creates a deep copy of JsonLdOptions object.
func (opt *JsonLdOptions) Copy() *JsonLdOptions {
	return &JsonLdOptions{
		Base:                  opt.Base,
		CompactArrays:         opt.CompactArrays,
		ExpandContext:         opt.ExpandContext,
		ProcessingMode:        opt.ProcessingMode,
		DocumentLoader:        opt.DocumentLoader,
		Embed:                 opt.Embed,
		Explicit:              opt.Explicit,
		RequireAll:            opt.RequireAll,
		FrameDefault:          opt.FrameDefault,
		OmitDefault:           opt.OmitDefault,
		OmitGraph:             opt.OmitGraph,
		UseRdfType:            opt.UseRdfType,
		UseNativeTypes:        opt.UseNativeTypes,
		ProduceGeneralizedRdf: opt.ProduceGeneralizedRdf,
		InputFormat:           opt.InputFormat,
		Format:                opt.Format,
		Algorithm:             opt.Algorithm,
		UseNamespaces:         opt.UseNamespaces,
		OutputForm:            opt.OutputForm,
		SafeMode:              opt.SafeMode,
	}
}
