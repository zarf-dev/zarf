// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package schema provides functions for generating and working with the Zarf JSON schema.
package schema

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

// GenerateSchema generates the JSON schema for the Zarf package configuration.
// It returns the schema as a byte slice suitable for validation or output.
func GenerateSchema() ([]byte, error) {
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

	typePackagePath := filepath.Join("..", "..", "api", "v1alpha1")

	// Get the Go comments from the v1alpha1 package
	if err := reflector.AddGoComments("github.com/zarf-dev/zarf/src/pkg/schema", typePackagePath); err != nil {
		return nil, fmt.Errorf("unable to add Go comments to schema: %w", err)
	}

	// Generate the schema from the ZarfPackage type
	schema := reflector.Reflect(&v1alpha1.ZarfPackage{})

	// Marshal to JSON
	schemaData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to marshal schema: %w", err)
	}

	// Parse back to a map so we can add YAML extensions
	var schemaMap map[string]any
	if err := json.Unmarshal(schemaData, &schemaMap); err != nil {
		return nil, fmt.Errorf("unable to unmarshal schema: %w", err)
	}

	// Add YAML extension support
	addYAMLExtensions(schemaMap)

	// Marshal back to JSON with indentation
	output, err := json.MarshalIndent(schemaMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to marshal final schema: %w", err)
	}

	return output, nil
}
