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

	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/artifact"
	syftFile "github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/format"
	"github.com/anchore/syft/syft/format/syftjson"
	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/anchore/syft/syft/source/directorysource"
	"github.com/anchore/syft/syft/source/filesource"
	"github.com/anchore/syft/syft/source/stereoscopesource"
	"github.com/defenseunicorns/pkg/helpers/v2"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
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
	l := logger.From(ctx)
	imageCount := len(imageList)
	componentCount := len(componentSBOMs)
	cachePath, err := config.GetAbsCachePath()
	if err != nil {
		return err
	}
	builder := Builder{
		// TODO(mkcp): Remove message on logger release
		spinner:    message.NewProgressSpinner("Creating SBOMs for %d images and %d components with files.", imageCount, componentCount),
		cachePath:  cachePath,
		imagesPath: paths.Images.Base,
		outputDir:  paths.SBOMs.Path,
	}
	defer builder.spinner.Stop()

	// Ensure the sbom directory exists
	_ = helpers.CreateDirectory(builder.outputDir, helpers.ReadWriteExecuteUser)

	// Generate a list of images and files for the sbom viewer
	json, err := builder.generateJSONList(componentSBOMs, imageList)
	if err != nil {
		// TODO(mkcp): Remove message on logger release
		builder.spinner.Errorf(err, "Unable to generate the SBOM image list")
		return fmt.Errorf("unable to generate the SBOM image list: %w", err)
	}
	builder.jsonList = json

	// Generate SBOM for each image
	currImage := 1
	l.Info("creating SBOMs for images", "count", imageCount)
	for _, refInfo := range imageList {
		// TODO(mkcp): Remove message on logger release
		builder.spinner.Updatef("Creating image SBOMs (%d of %d): %s", currImage, imageCount, refInfo.Reference)
		l.Info("creating image SBOMs", "reference", refInfo.Reference)

		// Get the image that we are creating an SBOM for
		img, err := utils.LoadOCIImage(paths.Images.Base, refInfo)
		if err != nil {
			// TODO(mkcp): Remove message on logger release
			builder.spinner.Errorf(err, "Unable to load the image to generate an SBOM")
			return fmt.Errorf("unable to load the image to generate an SBOM: %w", err)
		}

		jsonData, err := builder.createImageSBOM(ctx, img, refInfo.Reference)
		if err != nil {
			// TODO(mkcp): Remove message on logger release
			builder.spinner.Errorf(err, "Unable to create SBOM for image %s", refInfo.Reference)
			return fmt.Errorf("unable to create SBOM for image=%s: %w", refInfo.Reference, err)
		}

		if err = builder.createSBOMViewerAsset(refInfo.Reference, jsonData); err != nil {
			// TODO(mkcp): Remove message on logger release
			builder.spinner.Errorf(err, "Unable to create SBOM viewer for image %s", refInfo.Reference)
			return fmt.Errorf("unable to create SBOM viewer for image=%s: %w", refInfo.Reference, err)
		}

		currImage++
	}

	currComponent := 1

	// Generate SBOM for each component
	l.Info("creating SBOMs for components", "count", componentCount)
	for component := range componentSBOMs {
		// TODO(mkcp): Remove message on logger release
		builder.spinner.Updatef("Creating component file SBOMs (%d of %d): %s", currComponent, componentCount, component)
		l.Info("creating component file SBOMs", "component", component)

		if componentSBOMs[component] == nil {
			// TODO(mkcp): Remove message on logger release
			message.Debugf("Component %s has invalid SBOM, skipping", component)
			l.Debug("component has invalid SBOM, skipping", "component", component)
			continue
		}

		jsonData, err := builder.createFileSBOM(ctx, *componentSBOMs[component], component)
		if err != nil {
			// TODO(mkcp): Remove message on logger release
			builder.spinner.Errorf(err, "Unable to create SBOM for component %s", component)
			return fmt.Errorf("unable to create SBOM for component=%s: %w", component, err)
		}

		if err = builder.createSBOMViewerAsset(fmt.Sprintf("%s%s", componentPrefix, component), jsonData); err != nil {
			// TODO(mkcp): Remove message on logger release
			builder.spinner.Errorf(err, "Unable to create SBOM viewer for component %s", component)
			return fmt.Errorf("unable to create SBOM for component=%s: %w", component, err)
		}

		currComponent++
	}

	// Include the compare tool if there are any image SBOMs OR component SBOMs
	if len(componentSBOMs) > 0 || len(imageList) > 0 {
		if err := builder.createSBOMCompareAsset(); err != nil {
			// TODO(mkcp): Remove message on logger release
			builder.spinner.Errorf(err, "Unable to create SBOM compare tool")
			return fmt.Errorf("unable to create SBOM compare tool: %w", err)
		}
	}

	if err := paths.SBOMs.Archive(); err != nil {
		// TODO(mkcp): Remove message on logger release
		builder.spinner.Errorf(err, "Unable to archive SBOMs")
		return fmt.Errorf("unable to archive SBOMs: %w", err)
	}

	// TODO(mkcp): Remove message on logger release
	builder.spinner.Success()

	l.Debug("done building catalog")
	return nil
}

// createImageSBOM uses syft to generate SBOM for an image,
// some code/structure migrated from https://github.com/testifysec/go-witness/blob/v0.1.12/attestation/syft/syft.go.
func (b *Builder) createImageSBOM(ctx context.Context, img v1.Image, src string) ([]byte, error) {
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

	syftSrc := stereoscopesource.New(syftImage, stereoscopesource.ImageConfig{
		Reference: refInfo.Reference,
	})

	cfg := getDefaultSyftConfig()
	sbom, err := syft.CreateSBOM(ctx, syftSrc, cfg)
	if err != nil {
		return nil, err
	}

	jsonData, err := format.Encode(*sbom, syftjson.NewFormatEncoder())
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
func (b *Builder) createFileSBOM(ctx context.Context, componentSBOM layout.ComponentSBOM, component string) ([]byte, error) {
	catalog := pkg.NewCollection()
	relationships := []artifact.Relationship{}
	parentSource, err := directorysource.NewFromPath(componentSBOM.Component.Base)
	if err != nil {
		return nil, err
	}

	for _, sbomFile := range componentSBOM.Files {
		// Create the sbom source
		fileSrc, err := filesource.NewFromPath(sbomFile)
		if err != nil {
			return nil, err
		}

		cfg := getDefaultSyftConfig()
		sbom, err := syft.CreateSBOM(ctx, fileSrc, cfg)
		if err != nil {
			return nil, err
		}

		for pkg := range sbom.Artifacts.Packages.Enumerate() {
			containsSource := false

			// See if the source locations for this package contain the file Zarf indexed
			for _, location := range pkg.Locations.ToSlice() {
				if location.RealPath == fileSrc.Describe().Metadata.(source.FileMetadata).Path {
					containsSource = true
				}
			}

			// If the locations do not contain the source file (i.e. the package was inside a tarball), add the file source
			if !containsSource {
				sourceLocation := syftFile.NewLocation(fileSrc.Describe().Metadata.(source.FileMetadata).Path)
				pkg.Locations.Add(sourceLocation)
			}

			catalog.Add(pkg)
		}

		for _, r := range sbom.Relationships {
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
			Name:    "zarf",
			Version: config.CLIVersion,
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

func getDefaultSyftConfig() *syft.CreateSBOMConfig {
	cfg := syft.DefaultCreateSBOMConfig()
	cfg.ToolName = "zarf"
	cfg.ToolVersion = config.CLIVersion
	return cfg
}
