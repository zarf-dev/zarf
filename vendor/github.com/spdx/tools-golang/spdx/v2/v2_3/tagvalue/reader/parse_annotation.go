// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"fmt"
)

func (parser *tvParser) parsePairForAnnotation(tag string, value string) error {
	if parser.ann == nil {
		return fmt.Errorf("no annotation struct created in parser ann pointer")
	}

	switch tag {
	case "Annotator":
		subkey, subvalue, err := extractSubs(value)
		if err != nil {
			return err
		}
		if subkey == "Person" || subkey == "Organization" || subkey == "Tool" {
			parser.ann.Annotator.AnnotatorType = subkey
			parser.ann.Annotator.Annotator = subvalue
			return nil
		}
		return fmt.Errorf("unrecognized Annotator type %v", subkey)
	case "AnnotationDate":
		parser.ann.AnnotationDate = value
	case "AnnotationType":
		parser.ann.AnnotationType = value
	case "SPDXREF":
		deID, err := extractDocElementID(value)
		if err != nil {
			return err
		}
		parser.ann.AnnotationSPDXIdentifier = deID
	case "AnnotationComment":
		parser.ann.AnnotationComment = value
	default:
		return fmt.Errorf("received unknown tag %v in Annotation section", tag)
	}

	return nil
}
