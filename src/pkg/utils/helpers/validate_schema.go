package helpers

import (
	"bytes"
	"encoding/json"
	"log"
	"os"

	"github.com/santhosh-tekuri/jsonschema"
)

func ValidateZarfSchema(zarfUnmarshaledYaml interface{}, jsonFile string) bool {

	zarfSchema, err := os.ReadFile(jsonFile)
	if err != nil {
		log.Fatalf("Error reading file: %s", err)
	}

	zarfYamlAsJsonBytes, err := json.Marshal(zarfUnmarshaledYaml)
	if err != nil {
		panic(err)
	}

	compiler := jsonschema.NewCompiler()
	inMemoryZarfSchema := "zarf.schema.json"

	if err := compiler.AddResource(inMemoryZarfSchema, bytes.NewReader(zarfSchema)); err != nil {
		panic(err)
	}
	schema, err := compiler.Compile(inMemoryZarfSchema)
	if err != nil {
		panic(err)
	}
	if err := schema.Validate(bytes.NewReader(zarfYamlAsJsonBytes)); err != nil {
		panic(err)
	}
	return true
}
