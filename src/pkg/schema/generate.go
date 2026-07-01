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
	if err := writeSchema("v1alpha1", "zarf-v1alpha1-schema.json", &v1alpha1.ZarfPackage{}); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if err := writeSchema("v1beta1", "zarf-v1beta1-package-schema.json", &v1beta1.Package{}); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if err := writeSchema("v1beta1", "zarf-v1beta1-component-schema.json", &v1beta1.ComponentConfig{}); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	combined, err := genSchemas()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if err := writeSchemaFile("zarf.schema.json", combined); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func writeSchema(apiVersion, filename string, rootType any) error {
	schema, err := generateSchema(apiVersion, rootType)
	if err != nil {
		return fmt.Errorf("error generating %s schema: %w", filename, err)
	}
	return writeSchemaFile(filename, schema)
}

func writeSchemaFile(filename string, schema []byte) error {
	schema = append(schema, '\n')

	if err := os.WriteFile(filename, schema, 0644); err != nil {
		return fmt.Errorf("error writing schema file: %w", err)
	}
	fmt.Printf("Successfully generated %s\n", filename)
	return nil
}

// genSchemas builds a single schema that validates either a v1alpha1 or a
// v1beta1 package, selecting which version to apply based on the apiVersion field.
func genSchemas() ([]byte, error) {
	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unable to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// AddGoComments breaks if called with an absolute path, so we move to the
	// directory of this source file and reflect using relative package paths.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("unable to get the current filename")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		return nil, fmt.Errorf("unable to change to schema directory: %w", err)
	}

	schemaV1Alpha1, err := reflectInlined("v1alpha1", &v1alpha1.ZarfPackage{})
	if err != nil {
		return nil, err
	}
	schemaV1Beta1, err := reflectInlined("v1beta1", &v1beta1.Package{})
	if err != nil {
		return nil, err
	}

	schema := &jsonschema.Schema{
		If: &jsonschema.Schema{
			Properties: jsonschema.NewProperties(),
		},
		Then: schemaV1Alpha1,
		Else: &jsonschema.Schema{
			If: &jsonschema.Schema{
				Properties: jsonschema.NewProperties(),
			},
			Then: schemaV1Beta1,
		},
		Version: jsonschema.Version,
	}

	schema.If.Properties.Set("apiVersion", &jsonschema.Schema{
		Type: "string",
		Enum: []any{v1alpha1.APIVersion},
	})
	schema.Else.If.Properties.Set("apiVersion", &jsonschema.Schema{
		Type: "string",
		Enum: []any{v1beta1.APIVersion},
	})

	return marshalSchema(schema)
}

// reflectInlined reflects rootType with all types inlined so the resulting
// schema can be embedded directly into a branch of the combined schema.
func reflectInlined(apiVersion string, rootType any) (*jsonschema.Schema, error) {
	reflector := jsonschema.Reflector{DoNotReference: true, ExpandedStruct: true}

	typePackagePath := filepath.Join("..", "..", "api", apiVersion)
	modulePath := fmt.Sprintf("github.com/zarf-dev/zarf/src/api/%s", apiVersion)
	if err := reflector.AddGoComments(modulePath, typePackagePath); err != nil {
		return nil, fmt.Errorf("unable to add Go comments to %s schema: %w", apiVersion, err)
	}

	schema := reflector.Reflect(rootType)
	// Drop the per-branch $schema so only the combined schema declares the dialect.
	schema.Version = ""
	return schema, nil
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

	return marshalSchema(schema)
}

// marshalSchema renders the schema to indented JSON, enriching every object
// that has properties with the "x-" YAML extension allowance.
func marshalSchema(schema *jsonschema.Schema) ([]byte, error) {
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
