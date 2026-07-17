// This file is part of CycloneDX Go
//
// Licensed under the Apache License, Version 2.0 (the “License”);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an “AS IS” BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0
// Copyright (c) OWASP Foundation. All Rights Reserved.

package cyclonedx

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
)

type BOMEncoder interface {
	// Encode encodes a given BOM.
	Encode(bom *BOM) error

	// EncodeVersion encodes a given BOM in a specific version of the specification.
	// Choosing a lower spec version than what the BOM was constructed for will result
	// in loss of information. The original BOM struct is guaranteed to not be modified.
	EncodeVersion(bom *BOM, version SpecVersion) error

	// SetPretty toggles prettified output.
	SetPretty(pretty bool) BOMEncoder

	// SetEscapeHTML toggles escaped HTML output.
	SetEscapeHTML(escapeHTML bool) BOMEncoder
}

func NewBOMEncoder(writer io.Writer, format BOMFileFormat) BOMEncoder {
	if format == BOMFileFormatJSON {
		return &jsonBOMEncoder{writer: writer, escapeHTML: true}
	}
	return &xmlBOMEncoder{writer: writer}
}

type jsonBOMEncoder struct {
	writer     io.Writer
	pretty     bool
	escapeHTML bool
}

// Encode implements the BOMEncoder interface.
func (j jsonBOMEncoder) Encode(bom *BOM) error {
	if bom.SpecVersion < SpecVersion1_2 {
		return fmt.Errorf("json format is not supported for specification versions lower than %s", SpecVersion1_2)
	}

	encoder := json.NewEncoder(j.writer)
	encoder.SetEscapeHTML(j.escapeHTML)
	if j.pretty {
		encoder.SetIndent("", "  ")
	}

	return encoder.Encode(bom)
}

// EncodeVersion implements the BOMEncoder interface.
func (j jsonBOMEncoder) EncodeVersion(bom *BOM, specVersion SpecVersion) (err error) {
	bom, err = bom.copyAndConvert(specVersion)
	if err != nil {
		return
	}

	return j.Encode(bom)
}

// SetPretty implements the BOMEncoder interface.
func (j *jsonBOMEncoder) SetPretty(pretty bool) BOMEncoder {
	j.pretty = pretty
	return j
}

// SetEscapeHTML implements the BOMEncoder interface.
func (j *jsonBOMEncoder) SetEscapeHTML(escapeHTML bool) BOMEncoder {
	j.escapeHTML = escapeHTML
	return j
}

type xmlBOMEncoder struct {
	writer io.Writer
	pretty bool
}

// Encode implements the BOMEncoder interface.
func (x xmlBOMEncoder) Encode(bom *BOM) error {
	if _, err := fmt.Fprintf(x.writer, xml.Header); err != nil {
		return err
	}

	encoder := xml.NewEncoder(x.writer)
	if x.pretty {
		encoder.Indent("", "  ")
	}

	return encoder.Encode(bom)
}

// EncodeVersion implements the BOMEncoder interface.
func (x xmlBOMEncoder) EncodeVersion(bom *BOM, specVersion SpecVersion) (err error) {
	bom, err = bom.copyAndConvert(specVersion)
	if err != nil {
		return
	}

	return x.Encode(bom)
}

// SetPretty implements the BOMEncoder interface.
func (x *xmlBOMEncoder) SetPretty(pretty bool) BOMEncoder {
	x.pretty = pretty
	return x
}

// SetEscapeHTML implements the BOMEncoder interface.
func (j *xmlBOMEncoder) SetEscapeHTML(escapeHTML bool) BOMEncoder {
	// NOOP -- XML always needs to escape HTML
	return j
}
