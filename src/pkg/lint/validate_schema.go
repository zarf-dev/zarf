package lint

import (
	"errors"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/xeipuuv/gojsonschema"
)

const (
	zarfInvalidPrefix = "zarf.yaml is not valid:"
	zarfWarningPrefix = "zarf schema warning:"
	ZarfTemplateVar   = "###ZARF_PKG_TMPL_"
)

func validateSchema(unmarshalledYaml interface{}, jsonSchema []byte) error {
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(unmarshalledYaml)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		errorMessage := zarfInvalidPrefix
		for _, desc := range result.Errors() {
			errorMessage = fmt.Sprintf("%s\n - %s", errorMessage, desc.String())
		}
		err = errors.New(errorMessage)
	}

	return err
}

func checkForVarInComponentImport(zarfYaml types.ZarfPackage) error {
	valid := true
	errorMessage := zarfWarningPrefix
	componentWarningStart := "component."
	for i, component := range zarfYaml.Components {
		if strings.Contains(component.Import.Path, ZarfTemplateVar) {
			errorMessage = fmt.Sprintf("%s %s%d.import.path will not resolve ZARF_PKG_TMPL_* variables.",
				errorMessage, componentWarningStart, i)
			valid = false
		}
		if strings.Contains(component.Import.URL, ZarfTemplateVar) {
			errorMessage = fmt.Sprintf("%s %s%d.import.url will not resolve ZARF_PKG_TMPL_* variables.",
				errorMessage, componentWarningStart, i)
			valid = false
		}
	}
	if valid {
		return nil
	}
	return errors.New(errorMessage)
}
