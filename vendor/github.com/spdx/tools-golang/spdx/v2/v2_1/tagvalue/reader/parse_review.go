// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"fmt"

	"github.com/spdx/tools-golang/spdx/v2/v2_1"
)

func (parser *tvParser) parsePairFromReview(tag string, value string) error {
	switch tag {
	// tag for creating new review section
	case "Reviewer":
		parser.rev = &v2_1.Review{}
		parser.doc.Reviews = append(parser.doc.Reviews, parser.rev)
		subkey, subvalue, err := extractSubs(value)
		if err != nil {
			return err
		}
		switch subkey {
		case "Person":
			parser.rev.Reviewer = subvalue
			parser.rev.ReviewerType = "Person"
		case "Organization":
			parser.rev.Reviewer = subvalue
			parser.rev.ReviewerType = "Organization"
		case "Tool":
			parser.rev.Reviewer = subvalue
			parser.rev.ReviewerType = "Tool"
		default:
			return fmt.Errorf("unrecognized Reviewer type %v", subkey)
		}
	case "ReviewDate":
		parser.rev.ReviewDate = value
	case "ReviewComment":
		parser.rev.ReviewComment = value
	// for relationship tags, pass along but don't change state
	case "Relationship":
		parser.rln = &v2_1.Relationship{}
		parser.doc.Relationships = append(parser.doc.Relationships, parser.rln)
		return parser.parsePairForRelationship(tag, value)
	case "RelationshipComment":
		return parser.parsePairForRelationship(tag, value)
	// for annotation tags, pass along but don't change state
	case "Annotator":
		parser.ann = &v2_1.Annotation{}
		parser.doc.Annotations = append(parser.doc.Annotations, parser.ann)
		return parser.parsePairForAnnotation(tag, value)
	case "AnnotationDate":
		return parser.parsePairForAnnotation(tag, value)
	case "AnnotationType":
		return parser.parsePairForAnnotation(tag, value)
	case "SPDXREF":
		return parser.parsePairForAnnotation(tag, value)
	case "AnnotationComment":
		return parser.parsePairForAnnotation(tag, value)
	default:
		return fmt.Errorf("received unknown tag %v in Review section", tag)
	}

	return nil
}
