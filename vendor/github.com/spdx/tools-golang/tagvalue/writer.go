// Package tvsaver is used to save tools-golang data structures
// as SPDX tag-value documents.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package tagvalue

import (
	"fmt"
	"io"

	"github.com/spdx/tools-golang/convert"
	"github.com/spdx/tools-golang/spdx/common"
	"github.com/spdx/tools-golang/spdx/v2/v2_1"
	v2_1_writer "github.com/spdx/tools-golang/spdx/v2/v2_1/tagvalue/writer"
	"github.com/spdx/tools-golang/spdx/v2/v2_2"
	v2_2_writer "github.com/spdx/tools-golang/spdx/v2/v2_2/tagvalue/writer"
	"github.com/spdx/tools-golang/spdx/v2/v2_3"
	v2_3_writer "github.com/spdx/tools-golang/spdx/v2/v2_3/tagvalue/writer"
)

// Write takes an io.Writer and an SPDX Document,
// and writes it to the writer in tag-value format. It returns error
// if any error is encountered.
func Write(doc common.AnyDocument, w io.Writer) error {
	doc = convert.FromPtr(doc)
	switch doc := doc.(type) {
	case v2_1.Document:
		return v2_1_writer.RenderDocument(&doc, w)
	case v2_2.Document:
		return v2_2_writer.RenderDocument(&doc, w)
	case v2_3.Document:
		return v2_3_writer.RenderDocument(&doc, w)
	}
	return fmt.Errorf("unsupported document type: %s", convert.Describe(doc))
}
