// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package writer

import (
	"fmt"
	"io"

	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_2"
)

func renderAnnotation(ann *spdx.Annotation, w io.Writer) error {
	if ann.Annotator.Annotator != "" && ann.Annotator.AnnotatorType != "" {
		fmt.Fprintf(w, "Annotator: %s: %s\n", ann.Annotator.AnnotatorType, ann.Annotator.Annotator)
	}
	if ann.AnnotationDate != "" {
		fmt.Fprintf(w, "AnnotationDate: %s\n", ann.AnnotationDate)
	}
	if ann.AnnotationType != "" {
		fmt.Fprintf(w, "AnnotationType: %s\n", ann.AnnotationType)
	}
	annIDStr := common.RenderDocElementID(ann.AnnotationSPDXIdentifier)
	if annIDStr != "SPDXRef-" {
		fmt.Fprintf(w, "SPDXREF: %s\n", annIDStr)
	}
	if ann.AnnotationComment != "" {
		fmt.Fprintf(w, "AnnotationComment: %s\n", textify(ann.AnnotationComment))
	}

	return nil
}
