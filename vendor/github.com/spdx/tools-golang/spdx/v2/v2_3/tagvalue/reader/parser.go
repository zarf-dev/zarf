// Package reader contains functions to read, load and parse SPDX tag-value files.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package reader

import (
	"fmt"

	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_3"
	"github.com/spdx/tools-golang/tagvalue/reader"
)

// ParseTagValues takes a list of (tag, value) pairs, parses it and returns
// a pointer to a parsed SPDX Document.
func ParseTagValues(tvs []reader.TagValuePair) (*spdx.Document, error) {
	parser := tvParser{}
	for _, tv := range tvs {
		err := parser.parsePair(tv.Tag, tv.Value)
		if err != nil {
			return nil, err
		}
	}
	if parser.file != nil && parser.file.FileSPDXIdentifier == nullSpdxElementId {
		return nil, fmt.Errorf("file with FileName %s does not have SPDX identifier", parser.file.FileName)
	}
	if parser.pkg != nil && parser.pkg.PackageSPDXIdentifier == nullSpdxElementId {
		return nil, fmt.Errorf("package with PackageName %s does not have SPDX identifier", parser.pkg.PackageName)
	}
	return parser.doc, nil
}

func (parser *tvParser) parsePair(tag string, value string) error {
	switch parser.st {
	case psStart:
		return parser.parsePairFromStart(tag, value)
	case psCreationInfo:
		return parser.parsePairFromCreationInfo(tag, value)
	case psPackage:
		return parser.parsePairFromPackage(tag, value)
	case psFile:
		return parser.parsePairFromFile(tag, value)
	case psSnippet:
		return parser.parsePairFromSnippet(tag, value)
	case psOtherLicense:
		return parser.parsePairFromOtherLicense(tag, value)
	case psReview:
		return parser.parsePairFromReview(tag, value)
	default:
		return fmt.Errorf("parser state %v not recognized when parsing (%s, %s)", parser.st, tag, value)
	}
}

func (parser *tvParser) parsePairFromStart(tag string, value string) error {
	// fail if not in Start parser state
	if parser.st != psStart {
		return fmt.Errorf("got invalid state %v in parsePairFromStart", parser.st)
	}

	// create an SPDX Document data struct if we don't have one already
	if parser.doc == nil {
		parser.doc = &spdx.Document{ExternalDocumentReferences: []spdx.ExternalDocumentRef{}}
	}

	switch tag {
	case "DocumentComment":
		parser.doc.DocumentComment = value
	case "SPDXVersion":
		parser.doc.SPDXVersion = value
	case "DataLicense":
		parser.doc.DataLicense = value
	case "SPDXID":
		eID, err := extractElementID(value)
		if err != nil {
			return err
		}
		parser.doc.SPDXIdentifier = eID
	case "DocumentName":
		parser.doc.DocumentName = value
	case "DocumentNamespace":
		parser.doc.DocumentNamespace = value
	case "ExternalDocumentRef":
		documentRefID, uri, alg, checksum, err := extractExternalDocumentReference(value)
		if err != nil {
			return err
		}
		edr := spdx.ExternalDocumentRef{
			DocumentRefID: common.DocumentID(documentRefID),
			URI:           uri,
			Checksum:      common.Checksum{Algorithm: common.ChecksumAlgorithm(alg), Value: checksum},
		}
		parser.doc.ExternalDocumentReferences = append(parser.doc.ExternalDocumentReferences, edr)
	default:
		// move to Creation Info parser state
		parser.st = psCreationInfo
		return parser.parsePairFromCreationInfo(tag, value)
	}

	return nil
}
