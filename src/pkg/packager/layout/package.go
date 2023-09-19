// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type PackagePaths struct {
	Base      string
	ZarfYAML  string
	Checksums string

	Signature string

	Components Components
	SBOMs      SBOMs
	Images     Images
}

type InjectionMadnessPaths struct {
	InjectionBinary      string
	SeedImagesDir        string
	InjectorPayloadTarGz string
}

func New(baseDir string) *PackagePaths {
	return &PackagePaths{
		Base:      baseDir,
		ZarfYAML:  filepath.Join(baseDir, ZarfYAML),
		Checksums: filepath.Join(baseDir, Checksums),
		Components: Components{
			Base: filepath.Join(baseDir, ComponentsDir),
		},
	}
}

func (pp *PackagePaths) WithSignature(keyPath string) *PackagePaths {
	if keyPath != "" {
		pp.Signature = filepath.Join(pp.Base, Signature)
	}
	return pp
}

func (pp *PackagePaths) WithImages() *PackagePaths {
	pp.Images.Base = filepath.Join(pp.Base, ImagesDir)
	pp.Images.OCILayout = filepath.Join(pp.Images.Base, OCILayout)
	pp.Images.Index = filepath.Join(pp.Images.Base, IndexJSON)
	return pp
}

func (pp *PackagePaths) WithSBOMs() *PackagePaths {
	pp.SBOMs = SBOMs{
		Path: filepath.Join(pp.Base, SBOMDir),
	}
	return pp
}

func (pp *PackagePaths) SetFromLayers(layers []ocispec.Descriptor) {
	paths := []string{}
	for _, layer := range layers {
		if layer.Annotations[ocispec.AnnotationTitle] != "" {
			paths = append(paths, layer.Annotations[ocispec.AnnotationTitle])
		}
	}
	pp.SetFromPaths(paths)
}

func (pp *PackagePaths) SetFromPaths(paths []string) {
	for _, abs := range paths {
		switch path := abs; {
		case path == ZarfYAML:
			pp.ZarfYAML = path
		case path == Signature:
			pp.Signature = path
		case path == Checksums:
			pp.Checksums = path
		case path == SBOMTar:
			pp.SBOMs.Path = path
		case path == filepath.Join(ImagesDir, "oci-layout"):
			pp.Images.OCILayout = path
		case path == filepath.Join(ImagesDir, "index.json"):
			pp.Images.Index = path
		case strings.HasPrefix(path, filepath.Join(ImagesDir, "blobs", "sha256")):
			pp.Images.AddBlob(filepath.Base(path))
		case (strings.HasPrefix(ComponentsDir, path) && strings.HasSuffix(path, ".tar")):
			name := filepath.Base(path)
			withoutSuffix := strings.TrimSuffix(name, filepath.Ext(name))
			pp.Components.Tarballs[withoutSuffix] = path
		}
	}
}

// Files returns a map of all the files in the package.
func (pp *PackagePaths) Files() map[string]string {
	pathMap := make(map[string]string)
	stripBase := func(path string) string {
		rel, _ := filepath.Rel(pp.Base, path)
		return rel
	}
	add := func(path string) {
		if filepath.Ext(path) != "" {
			pathMap[stripBase(path)] = path
		}
	}
	add(pp.ZarfYAML)
	add(pp.Signature)
	add(pp.Checksums)

	add(pp.Images.OCILayout)
	add(pp.Images.Index)
	for _, blob := range pp.Images.Blobs {
		if blob != "" {
			pathMap[stripBase(blob)] = blob
		}
	}

	for _, tarball := range pp.Components.Tarballs {
		add(tarball)
	}

	add(pp.SBOMs.Path)
	return pathMap
}
