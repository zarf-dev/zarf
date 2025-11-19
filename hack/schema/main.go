package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/invopop/jsonschema"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// addYAMLExtensions walks through the JSON schema and adds patternProperties
// for "x-" prefixed fields to any object that has "properties".
// This allows YAML extensions (custom fields starting with x-) to be valid.
func addYAMLExtensions(data map[string]any) {
	// Add pattern properties if this object has "properties"
	if _, hasProperties := data["properties"]; hasProperties {
		if _, hasPatternProps := data["patternProperties"]; !hasPatternProps {
			data["patternProperties"] = map[string]any{
				"^x-": map[string]any{},
			}
		}
	}

	// Recursively walk through all nested objects
	for _, v := range data {
		switch val := v.(type) {
		case map[string]any:
			addYAMLExtensions(val)
		case []any:
			for _, item := range val {
				if obj, ok := item.(map[string]any); ok {
					addYAMLExtensions(obj)
				}
			}
		}
	}
}

func genSchema() (string, error) {
	reflector := jsonschema.Reflector(jsonschema.Reflector{ExpandedStruct: true})

	// AddGoComments breaks if called with a absolute path, so we move to the directory of the go executable
	// then use a relative path to the package
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return "", errors.New("unable to get the current filename")
	}
	goExecDir := filepath.Dir(filename)
	if err := os.Chdir(goExecDir); err != nil {
		return "", err
	}

	typePackagePath := filepath.Join("..", "..", "src", "api", "v1alpha1")

	if err := reflector.AddGoComments("github.com/zarf-dev/zarf/hack/schema", typePackagePath); err != nil {
		return "", err
	}

	schema := reflector.Reflect(&v1alpha1.ZarfPackage{})
	schemaData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to generate the Zarf config schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaData, &schemaMap); err != nil {
		return "", fmt.Errorf("unable to parse schema JSON: %w", err)
	}

	addYAMLExtensions(schemaMap)

	output, err := json.MarshalIndent(schemaMap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to marshal schema JSON: %w", err)
	}
	return string(output), nil
}

func main() {
	schema, err := genSchema()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(schema)
}
