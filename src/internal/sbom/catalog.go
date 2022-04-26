package sbom

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/testifysec/witness/pkg/attestation"
	"github.com/testifysec/witness/pkg/attestation/syft"
)

//go:embed viewer/*
var viewerAssets embed.FS

const JS_TEMPLATE = `
ZARF_SBOM_IMAGE_LIST = [];
ZARF_SBOM_DATA = 
`

var tranformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

func CatalogImages(tagToImage map[name.Tag]v1.Image, sbomDir, tarPath string) {
	imageCount := len(tagToImage)
	spinner := message.NewProgressSpinner("Creating SBOMs for %d images.", imageCount)
	defer spinner.Stop()

	actx, err := attestation.NewContext([]attestation.Attestor{})
	if err != nil {
		spinner.Fatalf(err, "Unable to make attestation context")
	}

	cachePath := config.GetImageCachePath()
	currImage := 1

	jsonImageList, err := generateImageListJSON(tagToImage)
	if err != nil {
		spinner.Fatalf(err, "Unable to generate the SBOM image list")
	}

	copyViewerFile(sbomDir, "library.js")
	copyViewerFile(sbomDir, "viewer.js")
	copyViewerFile(sbomDir, "theme.css")
	copyViewerFile(sbomDir, "styles.css")

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

		normalized := tranformRegex.ReplaceAllString(tag.String(), "_")
		sbomFile, err := os.Create(filepath.Join(sbomDir, fmt.Sprintf("%s.json", normalized)))
		if err != nil {
			spinner.Fatalf(err, "Unable to create SBOM file for image %s", tag.String())
		}
		sbomViewerFile, err := os.Create(filepath.Join(sbomDir, fmt.Sprintf("%s.html", normalized)))
		if err != nil {
			spinner.Fatalf(err, "Unable to create SBOM viewer file for image %s", tag.String())
		}

		defer sbomFile.Close()
		defer sbomViewerFile.Close()

		enc := json.NewEncoder(sbomFile)
		if err := enc.Encode(sbomAttestor); err != nil {
			spinner.Fatalf(err, "Unable to write SBOM file for image %s", tag.String())
		}

		template, err := viewerAssets.ReadFile("viewer/template.html")
		if err != nil {
			spinner.Fatalf(err, "Unable to read SBOM Viewer template file")
		}

		if _, err = sbomViewerFile.Write(template); err != nil {
			spinner.Fatalf(err, "Unable to write SBOM Viewer template file")
		}

		if _, err = sbomFile.Seek(0, 0); err != nil {
			spinner.Fatalf(err, "Unable to load generated SBOM file for image %s", tag.String())
		}

		sbombViewerData, err := ioutil.ReadAll(sbomFile)
		if err != nil {
			spinner.Fatalf(err, "Unable to load generated SBOM file for image %s", tag.String())
		}

		sbomViewerJS := fmt.Sprintf(`
			ZARF_SBOM_IMAGE_LIST = %s;
			ZARF_SBOM_DATA = %s;		
		`, jsonImageList, sbombViewerData)

		utils.ReplaceText(sbomViewerFile.Name(), "//ZARF_JS_DATA", sbomViewerJS)

		currImage++
	}

	spinner.Success()
}

func copyViewerFile(sbomDir, name string) error {
	data, err := viewerAssets.ReadFile("viewer/" + name)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(sbomDir, name), data, 0644)
}

// This could be optimized, but loop over all the images to create an image tag list
func generateImageListJSON(tagToImage map[name.Tag]v1.Image) ([]byte, error) {
	var imageList []string

	for tag := range tagToImage {
		normalized := tranformRegex.ReplaceAllString(tag.String(), "_")
		imageList = append(imageList, normalized)
	}

	return json.Marshal(imageList)
}
