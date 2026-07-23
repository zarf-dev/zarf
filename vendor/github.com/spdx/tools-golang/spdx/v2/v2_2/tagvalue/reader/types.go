// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"github.com/spdx/tools-golang/spdx/v2/common"
	"github.com/spdx/tools-golang/spdx/v2/v2_2"
)

type tvParser struct {
	// document into which data is being parsed
	doc *v2_2.Document

	// current parser state
	st tvParserState

	// current SPDX item being filled in, if any
	pkg       *v2_2.Package
	pkgExtRef *v2_2.PackageExternalReference
	file      *v2_2.File
	fileAOP   *v2_2.ArtifactOfProject
	snippet   *v2_2.Snippet
	otherLic  *v2_2.OtherLicense
	rln       *v2_2.Relationship
	ann       *v2_2.Annotation
	rev       *v2_2.Review
	// don't need creation info pointer b/c only one,
	// and we can get to it via doc.CreationInfo
}

// parser state (SPDX document version 2.2)
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
