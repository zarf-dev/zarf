// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_3"
)

type tvParser struct {
	// document into which data is being parsed
	doc *spdx.Document

	// current parser state
	st tvParserState

	// current SPDX item being filled in, if any
	pkg       *spdx.Package
	pkgExtRef *spdx.PackageExternalReference
	file      *spdx.File
	fileAOP   *spdx.ArtifactOfProject
	snippet   *spdx.Snippet
	otherLic  *spdx.OtherLicense
	rln       *spdx.Relationship
	ann       *spdx.Annotation
	rev       *spdx.Review
	// don't need creation info pointer b/c only one,
	// and we can get to it via doc.CreationInfo
}

// parser state (SPDX document)
type tvParserState int

const (
	// at beginning of document
	psStart tvParserState = iota

	// in document creation info section
	psCreationInfo

	// in package data section
	psPackage

	// in file data section (including "unpackaged" files)
	psFile

	// in snippet data section (including "unpackaged" files)
	psSnippet

	// in other license section
	psOtherLicense

	// in review section
	psReview
)

const nullSpdxElementId = common.ElementID("")
