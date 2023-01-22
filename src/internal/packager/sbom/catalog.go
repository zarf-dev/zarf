// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom contains tools for generating SBOMs.
package sbom

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/pkg/cataloger"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Builder is the main struct used to build SBOM artifacts.
type Builder struct {
	spinner    *message.Spinner
	cachePath  string
	imagesPath string
	sbomPath   string
	jsonList   []byte
}

//go:embed viewer/*
var viewerAssets embed.FS
var transformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

var componentPrefix = "zarf-component-"

// Catalog catalogs the given components and images to create an SBOM.
func Catalog(componentSBOMs map[string]*types.ComponentSBOM, tagToImage map[name.Tag]v1.Image, imagesPath, sbomPath string) {
	imageCount := len(tagToImage)
	componentCount := len(componentSBOMs)
	builder := Builder{
		spinner:    message.NewProgressSpinner("Creating SBOMs for %d images and %d components with files.", imageCount, componentCount),
		cachePath:  config.GetAbsCachePath(),
		imagesPath: imagesPath,
		sbomPath:   sbomPath,
	}
	defer builder.spinner.Stop()

	// Ensure the sbom directory exists
	_ = utils.CreateDirectory(builder.sbomPath, 0700)

	// Generate a list of images and files for the sbom viewer
	if json, err := builder.generateJSONList(componentSBOMs, tagToImage); err != nil {
		builder.spinner.Fatalf(err, "Unable to generate the SBOM image list")
	} else {
		builder.jsonList = json
	}

	currImage := 1

	// Generate SBOM for each image
	for tag := range tagToImage {
		builder.spinner.Updatef("Creating image SBOMs (%d of %d): %s", currImage, imageCount, tag)

		jsonData, err := builder.createImageSBOM(tag)
		if err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM for image %s", tag)
		}

		if err = builder.createSBOMViewerAsset(tag.String(), jsonData); err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM viewer for image %s", tag)
		}

		currImage++
	}

	currComponent := 1

	// Generate SBOM for each component
	for component := range componentSBOMs {
		builder.spinner.Updatef("Creating component file SBOMs (%d of %d): %s", currComponent, componentCount, component)

		if componentSBOMs[component] == nil {
			message.Debugf("Component %s has invalid SBOM, skipping", component)
			continue
		}

		jsonData, err := builder.createFileSBOM(*componentSBOMs[component], component)
		if err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM for component %s", component)
		}

		if err = builder.createSBOMViewerAsset(fmt.Sprintf("%s%s", componentPrefix, component), jsonData); err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM viewer for component %s", component)
		}

		currImage++
	}

	if len(componentSBOMs) > 0 || len(tagToImage) > 0 {
		if err := builder.createSBOMCompareAsset(); err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM compare tool")
		}
	}

	builder.spinner.Success()
}

// createImageSBOM uses syft to generate SBOM for an image,
// some code/structure migrated from https://github.com/testifysec/go-witness/blob/v0.1.12/attestation/syft/syft.go.
func (b *Builder) createImageSBOM(tag name.Tag) ([]byte, error) {
	// Get the image
	tarballImg, err := tarball.ImageFromPath(b.imagesPath, &tag)
	if err != nil {
		return nil, err
	}

	// Create the sbom
	imageCachePath := filepath.Join(b.cachePath, config.ZarfImageCacheDir)
	syftImage := image.NewImage(tarballImg, imageCachePath, image.WithTags(tag.String()))
	if err := syftImage.Read(); err != nil {
		return nil, err
	}

	syftSource, err := source.NewFromImage(syftImage, "")
	if err != nil {
		return nil, err
	}

	catalog, relationships, distro, err := syft.CatalogPackages(&syftSource, cataloger.DefaultConfig())
	if err != nil {
		return nil, err
	}

	artifact := sbom.SBOM{
		Descriptor: sbom.Descriptor{
			Name: "zarf",
		},
		Source: syftSource.Metadata,
		Artifacts: sbom.Artifacts{
			PackageCatalog:    catalog,
			LinuxDistribution: distro,
		},
		Relationships: relationships,
	}

	jsonData, err := syft.Encode(artifact, syft.FormatByID(syft.JSONFormatID))
	if err != nil {
		return nil, err
	}

	// Write the sbom to disk using the image tag as the filename
	filename := fmt.Sprintf("%s.json", tag.String())
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
func (b *Builder) createFileSBOM(componentSBOM types.ComponentSBOM, component string) ([]byte, error) {
	catalog := pkg.NewCatalog()
	relationships := []artifact.Relationship{}
	parentSource, err := source.NewFromDirectory(componentSBOM.ComponentPath.Base)
	if err != nil {
		return nil, err
	}

	for _, file := range componentSBOM.Files {
		// Create the sbom source
		fileSource, clean := source.NewFromFile(file)
		defer clean()

		// Dogsled distro since this is not a linux image we are scanning
		cat, rel, _, err := syft.CatalogPackages(&fileSource, cataloger.DefaultConfig())
		if err != nil {
			return nil, err
		}

		for pkg := range cat.Enumerate() {
			catalog.Add(pkg)
		}

		for _, r := range rel {
			relationships = append(relationships, artifact.Relationship{
				From: &parentSource,
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
		Source: parentSource.Metadata,
		Artifacts: sbom.Artifacts{
			PackageCatalog:    catalog,
			LinuxDistribution: &linux.Release{},
		},
		Relationships: relationships,
	}

	jsonData, err := syft.Encode(artifact, syft.FormatByID(syft.JSONFormatID))
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
	path := filepath.Join(b.sbomPath, b.getNormalizedFileName(filename))
	return os.Create(path)
}
