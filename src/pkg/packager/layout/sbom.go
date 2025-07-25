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

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/archive"
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
	jsonList, err := generateJSONList(componentSBOMs, images)
	if err != nil {
		return err
	}

	for _, refInfo := range images {
		img, err := utils.LoadOCIImage(filepath.Join(buildPath, string(ImagesDir)), refInfo)
		if err != nil {
			return fmt.Errorf("failed to load OCI image: %w", err)
		}
		l.Info("creating image SBOM", "reference", refInfo.Reference)
		b, err := createImageSBOM(ctx, cachePath, outputPath, img, refInfo.Reference)
		if err != nil {
			return fmt.Errorf("failed to create image sbom: %w", err)
		}
		err = createSBOMViewerAsset(outputPath, refInfo.Reference, b, jsonList)
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

func createImageSBOM(ctx context.Context, cachePath, outputPath string, img v1.Image, src string) ([]byte, error) {
	imageCachePath := filepath.Join(cachePath, ImagesDir)

	refInfo, err := transform.ParseImageRef(src)
	if err != nil {
		return nil, fmt.Errorf("failed to create ref for image %s: %w", src, err)
	}
	syftImage := image.NewImage(img, file.NewTempDirGenerator("zarf"), imageCachePath, image.WithTags(refInfo.Reference))
	err = syftImage.Read()
	if err != nil {
		return nil, err
	}
	cfg := getDefaultSyftConfig()
	syftSrc := stereoscopesource.New(syftImage, stereoscopesource.ImageConfig{
		Reference: refInfo.Reference,
	})
	sbom, err := syft.CreateSBOM(ctx, syftSrc, cfg)
	if err != nil {
		return nil, err
	}
	jsonData, err := format.Encode(*sbom, syftjson.NewFormatEncoder())
	if err != nil {
		return nil, err
	}

	normalizedName := getNormalizedFileName(fmt.Sprintf("%s.json", refInfo.Reference))
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

func generateJSONList(components []string, imageList []transform.Image) ([]byte, error) {
	var jsonList []string
	for _, refInfo := range imageList {
		normalized := getNormalizedFileName(refInfo.Reference)
		jsonList = append(jsonList, normalized)
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
