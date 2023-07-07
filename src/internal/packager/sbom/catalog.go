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

	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/logrus"
	"github.com/anchore/stereoscope"
	"github.com/anchore/stereoscope/pkg/file"
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
	"github.com/mholt/archiver/v3"
	"github.com/wagoodman/go-partybus"
)

// Builder is the main struct used to build SBOM artifacts.
type Builder struct {
	spinner     *message.Spinner
	cachePath   string
	imagesPath  string
	tmpSBOMPath string
	sbomTarPath string
	jsonList    []byte
}

//go:embed viewer/*
var viewerAssets embed.FS
var transformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

var componentPrefix = "zarf-component-"

// Catalog catalogs the given components and images to create an SBOM.
// func Catalog(componentSBOMs map[string]*types.ComponentSBOM, imgList []string, imagesPath, sbomPath string) error {
func Catalog(componentSBOMs map[string]*types.ComponentSBOM, imgList []string, tmpPaths types.TempPaths) error {
	imageCount := len(imgList)
	componentCount := len(componentSBOMs)
	builder := Builder{
		spinner:     message.NewProgressSpinner("Creating SBOMs for %d images and %d components with files.", imageCount, componentCount),
		cachePath:   config.GetAbsCachePath(),
		imagesPath:  tmpPaths.Images,
		sbomTarPath: tmpPaths.SbomTar,
		tmpSBOMPath: tmpPaths.Sboms,
	}
	defer builder.spinner.Stop()

	// Ensure the sbom directory exists
	_ = utils.CreateDirectory(builder.tmpSBOMPath, 0700)

	// Generate a list of images and files for the sbom viewer
	json, err := builder.generateJSONList(componentSBOMs, imgList)
	if err != nil {
		builder.spinner.Errorf(err, "Unable to generate the SBOM image list")
		return err
	}
	builder.jsonList = json

	eventBus := partybus.NewBus()
	stereoscope.SetBus(eventBus)
	syft.SetBus(eventBus)
	subscription := eventBus.Subscribe()
	defer eventBus.Unsubscribe(subscription)

	cfg := logrus.Config{
		Level:         logger.DebugLevel,
		EnableConsole: true,
	}
	log, _ := logrus.New(cfg)
	syft.SetLogger(log)
	stereoscope.SetLogger(log.Nested("steroscope"))

	events := subscription.Events()
	go func() {
		for event := range events {
			message.Debug("syft||stereoscope", message.JSONValue(event))
		}
	}()

	if len(imgList) > 0 {
		// Generate SBOM for all images at once using goroutines
		// Use the ConcurrencyTools part of the utils package to help with concurrency
		imageSBOMConcurrency := utils.NewConcurrencyTools[string, message.ErrorWithMessage](len(imgList))

		// Make sure cancel is always called
		defer imageSBOMConcurrency.Cancel()

		// Call a goroutine for each image
		for _, tag := range imgList {
			currentTag := tag
			go func() {
				// Make sure to call Done() on the WaitGroup when the goroutine finishes
				defer imageSBOMConcurrency.WaitGroupDone()
				// Get the image that we are creating an SBOM for
				img, err := utils.LoadOCIImage(tmpPaths.Images, currentTag)
				if err != nil {
					imageSBOMConcurrency.ErrorChan <- message.ErrorWithMessage{Error: err, Message: "Unable to load the image to generate an SBOM"}
					return
				}

				// If the context has been cancelled end the goroutine
				if imageSBOMConcurrency.IsDone() {
					return
				}

				// Generate the SBOM JSON for the given image
				jsonData, err := builder.createImageSBOM(img, currentTag)
				if err != nil {
					imageSBOMConcurrency.ErrorChan <- message.ErrorWithMessage{Error: err, Message: fmt.Sprintf("Unable to create SBOM for image %s", currentTag)}
					return
				}

				// If the context has been cancelled end the goroutine
				if imageSBOMConcurrency.IsDone() {
					return
				}

				// Create the SBOM viewer HTML for the given image
				if err = builder.createSBOMViewerAsset(currentTag, jsonData); err != nil {
					imageSBOMConcurrency.ErrorChan <- message.ErrorWithMessage{Error: err, Message: fmt.Sprintf("Unable to create SBOM viewer for image %s", currentTag)}
					return
				}

				// If the context has been cancelled end the goroutine
				if imageSBOMConcurrency.IsDone() {
					return
				}

				// Call the progress channel to let us know that the SBOM generation is done for this image
				imageSBOMConcurrency.ProgressChan <- currentTag
			}()
		}

		onImageSBOMProgress := func(tag string, i int) {
			builder.spinner.Updatef("Creating image SBOMs (%d of %d): %s", i, len(imgList), tag)
		}

		onImageSBOMError := func(erroredImage message.ErrorWithMessage) error {
			builder.spinner.Errorf(erroredImage.Error, erroredImage.Message)
			return erroredImage.Error
		}

		err = imageSBOMConcurrency.WaitWithProgress(onImageSBOMProgress, onImageSBOMError)
		if err != nil {
			return err
		}
	}

	// Generate SBOM for all components' files/dataInjections at once using goroutines

	if len(componentSBOMs) > 0 {
		builder.spinner.Updatef("Creating component file SBOMs (0 of %d)", len(componentSBOMs))

		// Use the ConcurrencyTools part of the utils package to help with concurrency
		fileSBOMConcurrency := utils.NewConcurrencyTools[string, message.ErrorWithMessage](len(componentSBOMs))

		for component := range componentSBOMs {
			currentComponent := component
			go func() {
				// Make sure to call Done() on the WaitGroup when the goroutine finishes
				defer fileSBOMConcurrency.WaitGroupDone()

				// Check if component requires SBOM generation
				if componentSBOMs[currentComponent] == nil {
					message.Debugf("Component %s has invalid SBOM, skipping", currentComponent)
					return
				}

				// If the context has been cancelled end the goroutine
				if fileSBOMConcurrency.IsDone() {
					return
				}

				// Generate the SBOM JSON for the given component
				jsonData, err := builder.createFileSBOM(*componentSBOMs[currentComponent], currentComponent)
				if err != nil {
					fileSBOMConcurrency.ErrorChan <- message.ErrorWithMessage{Error: err, Message: fmt.Sprintf("Unable to create SBOM for component %s", currentComponent)}
					return
				}

				// If the context has been cancelled end the goroutine
				if fileSBOMConcurrency.IsDone() {
					return
				}

				// Create the SBOM viewer HTML for the given component
				if err = builder.createSBOMViewerAsset(fmt.Sprintf("%s%s", componentPrefix, currentComponent), jsonData); err != nil {
					fileSBOMConcurrency.ErrorChan <- message.ErrorWithMessage{Error: err, Message: fmt.Sprintf("Unable to create SBOM viewer for component %s", currentComponent)}
					return
				}

				// If the context has been cancelled end the goroutine
				if fileSBOMConcurrency.IsDone() {
					return
				}

				// Call the progress channel to let us know that the SBOM generation is done for this component
				fileSBOMConcurrency.ProgressChan <- currentComponent
			}()
		}

		onFileSBOMProgress := func(component string, i int) {
			builder.spinner.Updatef("Creating component file SBOMs (%d of %d): %s", i, len(componentSBOMs), component)
		}

		onFileSBOMError := func(erroredComponent message.ErrorWithMessage) error {
			builder.spinner.Errorf(erroredComponent.Error, erroredComponent.Message)
			return erroredComponent.Error
		}

		if err := fileSBOMConcurrency.WaitWithProgress(onFileSBOMProgress, onFileSBOMError); err != nil {
			return err
		}
	}

	// Include the compare tool if there are any image SBOMs OR component SBOMs
	if len(componentSBOMs) > 0 || len(imgList) > 0 {
		if err := builder.createSBOMCompareAsset(); err != nil {
			builder.spinner.Errorf(err, "Unable to create SBOM compare tool")
			return err
		}
	}

	allSBOMFiles, err := filepath.Glob(filepath.Join(builder.tmpSBOMPath, "*"))
	if err != nil {
		builder.spinner.Errorf(err, "Unable to get a list of all SBOM files")
		return err
	}

	if err = archiver.Archive(allSBOMFiles, builder.sbomTarPath); err != nil {
		builder.spinner.Errorf(err, "Unable to create the sbom archive")
		return err
	}

	if err = os.RemoveAll(builder.tmpSBOMPath); err != nil {
		builder.spinner.Errorf(err, "Unable to remove the temporary SBOM directory")
		return err
	}

	builder.spinner.Success()

	return nil
}

