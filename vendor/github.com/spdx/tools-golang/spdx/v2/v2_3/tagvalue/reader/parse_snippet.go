// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"fmt"
	"strconv"

	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_3"
)

func (parser *tvParser) parsePairFromSnippet(tag string, value string) error {
	switch tag {
	// tag for creating new snippet section
	case "SnippetSPDXID":
		// check here whether the file contained an SPDX ID or not
		if parser.file != nil && parser.file.FileSPDXIdentifier == nullSpdxElementId {
			return fmt.Errorf("file with FileName %s does not have SPDX identifier", parser.file.FileName)
		}
		parser.snippet = &spdx.Snippet{}
		eID, err := extractElementID(value)
		if err != nil {
			return err
		}
		// FIXME: how should we handle where not associated with current file?
		if parser.file != nil {
			if parser.file.Snippets == nil {
				parser.file.Snippets = map[common.ElementID]*spdx.Snippet{}
			}
			parser.file.Snippets[eID] = parser.snippet
		}
		parser.snippet.SnippetSPDXIdentifier = eID
	// tag for creating new file section and going back to parsing File
	case "FileName":
		parser.st = psFile
		parser.snippet = nil
		return parser.parsePairFromFile(tag, value)
	// tag for creating new package section and going back to parsing Package
	case "PackageName":
		parser.st = psPackage
		parser.file = nil
		parser.snippet = nil
		return parser.parsePairFromPackage(tag, value)
	// tag for going on to other license section
	case "LicenseID":
		parser.st = psOtherLicense
		return parser.parsePairFromOtherLicense(tag, value)
	// tags for snippet data
	case "SnippetFromFileSPDXID":
		deID, err := extractDocElementID(value)
		if err != nil {
			return err
		}
		parser.snippet.SnippetFromFileSPDXIdentifier = deID.ElementRefID
	case "SnippetByteRange":
		byteStart, byteEnd, err := extractSubs(value)
		if err != nil {
			return err
		}
		bIntStart, err := strconv.Atoi(byteStart)
		if err != nil {
			return err
		}
		bIntEnd, err := strconv.Atoi(byteEnd)
		if err != nil {
			return err
		}

		if parser.snippet.Ranges == nil {
			parser.snippet.Ranges = []common.SnippetRange{}
		}
		byteRange := common.SnippetRange{StartPointer: common.SnippetRangePointer{Offset: bIntStart}, EndPointer: common.SnippetRangePointer{Offset: bIntEnd}}
		parser.snippet.Ranges = append(parser.snippet.Ranges, byteRange)
	case "SnippetLineRange":
		lineStart, lineEnd, err := extractSubs(value)
		if err != nil {
			return err
		}
		lInttStart, err := strconv.Atoi(lineStart)
		if err != nil {
			return err
		}
		lInttEnd, err := strconv.Atoi(lineEnd)
		if err != nil {
			return err
		}

		if parser.snippet.Ranges == nil {
			parser.snippet.Ranges = []common.SnippetRange{}
		}
		lineRange := common.SnippetRange{StartPointer: common.SnippetRangePointer{LineNumber: lInttStart}, EndPointer: common.SnippetRangePointer{LineNumber: lInttEnd}}
		parser.snippet.Ranges = append(parser.snippet.Ranges, lineRange)
	case "SnippetLicenseConcluded":
		parser.snippet.SnippetLicenseConcluded = value
	case "LicenseInfoInSnippet":
		parser.snippet.LicenseInfoInSnippet = append(parser.snippet.LicenseInfoInSnippet, value)
	case "SnippetLicenseComments":
		parser.snippet.SnippetLicenseComments = value
	case "SnippetCopyrightText":
		parser.snippet.SnippetCopyrightText = value
	case "SnippetComment":
		parser.snippet.SnippetComment = value
	case "SnippetName":
		parser.snippet.SnippetName = value
	case "SnippetAttributionText":
		parser.snippet.SnippetAttributionTexts = append(parser.snippet.SnippetAttributionTexts, value)
	// for relationship tags, pass along but don't change state
	case "Relationship":
		parser.rln = &spdx.Relationship{}
		parser.doc.Relationships = append(parser.doc.Relationships, parser.rln)
		return parser.parsePairForRelationship(tag, value)
	case "RelationshipComment":
		return parser.parsePairForRelationship(tag, value)
	// for annotation tags, pass along but don't change state
	case "Annotator":
		parser.ann = &spdx.Annotation{}
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
	// tag for going on to review section (DEPRECATED)
	case "Reviewer":
		parser.st = psReview
		return parser.parsePairFromReview(tag, value)
	default:
		return fmt.Errorf("received unknown tag %v in Snippet section", tag)
	}

	return nil
}
