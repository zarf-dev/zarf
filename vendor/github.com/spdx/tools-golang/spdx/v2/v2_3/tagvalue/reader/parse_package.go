// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"fmt"
	"strings"

	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_3"
)

func (parser *tvParser) parsePairFromPackage(tag string, value string) error {
	// expire pkgExtRef for anything other than a comment
	// (we'll actually handle the comment further below)
	if tag != "ExternalRefComment" {
		parser.pkgExtRef = nil
	}

	switch tag {
	case "PackageName":
		// if package already has a name, create and go on to a new package
		if parser.pkg == nil || parser.pkg.PackageName != "" {
			// check if the previous package contained an spdx Id or not
			if parser.pkg != nil && parser.pkg.PackageSPDXIdentifier == nullSpdxElementId {
				return fmt.Errorf("package with PackageName %s does not have SPDX identifier", parser.pkg.PackageName)
			}
			parser.pkg = &spdx.Package{
				FilesAnalyzed:             true,
				IsFilesAnalyzedTagPresent: false,
			}
		}
		parser.pkg.PackageName = value
	// tag for going on to file section
	case "FileName":
		parser.st = psFile
		return parser.parsePairFromFile(tag, value)
	// tag for going on to other license section
	case "LicenseID":
		parser.st = psOtherLicense
		return parser.parsePairFromOtherLicense(tag, value)
	case "SPDXID":
		eID, err := extractElementID(value)
		if err != nil {
			return err
		}
		parser.pkg.PackageSPDXIdentifier = eID
		if parser.doc.Packages == nil {
			parser.doc.Packages = []*spdx.Package{}
		}
		parser.doc.Packages = append(parser.doc.Packages, parser.pkg)
	case "PackageVersion":
		parser.pkg.PackageVersion = value
	case "PackageFileName":
		parser.pkg.PackageFileName = value
	case "PackageSupplier":
		supplier := &common.Supplier{Supplier: value}
		if value == "NOASSERTION" {
			parser.pkg.PackageSupplier = supplier
			break
		}

		subkey, subvalue, err := extractSubs(value)
		if err != nil {
			return err
		}
		switch subkey {
		case "Person", "Organization":
			supplier.Supplier = subvalue
			supplier.SupplierType = subkey
		default:
			return fmt.Errorf("unrecognized PackageSupplier type %v", subkey)
		}
		parser.pkg.PackageSupplier = supplier
	case "PackageOriginator":
		originator := &common.Originator{Originator: value}
		if value == "NOASSERTION" {
			parser.pkg.PackageOriginator = originator
			break
		}

		subkey, subvalue, err := extractSubs(value)
		if err != nil {
			return err
		}
		switch subkey {
		case "Person", "Organization":
			originator.Originator = subvalue
			originator.OriginatorType = subkey
		default:
			return fmt.Errorf("unrecognized PackageOriginator type %v", subkey)
		}
		parser.pkg.PackageOriginator = originator
	case "PackageDownloadLocation":
		parser.pkg.PackageDownloadLocation = value
	case "FilesAnalyzed":
		parser.pkg.IsFilesAnalyzedTagPresent = true
		if value == "false" {
			parser.pkg.FilesAnalyzed = false
		} else if value == "true" {
			parser.pkg.FilesAnalyzed = true
		}
	case "PackageVerificationCode":
		parser.pkg.PackageVerificationCode = extractCodeAndExcludes(value)
	case "PackageChecksum":
		subkey, subvalue, err := extractSubs(value)
		if err != nil {
			return err
		}
		if parser.pkg.PackageChecksums == nil {
			parser.pkg.PackageChecksums = []common.Checksum{}
		}
		switch common.ChecksumAlgorithm(subkey) {
		case common.SHA1,
			common.SHA224,
			common.SHA256,
			common.SHA384,
			common.SHA512,
			common.MD2,
			common.MD4,
			common.MD5,
			common.MD6,
			common.SHA3_256,
			common.SHA3_384,
			common.SHA3_512,
			common.BLAKE2b_256,
			common.BLAKE2b_384,
			common.BLAKE2b_512,
			common.BLAKE3,
			common.ADLER32:
			algorithm := common.ChecksumAlgorithm(subkey)
			parser.pkg.PackageChecksums = append(parser.pkg.PackageChecksums, common.Checksum{Algorithm: algorithm, Value: subvalue})
		default:
			return fmt.Errorf("got unknown checksum type %s", subkey)
		}
	case "PackageHomePage":
		parser.pkg.PackageHomePage = value
	case "PackageSourceInfo":
		parser.pkg.PackageSourceInfo = value
	case "PackageLicenseConcluded":
		parser.pkg.PackageLicenseConcluded = value
	case "PackageLicenseInfoFromFiles":
		parser.pkg.PackageLicenseInfoFromFiles = append(parser.pkg.PackageLicenseInfoFromFiles, value)
	case "PackageLicenseDeclared":
		parser.pkg.PackageLicenseDeclared = value
	case "PackageLicenseComments":
		parser.pkg.PackageLicenseComments = value
	case "PackageCopyrightText":
		parser.pkg.PackageCopyrightText = value
	case "PackageSummary":
		parser.pkg.PackageSummary = value
	case "PackageDescription":
		parser.pkg.PackageDescription = value
	case "PackageComment":
		parser.pkg.PackageComment = value
	case "PrimaryPackagePurpose":
		parser.pkg.PrimaryPackagePurpose = value
	case "ReleaseDate":
		parser.pkg.ReleaseDate = value
	case "BuiltDate":
		parser.pkg.BuiltDate = value
	case "ValidUntilDate":
		parser.pkg.ValidUntilDate = value
	case "PackageAttributionText":
		parser.pkg.PackageAttributionTexts = append(parser.pkg.PackageAttributionTexts, value)
	case "ExternalRef":
		parser.pkgExtRef = &spdx.PackageExternalReference{}
		parser.pkg.PackageExternalReferences = append(parser.pkg.PackageExternalReferences, parser.pkgExtRef)
		category, refType, locator, err := extractPackageExternalReference(value)
		if err != nil {
			return err
		}
		parser.pkgExtRef.Category = category
		parser.pkgExtRef.RefType = refType
		parser.pkgExtRef.Locator = locator
	case "ExternalRefComment":
		if parser.pkgExtRef == nil {
			return fmt.Errorf("no current ExternalRef found")
		}
		parser.pkgExtRef.ExternalRefComment = value
		// now, expire pkgExtRef anyway because it can have at most one comment
		parser.pkgExtRef = nil
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
		return fmt.Errorf("received unknown tag %v in Package section", tag)
	}

	return nil
}

