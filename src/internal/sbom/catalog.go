package sbom

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
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
	"github.com/testifysec/go-witness/attestation"
	"github.com/testifysec/go-witness/attestation/syft"
)

//go:embed viewer/*
var viewerAssets embed.FS
var tranformRegex = regexp.MustCompile(`(?m)[^a-zA-Z0-9\.\-]`)

func CatalogImages(tagToImage map[name.Tag]v1.Image, sbomDir, tarPath string) {
	// Ignore SBOM creation if there the flag is set
	if config.SkipSBOM {
		message.Debug("Skipping SBOM processing per --skip-sbom flag")
		return
	}

	imageCount := len(tagToImage)
	spinner := message.NewProgressSpinner("Creating SBOMs for %d images.", imageCount)
	defer spinner.Stop()

	actx, err := attestation.NewContext([]attestation.Attestor{})
	if err != nil {
		spinner.Fatalf(err, "Unable to make attestation context")
	}

	// Ensure the sbom directory exists
	_ = utils.CreateDirectory(sbomDir, 0700)

	cachePath := config.GetImageCachePath()
	currImage := 1

	// Generate a list of images for the sbom viewer
	jsonImageList, err := generateImageListJSON(tagToImage)
	if err != nil {
		spinner.Fatalf(err, "Unable to generate the SBOM image list")
	}

	// Generate SBOM for each image
	for tag := range tagToImage {
		spinner.Updatef("Creating image SBOMs (%d of %d): %s", currImage, imageCount, tag)

		// Get the image
		tarballImg, err := tarball.ImageFromPath(tarPath, &tag)
		if err != nil {
			spinner.Fatalf(err, "Unable to open image %s", tag.String())
		}

		// Create the sbom
		sbomAttestor := syft.New(syft.WithImageSource(tarballImg, cachePath, tag.String()))
		if err := sbomAttestor.Attest(actx); err != nil {
			spinner.Fatalf(err, "Unable to build sbom for image %s", tag.String())
		}

		// Write the sbom to disk using the image tag as the filename
		normalized := tranformRegex.ReplaceAllString(tag.String(), "_")
		sbomFile, err := os.Create(filepath.Join(sbomDir, fmt.Sprintf("%s.json", normalized)))
		if err != nil {
			spinner.Fatalf(err, "Unable to create SBOM file for image %s", tag.String())
		}

		// Create the sbom viewer file for the image
		sbomViewerFile, err := os.Create(filepath.Join(sbomDir, fmt.Sprintf("sbom-viewer-%s.html", normalized)))
		if err != nil {
			spinner.Fatalf(err, "Unable to create SBOM viewer file for image %s", tag.String())
		}

		defer sbomFile.Close()
		defer sbomViewerFile.Close()

		// Write the sbom json data to disk
		enc := json.NewEncoder(sbomFile)
		if err := enc.Encode(sbomAttestor); err != nil {
			spinner.Fatalf(err, "Unable to write SBOM file for image %s", tag.String())
		}

		// Reset the file reader to the start of the file
		if _, err = sbomFile.Seek(0, 0); err != nil {
			spinner.Fatalf(err, "Unable to load generated SBOM file for image %s", tag.String())
		}

		// Read the sbom json data into memory (avoid JSON encoding large structs twice)
		sbombViewerData, err := ioutil.ReadAll(sbomFile)
		if err != nil {
			spinner.Fatalf(err, "Unable to load generated SBOM file for image %s", tag.String())
		}

		// Create the sbomviewer template data
		tplData := struct {
			ThemeCSS  template.CSS
			ViewerCSS template.CSS
			ImageList template.JS
			Data      template.JS
			LibraryJS template.JS
			ViewerJS  template.JS
		}{
			ThemeCSS:  loadFileCSS("theme.css"),
			ViewerCSS: loadFileCSS("styles.css"),
			ImageList: template.JS(jsonImageList),
			Data:      template.JS(sbombViewerData),
			LibraryJS: loadFileJS("library.js"),
			ViewerJS:  loadFileJS("viewer.js"),
		}

		// Render the sbomviewer template
		tpl, err := template.ParseFS(viewerAssets, "viewer/template.gohtml")
		if err != nil {
			spinner.Fatalf(err, "Unable to parse SBOM Viewer template file")
		}

		// Write the sbomviewer template to disk
		if err := tpl.Execute(sbomViewerFile, tplData); err != nil {
			spinner.Fatalf(err, "Unable to execute SBOM Viewer template file")
		}

		currImage++
	}

	spinner.Success()
}

func loadFileCSS(name string) template.CSS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.CSS(data)
}

func loadFileJS(name string) template.JS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.JS(data)
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
