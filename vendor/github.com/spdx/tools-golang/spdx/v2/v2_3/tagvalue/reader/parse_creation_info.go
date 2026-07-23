// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"fmt"
	"strings"

	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_3"
)

func (parser *tvParser) parsePairFromCreationInfo(tag string, value string) error {
	// fail if not in Creation Info parser state
	if parser.st != psCreationInfo {
		return fmt.Errorf("got invalid state %v in parsePairFromCreationInfo", parser.st)
	}

	// create an SPDX Creation Info data struct if we don't have one already
	if parser.doc.CreationInfo == nil {
		parser.doc.CreationInfo = &spdx.CreationInfo{}
	}

	ci := parser.doc.CreationInfo
	switch tag {
	case "LicenseListVersion":
		ci.LicenseListVersion = value
	case "Creator":
		subkey, subvalue, err := extractSubs(value)
		if err != nil {
			return err
		}

		creator := common.Creator{Creator: subvalue}
		switch subkey {
		case "Person", "Organization", "Tool":
			creator.CreatorType = subkey
		default:
			return fmt.Errorf("unrecognized Creator type %v", subkey)
		}

		ci.Creators = append(ci.Creators, creator)
	case "Created":
		ci.Created = value
	case "CreatorComment":
		ci.CreatorComment = value

	// tag for going on to package section
	case "PackageName":
		// error if last file does not have an identifier
		// this may be a null case: can we ever have a "last file" in
		// the "creation info" state? should go on to "file" state
		// even when parsing unpackaged files.
		if parser.file != nil && parser.file.FileSPDXIdentifier == nullSpdxElementId {
			return fmt.Errorf("file with FileName %s does not have SPDX identifier", parser.file.FileName)
		}
		parser.st = psPackage
		parser.pkg = &spdx.Package{
			FilesAnalyzed:             true,
			IsFilesAnalyzedTagPresent: false,
		}
		return parser.parsePairFromPackage(tag, value)
	// tag for going on to _unpackaged_ file section
	case "FileName":
		// leave pkg as nil, so that packages will be placed in Files
		parser.st = psFile
		parser.pkg = nil
		return parser.parsePairFromFile(tag, value)
	// tag for going on to other license section
	case "LicenseID":
		parser.st = psOtherLicense
		return parser.parsePairFromOtherLicense(tag, value)
	// tag for going on to review section (DEPRECATED)
	case "Reviewer":
		parser.st = psReview
		return parser.parsePairFromReview(tag, value)
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
	case "DocumentComment":
		parser.st = psStart
		return parser.parsePairFromStart(tag, value)
	case "ExternalDocumentRef":
		parser.st = psStart
		return parser.parsePairFromStart(tag, value)
	default:
		return fmt.Errorf("received unknown tag %v in CreationInfo section", tag)
	}

	return nil
}

// ===== Helper functions =====

func extractExternalDocumentReference(value string) (string, string, string, string, error) {
	sp := strings.Split(value, " ")
	// remove any that are just whitespace
	keepSp := []string{}
	for _, s := range sp {
		ss := strings.TrimSpace(s)
		if ss != "" {
			keepSp = append(keepSp, ss)
		}
	}

	var documentRefID, uri, alg, checksum string

	// now, should have 4 items (or 3, if Alg and Checksum were joined)
	// and should be able to map them
	if len(keepSp) == 4 {
		documentRefID = keepSp[0]
		uri = keepSp[1]
		alg = keepSp[2]
		// check that colon is present for alg, and remove it
		if !strings.HasSuffix(alg, ":") {
			return "", "", "", "", fmt.Errorf("algorithm does not end with colon")
		}
		alg = strings.TrimSuffix(alg, ":")
		checksum = keepSp[3]
	} else if len(keepSp) == 3 {
		documentRefID = keepSp[0]
		uri = keepSp[1]
		// split on colon into alg and checksum
		parts := strings.SplitN(keepSp[2], ":", 2)
		if len(parts) != 2 {
			return "", "", "", "", fmt.Errorf("missing colon separator between algorithm and checksum")
		}
		alg = parts[0]
		checksum = parts[1]
	} else {
		return "", "", "", "", fmt.Errorf("expected 4 elements, got %d", len(keepSp))
	}

	// additionally, we should be able to parse the first element as a
	// DocumentRef- ID string, and we should remove that prefix
	if !strings.HasPrefix(documentRefID, "DocumentRef-") {
		return "", "", "", "", fmt.Errorf("expected first element to have DocumentRef- prefix")
	}
	documentRefID = strings.TrimPrefix(documentRefID, "DocumentRef-")
	if documentRefID == "" {
		return "", "", "", "", fmt.Errorf("document identifier has nothing after prefix")
	}

	return documentRefID, uri, alg, checksum, nil
}