// ===== Helper functions =====

func extractCodeAndExcludes(value string) *common.PackageVerificationCode {
	// FIXME this should probably be done using regular expressions instead
	// split by paren + word "excludes:"
	sp := strings.SplitN(value, "(excludes:", 2)
	if len(sp) < 2 {
		// not found; return the whole string as just the code
		return &common.PackageVerificationCode{Value: value, ExcludedFiles: []string{}}
	}

	// if we're here, code is in first part and excludes filename is in
	// second part, with trailing paren
	code := strings.TrimSpace(sp[0])
	parsedSp := strings.SplitN(sp[1], ")", 2)
	fileName := strings.TrimSpace(parsedSp[0])
	return &common.PackageVerificationCode{Value: code, ExcludedFiles: []string{fileName}}
}

func extractPackageExternalReference(value string) (string, string, string, error) {
	sp := strings.Split(value, " ")
	// remove any that are just whitespace
	keepSp := []string{}
	for _, s := range sp {
		ss := strings.TrimSpace(s)
		if ss != "" {
			keepSp = append(keepSp, ss)
		}
	}
	// now, should have 3 items and should be able to map them
	if len(keepSp) != 3 {
		return "", "", "", fmt.Errorf("expected 3 elements, got %d", len(keepSp))
	}
	return keepSp[0], keepSp[1], keepSp[2], nil
}
