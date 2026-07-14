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

// schemaAPI describes a single API version to include when generating a schema.
type schemaAPI struct {
	// dir is the package directory under src/api, used for reflection and Go comments.
	dir string
	// apiVersion is the value of the apiVersion field that selects this API in a combined schema.
	apiVersion string
	rootType   any
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	v1alpha1Package := schemaAPI{dir: "v1alpha1", apiVersion: v1alpha1.APIVersion, rootType: &v1alpha1.ZarfPackage{}}
	v1beta1Package := schemaAPI{dir: "v1beta1", apiVersion: v1beta1.APIVersion, rootType: &v1beta1.Package{}}
	v1beta1Component := schemaAPI{dir: "v1beta1", apiVersion: v1beta1.APIVersion, rootType: &v1beta1.ComponentConfig{}}

	if err := writeSchema("zarf-v1alpha1-schema.json", v1alpha1Package); err != nil {
		return err
	}
	if err := writeSchema("zarf-v1beta1-package-schema.json", v1beta1Package); err != nil {
		return err
	}
	if err := writeSchema("zarf-v1beta1-component-schema.json", v1beta1Component); err != nil {
		return err
	}

	combined, err := generateSchema(v1alpha1Package, v1beta1Package)
	if err != nil {
		return err
	}
	return writeSchemaFile("zarf.schema.json", combined)
}

func writeSchema(filename string, apis ...schemaAPI) error {
	schema, err := generateSchema(apis...)
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

// generateSchema reflects the given APIs into a JSON schema. A single API produces
// a plain schema; multiple APIs produce a combined schema that selects the matching
// branch based on the apiVersion field.
func generateSchema(apis ...schemaAPI) ([]byte, error) {
	restore, err := chdirToSchemaDir()
	if err != nil {
		return nil, err
	}
	defer restore()

	combined := len(apis) > 1

	schemas := make([]*jsonschema.Schema, len(apis))
	for i, api := range apis {
		// Branches of a combined schema must be inlined so each stands alone.
		schemas[i], err = reflectAPI(api, combined)
		if err != nil {
			return nil, err
		}
	}

	if !combined {
		return marshalSchema(schemas[0])
	}
	return marshalSchema(combineSchemas(apis, schemas))
}

// chdirToSchemaDir moves into this source file's directory so reflection can use
// relative package paths, returning a function that restores the original directory.
// AddGoComments breaks if called with an absolute path.
func chdirToSchemaDir() (func(), error) {
	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unable to get current directory: %w", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("unable to get the current filename")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		return nil, fmt.Errorf("unable to change to schema directory: %w", err)
	}
	return func() { os.Chdir(originalDir) }, nil
}

// reflectAPI reflects api.rootType into a schema. When inline is true all types are
// expanded in place so the schema can be embedded into a branch of a combined schema.
func reflectAPI(api schemaAPI, inline bool) (*jsonschema.Schema, error) {
	reflector := jsonschema.Reflector{ExpandedStruct: true, DoNotReference: inline}

	typePackagePath := filepath.Join("..", "..", "api", api.dir)
	modulePath := fmt.Sprintf("github.com/zarf-dev/zarf/src/api/%s", api.dir)
	if err := reflector.AddGoComments(modulePath, typePackagePath); err != nil {
		return nil, fmt.Errorf("unable to add Go comments to %s schema: %w", api.dir, err)
	}

	schema := reflector.Reflect(api.rootType)
	if inline {
		// Drop the per-branch $schema so only the combined schema declares the dialect.
		schema.Version = ""
	}
	return schema, nil
}

// combineSchemas chains the per-API schemas into one that applies the branch whose
// apiVersion matches, nesting each remaining version in the else clause.
func combineSchemas(apis []schemaAPI, schemas []*jsonschema.Schema) *jsonschema.Schema {
	root := &jsonschema.Schema{Version: jsonschema.Version}
	current := root
	for i := range apis {
		current.If = &jsonschema.Schema{Properties: jsonschema.NewProperties()}
		current.If.Properties.Set("apiVersion", &jsonschema.Schema{
			Type: "string",
			Enum: []any{apis[i].apiVersion},
		})
		current.Then = schemas[i]
		// Nest the remaining versions in the else clause.
		if i < len(apis)-1 {
			current.Else = &jsonschema.Schema{}
			current = current.Else
		}
	}
	return root
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
