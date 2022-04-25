package sbom

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anchore/syft/syft/pkg"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/testifysec/witness/pkg/attestation"
	"github.com/testifysec/witness/pkg/attestation/syft"
)

type SimplifiedSBOM struct {
	Source    any
	Distro    any
	Artifacts []SimplifiedSBOMArtifact
}
type SimplifiedSBOMArtifact struct {
	Type     string
	Name     string
	Version  string
	Metadata any
}

// artifact.type,
// artifact.name,
// artifact.version,
// fileList(artifact.metadata),
// artifact.metadata.description || '-',
// (artifact.metadata.maintainer || '-').replace(/\u003c(.*)\u003e/, '&nbsp;|&nbsp;&nbsp;<a href="mailto:$1">$1</a>'),
// artifact.metadata.installedSize || '-',

func CatalogImages(tagToImage map[name.Tag]v1.Image, sbomDir, tarPath string) {
	imageCount := len(tagToImage)
	spinner := message.NewProgressSpinner("Creating SBOMs for %d images.", imageCount)
	defer spinner.Stop()

	actx, err := attestation.NewContext([]attestation.Attestor{})
	if err != nil {
		spinner.Fatalf(err, "Unable to make attestation context")
	}

	cachePath := config.GetImageCachePath()
	viewerSBOM := make(map[string]SimplifiedSBOM)
	currImage := 1

	for tag := range tagToImage {
		spinner.Updatef("Creating image SBOMs (%d of %d): %s", currImage, imageCount, tag)
		tarballImg, err := tarball.ImageFromPath(tarPath, &tag)
		if err != nil {
			spinner.Fatalf(err, "Unable to open image %s", tag.String())
		}

		sbomAttestor := syft.New(syft.WithImageSource(tarballImg, cachePath, tag.String()))
		if err := sbomAttestor.Attest(actx); err != nil {
			spinner.Fatalf(err, "Unable to build sbom for image %s", tag.String())
		}

		sbomFile, err := os.Create(filepath.Join(sbomDir, fmt.Sprintf("%s.json", sbomAttestor.SBOM.Source.ImageMetadata.ID)))
		if err != nil {
			spinner.Fatalf(err, "Unable to create SBOM file for image %s", tag.String())
		}

		defer sbomFile.Close()
		enc := json.NewEncoder(sbomFile)
		if err := enc.Encode(sbomAttestor); err != nil {
			spinner.Fatalf(err, "Unable to write SBOM file for image %s", tag.String())
		}

		catalog := sbomAttestor.SBOM.Artifacts.PackageCatalog
		sorted := catalog.Sorted()

		var imageSBOM SimplifiedSBOM

		for _, artifact := range sorted {

			switch artifact.MetadataType {
			case pkg.ApkMetadataType:
				metadata, ok := artifact.Metadata.(pkg.ApkMetadata)
				if !ok {
					message.Debug("Unable to cast metadata to apk metadata")
					continue
				}
				message.Debug(metadata)

			}

			imageSBOM.Artifacts = append(imageSBOM.Artifacts, SimplifiedSBOMArtifact{
				Type:    string(artifact.Type),
				Name:    artifact.Name,
				Version: artifact.Version,
				// Metadata: metadata.data,
			})
		}

		viewerSBOM[tag.Name()] = imageSBOM

		message.Debug(sorted)

		currImage++
	}
	message.Debug(viewerSBOM)

	spinner.Success()
}
