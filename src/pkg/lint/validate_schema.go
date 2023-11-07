package lint

import (
	"errors"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/xeipuuv/gojsonschema"
)

const (
	zarfInvalidPrefix = "zarf.yaml is not valid:"
	zarfWarningPrefix = "zarf schema warning:"
	zarfTemplateVar   = "###ZARF_PKG_TMPL_"
)

func ValidateSchema(unmarshalledYaml interface{}, jsonSchema []byte) error {
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(unmarshalledYaml)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		errorMessage := zarfInvalidPrefix
		for _, desc := range result.Errors() {
			errorMessage = errorMessage + "\n - " + desc.String()
		}
		err = errors.New(errorMessage)
	}

	return err
}

func checkForVarInComponentImport(unmarshalledYaml types.ZarfPackage) error {
	valid := true
	errorMessage := zarfWarningPrefix
	componentWarningStart := "\n - component."
	for i, component := range unmarshalledYaml.Components {
		if strings.Contains(component.Import.Path, zarfTemplateVar) {
			errorMessage = errorMessage + componentWarningStart + strconv.Itoa(i) +
				".import.path will not resolve ZARF_PKG_TMPL_* variables"
			valid = false
		}
		if strings.Contains(component.Import.URL, zarfTemplateVar) {
			errorMessage = errorMessage + componentWarningStart + strconv.Itoa(i) +
				".import.url will not resolve ZARF_PKG_TMPL_* variables"
			valid = false
		}
	}
	if valid {
		return nil
	}
	return errors.New(errorMessage)
}
