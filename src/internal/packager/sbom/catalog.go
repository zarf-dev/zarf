// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom contains tools for generating SBOMs
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
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type Builder struct {
	spinner    *message.Spinner
	cachePath  string
	imagesPath string
	dir        string
	jsonList   []byte
}

//go:embed viewer/*
var viewerAssets embed.FS
var transformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

func Catalog(componentToFiles map[string][]string, tagToImage map[name.Tag]v1.Image, imagesPath, sbomDir string) {
	imageCount := len(tagToImage)
	componentCount := len(componentToFiles)
	builder := Builder{
		spinner:    message.NewProgressSpinner("Creating SBOMs for %d images and %d components with files.", imageCount, componentCount),
		cachePath:  config.GetAbsCachePath(),
		imagesPath: imagesPath,
		dir:        sbomDir,
	}
	defer builder.spinner.Stop()

	// Ensure the sbom directory exists
	_ = utils.CreateDirectory(builder.dir, 0700)

	// Generate a list of images and files for the sbom viewer
	if json, err := builder.generateJSONList(componentToFiles, tagToImage); err != nil {
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

	// Generate SBOM for each image
	for component := range componentToFiles {
		builder.spinner.Updatef("Creating component file SBOMs (%d of %d): %s", currComponent, componentCount, component)

		jsonData, err := builder.createFileSBOM(componentToFiles[component], component)
		if err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM for component %s", component)
		}

		if err = builder.createSBOMViewerAsset(fmt.Sprintf("zarf-component-%s", component), jsonData); err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM viewer for component %s", component)
		}

		currImage++
	}

	builder.spinner.Success()
}

// createImageSBOM uses syft to generate SBOM for an image,
// some code/structure migrated from https://github.com/testifysec/go-witness/blob/v0.1.12/attestation/syft/syft.go
func (builder *Builder) createImageSBOM(tag name.Tag) ([]byte, error) {
	// Get the image
	tarballImg, err := tarball.ImageFromPath(builder.imagesPath, &tag)
	if err != nil {
		return nil, err
	}

	// Create the sbom
	imageCachePath := filepath.Join(builder.cachePath, config.ZarfImageCacheDir)
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
	sbomFile, err := builder.createSBOMFile("%s.json", tag.String())
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

// createPathSBOM uses syft to generate SBOM for a filepath
func (builder *Builder) createFileSBOM(files []string, component string) ([]byte, error) {

	catalog := pkg.NewCatalog()
	relationships := []artifact.Relationship{}
	var distro *linux.Release
	var metadata source.Metadata

	for _, file := range files {
		// Create the sbom source
		syftSource, clean := source.NewFromFile(file)
		defer clean()

		cat, rel, dist, err := syft.CatalogPackages(&syftSource, cataloger.DefaultConfig())
		if err != nil {
			return nil, err
		}

		for pkg := range cat.Enumerate() {
			catalog.Add(pkg)
		}
		relationships = append(relationships, rel...)

		distro = dist

		metadata = syftSource.Metadata
	}

	artifact := sbom.SBOM{
		Descriptor: sbom.Descriptor{
			Name: "zarf",
		},
		Source: metadata,
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

	// Write the sbom to disk using the given name as the filename
	sbomFile, err := builder.createSBOMFile("%s.json", fmt.Sprintf("component-%s", component))
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

func (builder *Builder) getNormalizedFileName(identifier string) string {
	return transformRegex.ReplaceAllString(identifier, "_")
}

func (builder *Builder) createSBOMFile(name string, identifier string) (*os.File, error) {
	file := fmt.Sprintf(name, builder.getNormalizedFileName(identifier))
	path := filepath.Join(builder.dir, file)
	return os.Create(path)
}
