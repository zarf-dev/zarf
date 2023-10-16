package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
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
	m, err = toStringKeys(m)
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

func toStringKeys(val interface{}) (interface{}, error) {
	var err error
	switch val := val.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range val {
			k, ok := k.(string)
			if !ok {
				return nil, errors.New("found non-string key")
			}
			m[k], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case []interface{}:
		var l = make([]interface{}, len(val))
		for i, v := range val {
			l[i], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return l, nil
	default:
		return val, nil
	}
}
