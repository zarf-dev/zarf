// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom contains tools for generating SBOMs.
package sbom

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/artifact"
	syftFile "github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/format"
	"github.com/anchore/syft/syft/format/syftjson"
	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/pkg/cataloger"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/defenseunicorns/pkg/helpers/v2"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logging"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// Builder is the main struct used to build SBOM artifacts.
type Builder struct {
	spinner    *message.Spinner
	cachePath  string
	imagesPath string
	outputDir  string
	jsonList   []byte
}

//go:embed viewer/*
var viewerAssets embed.FS
var transformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

var componentPrefix = "zarf-component-"

// Catalog catalogs the given components and images to create an SBOM.
func Catalog(ctx context.Context, componentSBOMs map[string]*layout.ComponentSBOM, imageList []transform.Image, paths *layout.PackagePaths) error {
	imageCount := len(imageList)
	componentCount := len(componentSBOMs)
	builder := Builder{
		spinner:    message.NewProgressSpinner("Creating SBOMs for %d images and %d components with files.", imageCount, componentCount),
		cachePath:  config.GetAbsCachePath(),
		imagesPath: paths.Images.Base,
		outputDir:  paths.SBOMs.Path,
	}
	defer builder.spinner.Stop()

	// Ensure the sbom directory exists
	_ = helpers.CreateDirectory(builder.outputDir, helpers.ReadWriteExecuteUser)

	// Generate a list of images and files for the sbom viewer
	json, err := builder.generateJSONList(componentSBOMs, imageList)
	if err != nil {
		builder.spinner.Errorf(err, "Unable to generate the SBOM image list")
		return err
	}
	builder.jsonList = json

	// Generate SBOM for each image
	currImage := 1
	for _, refInfo := range imageList {
		builder.spinner.Updatef("Creating image SBOMs (%d of %d): %s", currImage, imageCount, refInfo.Reference)

		// Get the image that we are creating an SBOM for
		img, err := utils.LoadOCIImage(paths.Images.Base, refInfo)
		if err != nil {
			builder.spinner.Errorf(err, "Unable to load the image to generate an SBOM")
			return err
		}

		jsonData, err := builder.createImageSBOM(img, refInfo.Reference)
		if err != nil {
			builder.spinner.Errorf(err, "Unable to create SBOM for image %s", refInfo.Reference)
			return err
		}

		if err = builder.createSBOMViewerAsset(refInfo.Reference, jsonData); err != nil {
			builder.spinner.Errorf(err, "Unable to create SBOM viewer for image %s", refInfo.Reference)
			return err
		}

		currImage++
	}

	currComponent := 1

	// Generate SBOM for each component
	for component := range componentSBOMs {
		builder.spinner.Updatef("Creating component file SBOMs (%d of %d): %s", currComponent, componentCount, component)

		if componentSBOMs[component] == nil {
			logging.FromContextOrDiscard(ctx).Debug("component has invalid SBOM, skipping", "component", component)
			continue
		}

		jsonData, err := builder.createFileSBOM(*componentSBOMs[component], component)
		if err != nil {
			builder.spinner.Errorf(err, "Unable to create SBOM for component %s", component)
			return err
		}

		if err = builder.createSBOMViewerAsset(fmt.Sprintf("%s%s", componentPrefix, component), jsonData); err != nil {
			builder.spinner.Errorf(err, "Unable to create SBOM viewer for component %s", component)
			return err
		}

		currComponent++
	}

	// Include the compare tool if there are any image SBOMs OR component SBOMs
	if len(componentSBOMs) > 0 || len(imageList) > 0 {
		if err := builder.createSBOMCompareAsset(); err != nil {
			builder.spinner.Errorf(err, "Unable to create SBOM compare tool")
			return err
		}
	}

	if err := paths.SBOMs.Archive(); err != nil {
		builder.spinner.Errorf(err, "Unable to archive SBOMs")
		return err
	}

	builder.spinner.Success()

	return nil
}

