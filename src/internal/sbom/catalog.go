package sbom

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/pkg/cataloger"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type Builder struct {
	spinner       *message.Spinner
	cachePath     string
	tarPath       string
	dir           string
	jsonImageList []byte
}

//go:embed viewer/*
var viewerAssets embed.FS
var transformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

func CatalogImages(tagToImage map[name.Tag]v1.Image, sbomDir, tarPath string) {
	// Ignore SBOM creation if there the flag is set
	if config.CreateOptions.SkipSBOM {
		message.Debug("Skipping SBOM processing per --skip-sbom flag")
		return
	}

	imageCount := len(tagToImage)
	builder := Builder{
		spinner:   message.NewProgressSpinner("Creating SBOMs for %d images.", imageCount),
		cachePath: config.GetCachePath(),
		tarPath:   tarPath,
		dir:       sbomDir,
	}
	defer builder.spinner.Stop()

	// Ensure the sbom directory exists
	_ = utils.CreateDirectory(builder.dir, 0700)

	currImage := 1

	// Generate a list of images for the sbom viewer
	if json, err := builder.generateImageListJSON(tagToImage); err != nil {
		builder.spinner.Fatalf(err, "Unable to generate the SBOM image list")
	} else {
		builder.jsonImageList = json
	}

	// Generate SBOM for each image
	for tag := range tagToImage {
		builder.spinner.Updatef("Creating image SBOMs (%d of %d): %s", currImage, imageCount, tag)

		jsonData, err := builder.createImageSBOM(tag)
		if err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM for image %s", tag)
		}

		if err = builder.createSBOMViewerAsset(tag, jsonData); err != nil {
			builder.spinner.Fatalf(err, "Unable to create SBOM viewer for image %s", tag)
		}

		currImage++
	}

	builder.spinner.Success()
}

// uses syft to generate SBOM for an image,
// some code/structure migrated from https://github.com/testifysec/go-witness/blob/v0.1.12/attestation/syft/syft.go
func (builder *Builder) createImageSBOM(tag name.Tag) ([]byte, error) {
	// Get the image
	tarballImg, err := tarball.ImageFromPath(builder.tarPath, &tag)
	if err != nil {
		return nil, err
	}

	// Create the sbom
	imageCachePath := filepath.Join(builder.cachePath, "images")
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
	sbomFile, err := builder.createSBOMFile("%s.json", tag)
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

func (builder *Builder) getNormalizedTag(tag name.Tag) string {
	return transformRegex.ReplaceAllString(tag.String(), "_")
}

func (builder *Builder) createSBOMFile(name string, tag name.Tag) (*os.File, error) {
	file := fmt.Sprintf(name, builder.getNormalizedTag(tag))
	path := filepath.Join(builder.dir, file)
	return os.Create(path)
}
