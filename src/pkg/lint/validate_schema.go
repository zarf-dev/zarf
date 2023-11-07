package lint

import (
	"errors"

	"github.com/xeipuuv/gojsonschema"
)

func ValidateSchema(unmarshalledYaml interface{}, jsonSchema []byte) error {
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(unmarshalledYaml)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		errorMessage := "The document is not valid:"
		for _, desc := range result.Errors() {
			errorMessage = errorMessage + "\n - " + desc.String()
		}
		err = errors.New(errorMessage)
	}

	return err
}