// createImageSBOM uses syft to generate SBOM for an image,
// some code/structure migrated from https://github.com/testifysec/go-witness/blob/v0.1.12/attestation/syft/syft.go.
func (b *Builder) createImageSBOM(img v1.Image, src string) ([]byte, error) {
	// Get the image reference.
	refInfo, err := transform.ParseImageRef(src)
	if err != nil {
		return nil, fmt.Errorf("failed to create ref for image %s: %w", src, err)
	}

	// Create the sbom.
	imageCachePath := filepath.Join(b.cachePath, layout.ImagesDir)

	// Ensure the image cache directory exists.
	if err := helpers.CreateDirectory(imageCachePath, helpers.ReadWriteExecuteUser); err != nil {
		return nil, err
	}

	syftImage := image.NewImage(img, file.NewTempDirGenerator("zarf"), imageCachePath, image.WithTags(refInfo.Reference))
	if err := syftImage.Read(); err != nil {
		return nil, err
	}

	syftSource, err := source.NewFromStereoscopeImageObject(syftImage, refInfo.Reference, nil)
	if err != nil {
		return nil, err
	}

	catalog, relationships, distro, err := syft.CatalogPackages(syftSource, cataloger.DefaultConfig())
	if err != nil {
		return nil, err
	}

	artifact := sbom.SBOM{
		Descriptor: sbom.Descriptor{
			Name: "zarf",
		},
		Source: syftSource.Describe(),
		Artifacts: sbom.Artifacts{
			Packages:          catalog,
			LinuxDistribution: distro,
		},
		Relationships: relationships,
	}

	jsonData, err := format.Encode(artifact, syftjson.NewFormatEncoder())
	if err != nil {
		return nil, err
	}

	// Write the sbom to disk using the image ref as the filename
	filename := fmt.Sprintf("%s.json", refInfo.Reference)
	sbomFile, err := b.createSBOMFile(filename)
	if err != nil {
		return nil, err
	}
	defer sbomFile.Close()

	if _, err = sbomFile.Write(jsonData); err != nil {
		return nil, err
	}

	// Return the json data
	return jsonData, nil
}

// createPathSBOM uses syft to generate SBOM for a filepath.
func (b *Builder) createFileSBOM(componentSBOM layout.ComponentSBOM, component string) ([]byte, error) {
	catalog := pkg.NewCollection()
	relationships := []artifact.Relationship{}
	parentSource, err := source.NewFromDirectoryPath(componentSBOM.Component.Base)
	if err != nil {
		return nil, err
	}

	for _, sbomFile := range componentSBOM.Files {
		// Create the sbom source
		fileSource, err := source.NewFromFile(source.FileConfig{Path: sbomFile})
		if err != nil {
			return nil, err
		}

		// Dogsled distro since this is not a linux image we are scanning
		cat, rel, _, err := syft.CatalogPackages(fileSource, cataloger.DefaultConfig())
		if err != nil {
			return nil, err
		}

		for pkg := range cat.Enumerate() {
			containsSource := false

			// See if the source locations for this package contain the file Zarf indexed
			for _, location := range pkg.Locations.ToSlice() {
				if location.RealPath == fileSource.Describe().Metadata.(source.FileSourceMetadata).Path {
					containsSource = true
				}
			}

			// If the locations do not contain the source file (i.e. the package was inside a tarball), add the file source
			if !containsSource {
				sourceLocation := syftFile.NewLocation(fileSource.Describe().Metadata.(source.FileSourceMetadata).Path)
				pkg.Locations.Add(sourceLocation)
			}

			catalog.Add(pkg)
		}

		for _, r := range rel {
			relationships = append(relationships, artifact.Relationship{
				From: parentSource,
				To:   r.To,
				Type: r.Type,
				Data: r.Data,
			})
		}
	}

	artifact := sbom.SBOM{
		Descriptor: sbom.Descriptor{
			Name: "zarf",
		},
		Source: parentSource.Describe(),
		Artifacts: sbom.Artifacts{
			Packages:          catalog,
			LinuxDistribution: &linux.Release{},
		},
		Relationships: relationships,
	}

	jsonData, err := format.Encode(artifact, syftjson.NewFormatEncoder())
	if err != nil {
		return nil, err
	}

	// Write the sbom to disk using the component prefix and name as the filename
	filename := fmt.Sprintf("%s%s.json", componentPrefix, component)
	sbomFile, err := b.createSBOMFile(filename)
	if err != nil {
		return nil, err
	}
	defer sbomFile.Close()

	if _, err = sbomFile.Write(jsonData); err != nil {
		return nil, err
	}

	// Return the json data
	return jsonData, nil
}

func (b *Builder) getNormalizedFileName(identifier string) string {
	return transformRegex.ReplaceAllString(identifier, "_")
}

func (b *Builder) createSBOMFile(filename string) (*os.File, error) {
	path := filepath.Join(b.outputDir, b.getNormalizedFileName(filename))
	return os.Create(path)
}
