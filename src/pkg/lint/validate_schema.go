package lint

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"
)

func ValidateSchema(unmarshalledYaml interface{}, jsonSchema []byte) error {

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
		return fmt.Errorf("this is an error I'm messing with %s", err)
	}

	return nil
}
