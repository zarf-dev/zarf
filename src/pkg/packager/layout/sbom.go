// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

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
	clayout "github.com/google/go-containerregistry/pkg/v1/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

const componentPrefix = "zarf-component-"

//go:embed viewer/*
var viewerAssets embed.FS
var transformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

func generateSBOM(ctx context.Context, pkg v1alpha1.ZarfPackage, buildPath string, images []transform.Image, cachePath string) (err error) {
	l := logger.From(ctx)
	outputPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(outputPath))
	}()

	componentSBOMs := []string{}
	for _, comp := range pkg.Components {
		if len(comp.Files) > 0 || len(comp.DataInjections) > 0 {
			componentSBOMs = append(componentSBOMs, comp.Name)
		}
	}

	type imageSBOMTarget struct {
		img        v1.Image
		identifier string
	}
	var targets []imageSBOMTarget
	for _, refInfo := range images {
		platformImages, err := loadOCIImagePlatforms(filepath.Join(buildPath, string(ImagesDir)), refInfo)
		if err != nil {
			return fmt.Errorf("failed to load OCI image: %w", err)
		}
		for _, pi := range platformImages {
			identifier := refInfo.Reference
			if pi.platform != nil && pi.platform.Architecture != "" {
				identifier = fmt.Sprintf("%s-%s-%s", refInfo.Reference, pi.platform.OS, pi.platform.Architecture)
				if pi.platform.Variant != "" {
					identifier = fmt.Sprintf("%s-%s", identifier, pi.platform.Variant)
				}
			}
			targets = append(targets, imageSBOMTarget{img: pi.image, identifier: identifier})
		}
	}

	identifiers := make([]string, 0, len(targets))
	for _, t := range targets {
		identifiers = append(identifiers, t.identifier)
	}
	jsonList, err := generateJSONList(componentSBOMs, identifiers)
	if err != nil {
		return err
	}

	for _, t := range targets {
		l.Info("creating image SBOM", "reference", t.identifier)
		b, err := createImageSBOM(ctx, cachePath, outputPath, t.img, t.identifier)
		if err != nil {
			return fmt.Errorf("failed to create image sbom: %w", err)
		}
		err = createSBOMViewerAsset(outputPath, t.identifier, b, jsonList)
		if err != nil {
			return err
		}
	}

	// Generate SBOM for each component
	for _, comp := range pkg.Components {
		if len(comp.DataInjections) == 0 && len(comp.Files) == 0 {
			continue
		}
		jsonData, err := createFileSBOM(ctx, comp, outputPath, buildPath)
		if err != nil {
			return err
		}
		err = createSBOMViewerAsset(outputPath, fmt.Sprintf("%s%s", componentPrefix, comp.Name), jsonData, jsonList)
		if err != nil {
			return err
		}
	}

	// Include the compare tool if there are any image SBOMs OR component SBOMs
	err = createSBOMCompareAsset(outputPath)
	if err != nil {
		return err
	}

	err = createReproducibleTarballFromDir(outputPath, "", filepath.Join(buildPath, "sboms.tar"), false)
	if err != nil {
		return err
	}

	return nil
}

