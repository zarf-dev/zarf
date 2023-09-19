// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
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
	for _, rel := range paths {
		switch path := rel; {
		case path == ZarfYAML:
			pp.ZarfYAML = filepath.Join(pp.Base, path)
		case path == Signature:
			pp.Signature = filepath.Join(pp.Base, path)
		case path == Checksums:
			pp.Checksums = filepath.Join(pp.Base, path)
		case path == SBOMTar:
			pp.SBOMs.Path = filepath.Join(pp.Base, path)
		case path == filepath.Join(ImagesDir, OCILayout):
			pp.Images.OCILayout = filepath.Join(pp.Base, path)
		case path == filepath.Join(ImagesDir, IndexJSON):
			pp.Images.Index = filepath.Join(pp.Base, path)
		case strings.HasPrefix(path, filepath.Join(ImagesDir, "blobs", "sha256")):
			if pp.Images.Base == "" {
				pp.Images.Base = filepath.Join(pp.Base, ImagesDir)
			}
			pp.Images.AddBlob(filepath.Base(path))
		case strings.HasPrefix(path, ComponentsDir) && filepath.Ext(path) == ".tar":
			if pp.Components.Base == "" {
				pp.Components.Base = filepath.Join(pp.Base, ComponentsDir)
			}
			componentName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if pp.Components.Tarballs == nil {
				pp.Components.Tarballs = make(map[string]string)
			}
			pp.Components.Tarballs[componentName] = filepath.Join(pp.Base, path)
		default:
			message.Debug("ignoring path", path)
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
		if path == "" {
			return
		}
		pathMap[stripBase(path)] = path
	}
	add(pp.ZarfYAML)
	add(pp.Signature)
	add(pp.Checksums)

	add(pp.Images.OCILayout)
	add(pp.Images.Index)
	for _, blob := range pp.Images.Blobs {
		add(blob)
	}

	for _, tarball := range pp.Components.Tarballs {
		add(tarball)
	}

	if filepath.Ext(pp.SBOMs.Path) == ".tar" {
		add(pp.SBOMs.Path)
	}
	return pathMap
}
