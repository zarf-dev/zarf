package helpers

import (
	"bytes"
	"encoding/json"
	"log"
	"os"

	"github.com/santhosh-tekuri/jsonschema"
	"gopkg.in/yaml.v3"
)

func ValidateZarfSchema(yamlFile, jsonFile string) bool {

	yamlBytes, err := os.ReadFile(yamlFile)
	if err != nil {
		log.Fatalf("Error reading file: %s", err)
	}

	zarfSchema, err := os.ReadFile(jsonFile)
	if err != nil {
		log.Fatalf("Error reading file: %s", err)
	}

	var unmarshalledYaml interface{}
	err = yaml.Unmarshal(yamlBytes, &unmarshalledYaml)
	if err != nil {
		panic(err)
	}

	zarfYamlAsJsonBytes, err := json.Marshal(unmarshalledYaml)
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
