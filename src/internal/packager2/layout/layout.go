// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for inteacting the Zarf packages.
package layout

import (
	goyaml "github.com/goccy/go-yaml"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/deprecated"
)

// Constants used in the default package layout.
const (
	ZarfYAML  = "zarf.yaml"
	Signature = "zarf.yaml.sig"
	Checksums = "checksums.txt"

	ImagesDir     = "images"
	ComponentsDir = "components"

	SBOMDir = "zarf-sbom"
	SBOMTar = "sboms.tar"

	IndexJSON = "index.json"
	OCILayout = "oci-layout"
)

// ComponentDir is the type for the different directories in a component.
type ComponentDir string

// Different component directory types.
const (
	RepoComponentDir      ComponentDir = "repos"
	FilesComponentDir     ComponentDir = "files"
	ChartsComponentDir    ComponentDir = "charts"
	ManifestsComponentDir ComponentDir = "manifests"
	DataComponentDir      ComponentDir = "data"
	ValuesComponentDir    ComponentDir = "values"
)

// ParseZarfPackage parses the yaml passed as a byte slice and applies potential schema migrations.
func ParseZarfPackage(b []byte) (v1alpha1.ZarfPackage, error) {
	var pkg v1alpha1.ZarfPackage
	err := goyaml.Unmarshal(b, &pkg)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	if len(pkg.Build.Migrations) > 0 {
		for idx, component := range pkg.Components {
			pkg.Components[idx], _ = deprecated.MigrateComponent(pkg.Build, component)
		}
	}
	return pkg, nil
}