func createImageSBOM(ctx context.Context, cachePath, outputPath string, img v1.Image, identifier string) ([]byte, error) {
	imageCachePath := filepath.Join(cachePath, ImagesDir)

	// This is a write cache
	if err := helpers.CreateDirectory(imageCachePath, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image cache directory %s: %w", imageCachePath, err)
	}

	syftImage := image.New(img, file.NewTempDirGenerator("zarf"), imageCachePath, image.WithTags(identifier))
	err := syftImage.Read()
	if err != nil {
		return nil, err
	}
	cfg := getDefaultSyftConfig()
	syftSrc := stereoscopesource.New(syftImage, stereoscopesource.ImageConfig{
		Reference: identifier,
	})
	sbom, err := syft.CreateSBOM(ctx, syftSrc, cfg)
	if err != nil {
		return nil, err
	}
	jsonData, err := format.Encode(*sbom, syftjson.NewFormatEncoder())
	if err != nil {
		return nil, err
	}

	normalizedName := getNormalizedFileName(fmt.Sprintf("%s.json", identifier))
	path := filepath.Join(outputPath, normalizedName)
	err = os.WriteFile(path, jsonData, 0o666)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func createFileSBOM(ctx context.Context, component v1alpha1.ZarfComponent, outputPath, buildPath string) (_ []byte, err error) {
	l := logger.From(ctx)
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()
	tarPath := filepath.Join(buildPath, ComponentsDir, component.Name) + ".tar"
	err = archive.Decompress(ctx, tarPath, tmpDir, archive.DecompressOpts{})
	if err != nil {
		return nil, err
	}
	sbomFiles := []string{}
	appendSBOMFiles := func(path string) error {
		if helpers.IsDir(path) {
			files, err := helpers.RecursiveFileList(path, nil, false)
			if err != nil {
				return err
			}
			sbomFiles = append(sbomFiles, files...)
		} else {
			sbomFiles = append(sbomFiles, path)
		}
		return nil
	}
	for i, file := range component.Files {
		path := filepath.Join(tmpDir, component.Name, string(FilesComponentDir), strconv.Itoa(i), filepath.Base(file.Target))
		err := appendSBOMFiles(path)
		if err != nil {
			return nil, err
		}
	}
	for i, data := range component.DataInjections {
		path := filepath.Join(tmpDir, component.Name, string(DataComponentDir), strconv.Itoa(i), filepath.Base(data.Target.Path))
		err := appendSBOMFiles(path)
		if err != nil {
			return nil, err
		}
	}

	parentSource, err := directorysource.NewFromPath(tmpDir)
	if err != nil {
		return nil, err
	}
	catalog := pkg.NewCollection()
	relationships := []artifact.Relationship{}
	for _, sbomFile := range sbomFiles {
		l.Info("creating file SBOMs", "file", sbomFile)
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
			fileMetadata, ok := fileSrc.Describe().Metadata.(source.FileMetadata)
			if !ok {
				return nil, errors.New("failed to get file metadata from SBOM source")
			}

			// See if the source locations for this package contain the file Zarf indexed
			for _, location := range pkg.Locations.ToSlice() {
				if location.RealPath == fileMetadata.Path {
					containsSource = true
				}
			}

			// If the locations do not contain the source file (i.e. the package was inside a tarball), add the file source
			if !containsSource {
				sourceLocation := syftFile.NewLocation(fileMetadata.Path)
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

	filename := fmt.Sprintf("%s%s.json", componentPrefix, component.Name)
	path := filepath.Join(outputPath, getNormalizedFileName(filename))
	err = os.WriteFile(path, jsonData, 0o666)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func createSBOMViewerAsset(outputDir, identifier string, jsonData, jsonList []byte) error {
	filename := fmt.Sprintf("sbom-viewer-%s.html", getNormalizedFileName(identifier))
	return createSBOMHTML(outputDir, filename, "viewer/template.gohtml", jsonData, jsonList)
}

func createSBOMCompareAsset(outputDir string) error {
	return createSBOMHTML(outputDir, "compare.html", "viewer/compare.gohtml", nil, nil)
}

func createSBOMHTML(outputDir, filename, goTemplate string, jsonData, jsonList []byte) error {
	path := filepath.Join(outputDir, getNormalizedFileName(filename))
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()
	themeCSS, err := loadFileCSS("theme.css")
	if err != nil {
		return err
	}
	viewerCSS, err := loadFileCSS("styles.css")
	if err != nil {
		return err
	}
	libraryJS, err := loadFileJS("library.js")
	if err != nil {
		return err
	}
	commonJS, err := loadFileJS("common.js")
	if err != nil {
		return err
	}
	viewerJS, err := loadFileJS("viewer.js")
	if err != nil {
		return err
	}
	compareJS, err := loadFileJS("compare.js")
	if err != nil {
		return err
	}
	tplData := struct {
		ThemeCSS  template.CSS
		ViewerCSS template.CSS
		List      template.JS
		Data      template.JS
		LibraryJS template.JS
		CommonJS  template.JS
		ViewerJS  template.JS
		CompareJS template.JS
	}{
		ThemeCSS:  themeCSS,
		ViewerCSS: viewerCSS,
		List:      template.JS(jsonList),
		Data:      template.JS(jsonData),
		LibraryJS: libraryJS,
		CommonJS:  commonJS,
		ViewerJS:  viewerJS,
		CompareJS: compareJS,
	}
	tpl, err := template.ParseFS(viewerAssets, goTemplate)
	if err != nil {
		return err
	}
	return tpl.Execute(file, tplData)
}

func loadFileCSS(name string) (template.CSS, error) {
	data, err := viewerAssets.ReadFile("viewer/" + name)
	if err != nil {
		return "", err
	}
	return template.CSS(data), nil
}

func loadFileJS(name string) (template.JS, error) {
	data, err := viewerAssets.ReadFile("viewer/" + name)
	if err != nil {
		return "", err
	}
	return template.JS(data), nil
}

func getNormalizedFileName(identifier string) string {
	return transformRegex.ReplaceAllString(identifier, "_")
}

func generateJSONList(components []string, imageIdentifiers []string) ([]byte, error) {
	var jsonList []string
	for _, id := range imageIdentifiers {
		jsonList = append(jsonList, getNormalizedFileName(id))
	}
	for _, k := range components {
		normalized := getNormalizedFileName(fmt.Sprintf("%s%s", componentPrefix, k))
		jsonList = append(jsonList, normalized)
	}
	return json.Marshal(jsonList)
}

func getDefaultSyftConfig() *syft.CreateSBOMConfig {
	cfg := syft.DefaultCreateSBOMConfig()
	cfg.ToolName = "zarf"
	cfg.ToolVersion = config.CLIVersion
	return cfg
}

// platformImage pairs a loaded image with the platform it targets.
// platform is nil for images stored as a single-platform manifest.
type platformImage struct {
	image    v1.Image
	platform *v1.Platform
}

// loadOCIImagePlatforms returns the v1.Images for refInfo. Single-platform images return one entry
// with a nil platform; multi-arch indexes return one entry per platform manifest.
// Non container images (e.g. Helm charts) are skipped — returning an empty slice is not an error.
func loadOCIImagePlatforms(imgPath string, refInfo transform.Image) ([]platformImage, error) {
	layoutPath := clayout.Path(imgPath)
	imgIdx, err := layoutPath.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get image index: %w", err)
	}
	idxManifest, err := imgIdx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image manifest: %w", err)
	}

	for _, manifest := range idxManifest.Manifests {
		if manifest.Annotations[ocispec.AnnotationRefName] != refInfo.Reference {
			continue
		}

		if images.IsIndex(string(manifest.MediaType)) {
			return collectPlatformImagesFromIndex(imgIdx, manifest.Digest, refInfo.Reference)
		}

		img, err := layoutPath.Image(manifest.Digest)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup image %s: %w", refInfo.Reference, err)
		}
		isContainer, err := imageHasOnlyContainerLayers(img)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect manifest for %s: %w", refInfo.Reference, err)
		}
		if !isContainer {
			return nil, nil
		}
		return []platformImage{{image: img}}, nil
	}

	return nil, fmt.Errorf("unable to find image (%s) at the path (%s)", refInfo.Reference, imgPath)
}

