// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom contains tools for generating SBOMs.
package sbom

import (
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func (b *Builder) createSBOMViewerAsset(identifier string, jsonData []byte) error {
	filename := fmt.Sprintf("sbom-viewer-%s.html", b.getNormalizedFileName(identifier))
	return b.createSBOMHTML(filename, "viewer/template.gohtml", jsonData)
}

func (b *Builder) createSBOMCompareAsset() error {
	return b.createSBOMHTML("compare.html", "viewer/compare.gohtml", []byte{})
}

func (b *Builder) createSBOMHTML(filename string, goTemplate string, jsonData []byte) error {
	// Create the sbom viewer file for the image
	sbomViewerFile, err := b.createSBOMFile(filename)
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
		CommonJS  template.JS
		ViewerJS  template.JS
		CompareJS template.JS
	}{
		ThemeCSS:  b.loadFileCSS("theme.css"),
		ViewerCSS: b.loadFileCSS("styles.css"),
		List:      template.JS(b.jsonList),
		Data:      template.JS(jsonData),
		LibraryJS: b.loadFileJS("library.js"),
		CommonJS:  b.loadFileJS("common.js"),
		ViewerJS:  b.loadFileJS("viewer.js"),
		CompareJS: b.loadFileJS("compare.js"),
	}

	// Render the sbomviewer template
	tpl, err := template.ParseFS(viewerAssets, goTemplate)
	if err != nil {
		return err
	}

	// Write the sbomviewer template to disk
	return tpl.Execute(sbomViewerFile, tplData)
}

func (b *Builder) loadFileCSS(name string) template.CSS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.CSS(data)
}

func (b *Builder) loadFileJS(name string) template.JS {
	data, _ := viewerAssets.ReadFile("viewer/" + name)
	return template.JS(data)
}

// This could be optimized, but loop over all the images and components to create a list of json files.
func (b *Builder) generateJSONList(componentToFiles map[string]*layout.ComponentSBOM, imageList []transform.Image) ([]byte, error) {
	var jsonList []string

	for _, refInfo := range imageList {
		normalized := b.getNormalizedFileName(refInfo.Reference)
		jsonList = append(jsonList, normalized)
	}

	for component := range componentToFiles {
		normalized := b.getNormalizedFileName(fmt.Sprintf("%s%s", componentPrefix, component))
		jsonList = append(jsonList, normalized)
	}

	return json.Marshal(jsonList)
}
