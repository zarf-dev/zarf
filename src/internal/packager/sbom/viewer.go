package sbom

import (
	"encoding/json"
	"html/template"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func (builder *Builder) createSBOMViewerAsset(tag name.Tag, jsonData []byte) error {

	// Create the sbom viewer file for the image
	sbomViewerFile, err := builder.createSBOMFile("sbom-viewer-%s.html", tag)
	if err != nil {
		return err
	}

	defer sbomViewerFile.Close()

	// Create the sbomviewer template data
	tplData := struct {
		ThemeCSS  template.CSS
		ViewerCSS template.CSS
		ImageList template.JS
		Data      template.JS
		LibraryJS template.JS
		ViewerJS  template.JS
	}{
		ThemeCSS:  builder.loadFileCSS("theme.css"),
		ViewerCSS: builder.loadFileCSS("styles.css"),
		ImageList: template.JS(builder.jsonImageList),
		Data:      template.JS(jsonData),
		LibraryJS: builder.loadFileJS("library.js"),
		ViewerJS:  builder.loadFileJS("viewer.js"),
	}

	// Render the sbomviewer template
	tpl, err := template.ParseFS(viewerAssets, "viewer/template.gohtml")
	if err != nil {
		return err
	}

	// Write the sbomviewer template to disk
	if err := tpl.Execute(sbomViewerFile, tplData); err != nil {
		return err
	}

	return nil
}

func (builder *Builder) loadFileCSS(name string) template.CSS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.CSS(data)
}

func (builder *Builder) loadFileJS(name string) template.JS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.JS(data)
}

// This could be optimized, but loop over all the images to create an image tag list
func (builder *Builder) generateImageListJSON(tagToImage map[name.Tag]v1.Image) ([]byte, error) {
	var imageList []string

	for tag := range tagToImage {
		normalized := builder.getNormalizedTag(tag)
		imageList = append(imageList, normalized)
	}

	return json.Marshal(imageList)
}
