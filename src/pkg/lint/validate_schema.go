package lint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/santhosh-tekuri/jsonschema"
	"github.com/xeipuuv/gojsonschema"
)

func ValidateSchema2(unmarshalledYaml interface{}, jsonSchema []byte) error {

	zarfYamlAsJsonBytes, err := json.Marshal(unmarshalledYaml)
	if err != nil {
		return err
	}

	compiler := jsonschema.NewCompiler()
	inMemoryZarfSchema := "schema.json"

	if err := compiler.AddResource(inMemoryZarfSchema, bytes.NewReader(jsonSchema)); err != nil {
		return err
	}
	schema, err := compiler.Compile(inMemoryZarfSchema)
	if err != nil {
		return err
	}
	if err := schema.Validate(bytes.NewReader(zarfYamlAsJsonBytes)); err != nil {
		return fmt.Errorf("schema validation error: %s", err)
	}

	return nil
}

func ValidateSchema(unmarshalledYaml interface{}, jsonSchema []byte) error {
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(unmarshalledYaml)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if result.Valid() {
		message.Success("The document is valid\n")
	} else {
		errorMessage := "The document is not valid: "
		for i, desc := range result.Errors() {
			if i != 0 {
				errorMessage = errorMessage + ", "
			}
			errorMessage = errorMessage + desc.String()
		}
		err = errors.New(errorMessage)
	}

	return err
}
