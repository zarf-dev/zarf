package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/santhosh-tekuri/jsonschema"
	"gopkg.in/yaml.v3"
)

func ReadSchema(yamlFile, jsonFile string) bool {

	yamlBytes, err := os.ReadFile(yamlFile)
	if err != nil {
		log.Fatalf("Error reading YAML file: %s", yamlFile)
	}

	jsonBytes, err := os.ReadFile(jsonFile)
	if err != nil {
		log.Fatalf("Error reading YAML file: %s", jsonFile)
	}

	var m interface{}
	err = yaml.Unmarshal(yamlBytes, &m)
	if err != nil {
		panic(err)
	}

	mJSON, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	yamlJsonReader := bytes.NewReader(mJSON)
	schemaReader := bytes.NewReader(jsonBytes)

	compiler := jsonschema.NewCompiler()

	if err := compiler.AddResource("schema.json", schemaReader); err != nil {
		panic(err)
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		panic(err)
	}
	if err := schema.Validate(yamlJsonReader); err != nil {
		panic(err)
	}
	fmt.Println("validation successfull")
	return true
}
