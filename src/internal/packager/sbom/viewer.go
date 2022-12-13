// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom contains tools for generating SBOMs
package sbom

import (
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func (builder *Builder) createSBOMViewerAsset(identifier string, jsonData []byte) error {

	// Create the sbom viewer file for the image
	sbomViewerFile, err := builder.createSBOMFile("sbom-viewer-%s.html", identifier)
	if err != nil {
		return err
	}

	defer sbomViewerFile.Close()

	// Create the sbomviewer template data
	tplData := struct {
		ThemeCSS  template.CSS
		ViewerCSS template.CSS
		List      template.JS
		Data      template.JS
		LibraryJS template.JS
		ViewerJS  template.JS
	}{
		ThemeCSS:  builder.loadFileCSS("theme.css"),
		ViewerCSS: builder.loadFileCSS("styles.css"),
		List:      template.JS(builder.jsonList),
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
	return tpl.Execute(sbomViewerFile, tplData)
}

func (builder *Builder) loadFileCSS(name string) template.CSS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.CSS(data)
}

func (builder *Builder) loadFileJS(name string) template.JS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.JS(data)
}

// This could be optimized, but loop over all the images and components to create a list of json files
func (builder *Builder) generateJSONList(componentToFiles map[string][]string, tagToImage map[name.Tag]v1.Image) ([]byte, error) {
	var jsonList []string

	for tag := range tagToImage {
		normalized := builder.getNormalizedFileName(tag.String())
		jsonList = append(jsonList, normalized)
	}

	for component := range componentToFiles {
		normalized := builder.getNormalizedFileName(fmt.Sprintf("zarf-component-%s", component))
		jsonList = append(jsonList, normalized)
	}

	return json.Marshal(jsonList)
}
