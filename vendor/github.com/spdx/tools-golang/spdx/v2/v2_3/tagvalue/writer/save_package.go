// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package writer

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx "github.com/spdx/tools-golang/spdx/v2/v2_3"
)

func renderPackage(pkg *spdx.Package, w io.Writer) error {
	if pkg.PackageName != "" {
		fmt.Fprintf(w, "PackageName: %s\n", pkg.PackageName)
	}
	if pkg.PackageSPDXIdentifier != "" {
		fmt.Fprintf(w, "SPDXID: %s\n", common.RenderElementID(pkg.PackageSPDXIdentifier))
	}
	if pkg.PackageVersion != "" {
		fmt.Fprintf(w, "PackageVersion: %s\n", pkg.PackageVersion)
	}
	if pkg.PackageFileName != "" {
		fmt.Fprintf(w, "PackageFileName: %s\n", pkg.PackageFileName)
	}
	if pkg.PackageSupplier != nil && pkg.PackageSupplier.Supplier != "" {
		if pkg.PackageSupplier.SupplierType == "" {
			fmt.Fprintf(w, "PackageSupplier: %s\n", pkg.PackageSupplier.Supplier)
		} else {
			fmt.Fprintf(w, "PackageSupplier: %s: %s\n", pkg.PackageSupplier.SupplierType, pkg.PackageSupplier.Supplier)
		}
	}
	if pkg.PackageOriginator != nil && pkg.PackageOriginator.Originator != "" {
		if pkg.PackageOriginator.OriginatorType == "" {
			fmt.Fprintf(w, "PackageOriginator: %s\n", pkg.PackageOriginator.Originator)
		} else {
			fmt.Fprintf(w, "PackageOriginator: %s: %s\n", pkg.PackageOriginator.OriginatorType, pkg.PackageOriginator.Originator)
		}
	}
	if pkg.PackageDownloadLocation != "" {
		fmt.Fprintf(w, "PackageDownloadLocation: %s\n", pkg.PackageDownloadLocation)
	}
	if pkg.PrimaryPackagePurpose != "" {
		fmt.Fprintf(w, "PrimaryPackagePurpose: %s\n", pkg.PrimaryPackagePurpose)
	}
	if pkg.ReleaseDate != "" {
		fmt.Fprintf(w, "ReleaseDate: %s\n", pkg.ReleaseDate)
	}
	if pkg.BuiltDate != "" {
		fmt.Fprintf(w, "BuiltDate: %s\n", pkg.BuiltDate)
	}
	if pkg.ValidUntilDate != "" {
		fmt.Fprintf(w, "ValidUntilDate: %s\n", pkg.ValidUntilDate)
	}
	if pkg.FilesAnalyzed {
		if pkg.IsFilesAnalyzedTagPresent {
			fmt.Fprintf(w, "FilesAnalyzed: true\n")
		}
	} else {
		fmt.Fprintf(w, "FilesAnalyzed: false\n")
	}
	if pkg.PackageVerificationCode != nil && pkg.PackageVerificationCode.Value != "" && pkg.FilesAnalyzed == true {
		if len(pkg.PackageVerificationCode.ExcludedFiles) == 0 {
			fmt.Fprintf(w, "PackageVerificationCode: %s\n", pkg.PackageVerificationCode.Value)
		} else {
			fmt.Fprintf(w, "PackageVerificationCode: %s (excludes: %s)\n", pkg.PackageVerificationCode.Value, strings.Join(pkg.PackageVerificationCode.ExcludedFiles, ", "))
		}
	}

	for _, checksum := range pkg.PackageChecksums {
		fmt.Fprintf(w, "PackageChecksum: %s: %s\n", checksum.Algorithm, checksum.Value)
	}

	if pkg.PackageHomePage != "" {
		fmt.Fprintf(w, "PackageHomePage: %s\n", pkg.PackageHomePage)
	}
	if pkg.PackageSourceInfo != "" {
		fmt.Fprintf(w, "PackageSourceInfo: %s\n", textify(pkg.PackageSourceInfo))
	}
	if pkg.PackageLicenseConcluded != "" {
		fmt.Fprintf(w, "PackageLicenseConcluded: %s\n", pkg.PackageLicenseConcluded)
	}
	if pkg.FilesAnalyzed {
		for _, s := range pkg.PackageLicenseInfoFromFiles {
			fmt.Fprintf(w, "PackageLicenseInfoFromFiles: %s\n", s)
		}
	}
	if pkg.PackageLicenseDeclared != "" {
		fmt.Fprintf(w, "PackageLicenseDeclared: %s\n", pkg.PackageLicenseDeclared)
	}
	if pkg.PackageLicenseComments != "" {
		fmt.Fprintf(w, "PackageLicenseComments: %s\n", textify(pkg.PackageLicenseComments))
	}
	if pkg.PackageCopyrightText != "" {
		fmt.Fprintf(w, "PackageCopyrightText: %s\n", textify(pkg.PackageCopyrightText))
	}
	if pkg.PackageSummary != "" {
		fmt.Fprintf(w, "PackageSummary: %s\n", textify(pkg.PackageSummary))
	}
	if pkg.PackageDescription != "" {
		fmt.Fprintf(w, "PackageDescription: %s\n", textify(pkg.PackageDescription))
	}
	if pkg.PackageComment != "" {
		fmt.Fprintf(w, "PackageComment: %s\n", textify(pkg.PackageComment))
	}
	for _, s := range pkg.PackageExternalReferences {
		fmt.Fprintf(w, "ExternalRef: %s %s %s\n", s.Category, s.RefType, s.Locator)
		if s.ExternalRefComment != "" {
			fmt.Fprintf(w, "ExternalRefComment: %s\n", textify(s.ExternalRefComment))
		}
	}
	for _, s := range pkg.PackageAttributionTexts {
		fmt.Fprintf(w, "PackageAttributionText: %s\n", textify(s))
	}

	fmt.Fprintf(w, "\n")

	// also render any files for this package
	sort.Slice(pkg.Files, func(i, j int) bool {
		return pkg.Files[i].FileSPDXIdentifier < pkg.Files[j].FileSPDXIdentifier
	})
	for _, fi := range pkg.Files {
		renderFile(fi, w)
	}

	return nil
}
