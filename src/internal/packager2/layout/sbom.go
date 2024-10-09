package layout

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
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

const componentPrefix = "zarf-component-"

//go:embed viewer/*
var viewerAssets embed.FS
var transformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

func GenerateSBOM(ctx context.Context, buildPath string, images []transform.Image) error {
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	cachePath, err := config.GetAbsCachePath()
	if err != nil {
		return err
	}

	jsonList, err := generateJSONList(nil, images)
	if err != nil {
		return err
	}

	for _, refInfo := range images {
		img, err := utils.LoadOCIImage(filepath.Join(buildPath, string(ImagesDir)), refInfo)
		if err != nil {
			return err
		}
		b, err := createImageSBOM(ctx, cachePath, tmpDir, img, refInfo.Reference)
		if err != nil {
			return err
		}
		err = createSBOMViewerAsset(tmpDir, refInfo.Reference, b, jsonList)
		if err != nil {
			return err
		}
	}

	// Generate SBOM for each component
	// for component := range componentSBOMs {
	// 	jsonData, err := builder.createFileSBOM(ctx, *componentSBOMs[component], component)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	if err = builder.createSBOMViewerAsset(fmt.Sprintf("%s%s", componentPrefix, component), jsonData); err != nil {
	// 		return err
	// 	}
	// }

	// Include the compare tool if there are any image SBOMs OR component SBOMs
	// if len(components) > 0 || len(images) > 0 {
	// 	if err := builder.createSBOMCompareAsset(); err != nil {
	// 		return err
	// 	}
	// }

	// if err := paths.SBOMs.Archive(); err != nil {
	// 	return err
	// }

	return nil
}

func createImageSBOM(ctx context.Context, cachePath, outputPath string, img v1.Image, src string) ([]byte, error) {
	imageCachePath := filepath.Join(cachePath, ImagesDir)
	err := os.MkdirAll(imageCachePath, helpers.ReadWriteExecuteUser)
	if err != nil {
		return nil, err
	}

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

func createFileSBOM(ctx context.Context, outputPath string, componentSBOM layout.ComponentSBOM, component string) ([]byte, error) {
	parentSource, err := directorysource.NewFromPath(componentSBOM.Component.Base)
	if err != nil {
		return nil, err
	}

	catalog := pkg.NewCollection()
	relationships := []artifact.Relationship{}
	for _, sbomFile := range componentSBOM.Files {
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

	filename := fmt.Sprintf("%s%s.json", componentPrefix, component)
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

func createSBOMHTML(outputDir string, filename string, goTemplate string, jsonData []byte, jsonList []byte) error {
	path := filepath.Join(outputDir, getNormalizedFileName(filename))
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create the sbomviewer template data
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
		ThemeCSS:  loadFileCSS("theme.css"),
		ViewerCSS: loadFileCSS("styles.css"),
		List:      template.JS(jsonList),
		Data:      template.JS(jsonData),
		LibraryJS: loadFileJS("library.js"),
		CommonJS:  loadFileJS("common.js"),
		ViewerJS:  loadFileJS("viewer.js"),
		CompareJS: loadFileJS("compare.js"),
	}

	// Render the sbomviewer template
	tpl, err := template.ParseFS(viewerAssets, goTemplate)
	if err != nil {
		return err
	}

	// Write the sbomviewer template to disk
	return tpl.Execute(file, tplData)
}

func loadFileCSS(name string) template.CSS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.CSS(data)
}

func loadFileJS(name string) template.JS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.JS(data)
}

func getNormalizedFileName(identifier string) string {
	return transformRegex.ReplaceAllString(identifier, "_")
}

func generateJSONList(componentToFiles map[string]*layout.ComponentSBOM, imageList []transform.Image) ([]byte, error) {
	var jsonList []string
	for _, refInfo := range imageList {
		normalized := getNormalizedFileName(refInfo.Reference)
		jsonList = append(jsonList, normalized)
	}
	for k := range componentToFiles {
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
