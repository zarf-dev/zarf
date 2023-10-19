package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"
)

func ValidateZarfSchema(zarfUnmarshaledYaml interface{}, zarfJsonSchema []byte) error {

	zarfYamlAsJsonBytes, err := json.Marshal(zarfUnmarshaledYaml)
	if err != nil {
		return err
	}

	compiler := jsonschema.NewCompiler()
	inMemoryZarfSchema := "zarf.schema.json"
	fmt.Println("we are here")
	fmt.Println(zarfYamlAsJsonBytes)

	if err := compiler.AddResource(inMemoryZarfSchema, bytes.NewReader(zarfJsonSchema)); err != nil {
		return err
	}
	schema, err := compiler.Compile(inMemoryZarfSchema)
	if err != nil {
		return err
	}
	if err := schema.Validate(bytes.NewReader(zarfYamlAsJsonBytes)); err != nil {
		return err
	}

	return nil
}