func imageHasOnlyContainerLayers(img v1.Image) (bool, error) {
	raw, err := img.RawManifest()
	if err != nil {
		return false, err
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return false, err
	}
	return images.OnlyHasImageLayers(manifest), nil
}

func collectPlatformImagesFromIndex(parent v1.ImageIndex, indexDigest v1.Hash, ref string) ([]platformImage, error) {
	idx, err := parent.ImageIndex(indexDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to load image index for %s: %w", ref, err)
	}
	manifest, err := idx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to parse image index manifest for %s: %w", ref, err)
	}
	var platformImages []platformImage
	for _, child := range manifest.Manifests {
		switch {
		case images.IsIndex(string(child.MediaType)):
			nested, err := collectPlatformImagesFromIndex(idx, child.Digest, ref)
			if err != nil {
				return nil, err
			}
			platformImages = append(platformImages, nested...)
		case images.IsManifest(string(child.MediaType)):
			img, err := idx.Image(child.Digest)
			if err != nil {
				return nil, fmt.Errorf("failed to lookup platform image for %s: %w", ref, err)
			}
			rawManifest, err := img.RawManifest()
			if err != nil {
				return nil, fmt.Errorf("failed to read platform manifest for %s: %w", ref, err)
			}
			var childManifest ocispec.Manifest
			if err := json.Unmarshal(rawManifest, &childManifest); err != nil {
				return nil, fmt.Errorf("failed to parse platform manifest for %s: %w", ref, err)
			}
			if !images.OnlyHasImageLayers(childManifest) {
				continue
			}
			platformImages = append(platformImages, platformImage{
				image:    img,
				platform: child.Platform,
			})
		}
	}
	return platformImages, nil
}