// createImageSBOM uses syft to generate SBOM for an image,
// some code/structure migrated from https://github.com/testifysec/go-witness/blob/v0.1.12/attestation/syft/syft.go.
func (b *Builder) createImageSBOM(img v1.Image, tagStr string) ([]byte, error) {
	// Get the image reference.
	tag, err := name.NewTag(tagStr, name.WeakValidation)
	if err != nil {
		return nil, err
	}

	// Create the sbom.
	imageCachePath := filepath.Join(b.cachePath, config.ZarfImageCacheDir)

	// Ensure the image cache directory exists.
	if err := utils.CreateDirectory(imageCachePath, 0700); err != nil {
		return nil, err
	}

	tmp := file.NewTempDirGenerator("zarf")
	defer tmp.Cleanup()
	syftImage := image.New(img, tmp, imageCachePath, image.WithTags(tag.String()))
	// defer syftImage.Cleanup()
	if err := syftImage.Read(); err != nil {
		return nil, err
	}

	syftSource, err := source.NewFromImage(syftImage, "")
	// defer syftSource.Image.Cleanup()
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
			Packages:          catalog,
			LinuxDistribution: distro,
		},
		Relationships: relationships,
	}

	jsonData, err := syft.Encode(artifact, syft.FormatByID(syft.JSONFormatID))
	if err != nil {
		return nil, err
	}

	// Write the sbom to disk using the image tag as the filename
	filename := fmt.Sprintf("%s.json", tag)
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
	catalog := pkg.NewCollection()
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
			containsSource := false

			// See if the source locations for this package contain the file Zarf indexed
			for _, location := range pkg.Locations.ToSlice() {
				if location.RealPath == fileSource.Metadata.Path {
					containsSource = true
				}
			}

			// If the locations do not contain the source file (i.e. the package was inside a tarball), add the file source
			if !containsSource {
				sourceLocation := source.NewLocation(fileSource.Metadata.Path)
				pkg.Locations.Add(sourceLocation)
			}

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
			Packages:          catalog,
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
	path := filepath.Join(b.tmpSBOMPath, b.getNormalizedFileName(filename))
	return os.Create(path)
}
