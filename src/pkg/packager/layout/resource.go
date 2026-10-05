// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"path"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

const (
	// DocumentationDir contains individually-addressable documentation resources in v1beta1 packages.
	DocumentationDir = "documentation"
)

// UsesGranularResourceLayout reports whether a package stores documentation and SBOMs as individual files.
// v1alpha1, including definitions that omit apiVersion, retains archive-backed resources.
func UsesGranularResourceLayout(pkg api.Package) bool {
	return pkg.GetAPIVersion() != v1alpha1.APIVersion
}

// DocumentationResourcePath returns the v1beta1 package-relative path for a documentation file.
func DocumentationResourcePath(fileName string) string {
	return path.Join(DocumentationDir, fileName)
}
