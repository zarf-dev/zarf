// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

//go:build tools

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
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func main() {
	if err := writeSchema("v1alpha1"); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if err := writeSchema("v1beta1"); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func writeSchema(apiVersion string) error {
	var schema []byte
	var err error
	switch apiVersion {
	case "v1alpha1":
		schema, err = generateSchema("v1alpha1", &v1alpha1.ZarfPackage{})
	case "v1beta1":
		schema, err = generateSchema("v1beta1", &v1beta1.ZarfPackage{})
	default:
		return fmt.Errorf("unknown API version: %s", apiVersion)
	}
	if err != nil {
		return fmt.Errorf("error generating %s schema: %w", apiVersion, err)
	}

	// Add trailing newline to match linter expectations
	schema = append(schema, '\n')

	filename := fmt.Sprintf("zarf-%s-schema.json", apiVersion)
	if err := os.WriteFile(filename, schema, 0644); err != nil {
		return fmt.Errorf("error writing schema file: %w", err)
	}
	fmt.Printf("Successfully generated %s\n", filename)
	return nil
}

func generateSchema(apiVersion string, rootType any) ([]byte, error) {
	reflector := jsonschema.Reflector{ExpandedStruct: true}

	// AddGoComments breaks if called with an absolute path, so we save the current
	// directory, move to the directory of this source file, then use a relative path
	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unable to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("unable to get the current filename")
	}
	schemaDir := filepath.Dir(filename)
	if err := os.Chdir(schemaDir); err != nil {
		return nil, fmt.Errorf("unable to change to schema directory: %w", err)
	}

	typePackagePath := filepath.Join("..", "..", "api", apiVersion)
	modulePath := fmt.Sprintf("github.com/zarf-dev/zarf/src/api/%s", apiVersion)

	if err := reflector.AddGoComments(modulePath, typePackagePath); err != nil {
		return nil, fmt.Errorf("unable to add Go comments to schema: %w", err)
	}

	schema := reflector.Reflect(rootType)

	schemaData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to marshal schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaData, &schemaMap); err != nil {
		return nil, fmt.Errorf("unable to unmarshal schema: %w", err)
	}

	addYAMLExtensions(schemaMap)

	output, err := json.MarshalIndent(schemaMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to marshal final schema: %w", err)
	}

	return output, nil
}

// addYAMLExtensions walks through the JSON schema and adds patternProperties
// for "x-" prefixed fields to any object that has "properties".
// This allows YAML extensions (custom fields starting with x-) to be valid.
func addYAMLExtensions(data map[string]any) {
	propertiesKey := "properties"
	patternPropertiesKey := "patternProperties"
	yamlExtensionRegex := "^x-"
	if _, hasProperties := data[propertiesKey]; hasProperties {
		if _, hasPatternProps := data[patternPropertiesKey]; !hasPatternProps {
			data[patternPropertiesKey] = map[string]any{
				yamlExtensionRegex: map[string]any{},
			}
		}
	}

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
