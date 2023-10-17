package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"
)

func ValidateZarfSchema(zarfUnmarshaledYaml interface{}, zarfJsonSchema []byte) bool {

	zarfYamlAsJsonBytes, err := json.Marshal(zarfUnmarshaledYaml)
	if err != nil {
		panic(err)
	}

	compiler := jsonschema.NewCompiler()
	inMemoryZarfSchema := "zarf.schema.json"
	fmt.Println(string(zarfYamlAsJsonBytes))

	if err := compiler.AddResource(inMemoryZarfSchema, bytes.NewReader(zarfJsonSchema)); err != nil {
		panic(err)
	}
	schema, err := compiler.Compile(inMemoryZarfSchema)
	if err != nil {
		panic(err)
	}
	if err := schema.Validate(bytes.NewReader(zarfYamlAsJsonBytes)); err != nil {
		panic(err)
	}
	fmt.Println("validation succesful")
	return true
}
