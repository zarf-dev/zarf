package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

// updateRefs recursively updates all $ref paths in a schema to use a namespace prefix
func updateRefs(schema *jsonschema.Schema, prefix string) {
	if schema == nil {
		return
	}

	// Update the $ref if it points to a definition
	if schema.Ref != "" && len(schema.Ref) > 8 && schema.Ref[:8] == "#/$defs/" {
		defName := schema.Ref[8:]
		schema.Ref = "#/$defs/" + prefix + defName
	}

	// Recursively update refs in nested schemas
	for _, s := range schema.AllOf {
		updateRefs(s, prefix)
	}
	for _, s := range schema.AnyOf {
		updateRefs(s, prefix)
	}
	for _, s := range schema.OneOf {
		updateRefs(s, prefix)
	}
	updateRefs(schema.Not, prefix)
	updateRefs(schema.If, prefix)
	updateRefs(schema.Then, prefix)
	updateRefs(schema.Else, prefix)
	updateRefs(schema.Items, prefix)
	updateRefs(schema.Contains, prefix)
	updateRefs(schema.AdditionalProperties, prefix)
	updateRefs(schema.PropertyNames, prefix)

	// Update refs in properties
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			updateRefs(pair.Value, prefix)
		}
	}

	// Update refs in pattern properties
	for _, s := range schema.PatternProperties {
		updateRefs(s, prefix)
	}

	// Update refs in dependent schemas
	for _, s := range schema.DependentSchemas {
		updateRefs(s, prefix)
	}

	// Update refs in prefix items
	for _, s := range schema.PrefixItems {
		updateRefs(s, prefix)
	}

	// Update refs in definitions themselves
	for _, s := range schema.Definitions {
		updateRefs(s, prefix)
	}
}

func genSchema() (string, error) {
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

	// Generate v1alpha1 schema
	reflectorV1Alpha1 := jsonschema.Reflector{ExpandedStruct: true}
	typePackagePathV1Alpha1 := filepath.Join("..", "..", "src", "api", "v1alpha1")
	if err := reflectorV1Alpha1.AddGoComments("github.com/zarf-dev/zarf/hack/schema", typePackagePathV1Alpha1); err != nil {
		return "", err
	}
	schemaV1Alpha1 := reflectorV1Alpha1.Reflect(&v1alpha1.ZarfPackage{})

	// Generate v1beta1 schema
	reflectorV1Beta1 := jsonschema.Reflector{ExpandedStruct: true}
	typePackagePathV1Beta1 := filepath.Join("..", "..", "src", "api", "v1beta1")
	if err := reflectorV1Beta1.AddGoComments("github.com/zarf-dev/zarf/hack/schema", typePackagePathV1Beta1); err != nil {
		return "", err
	}
	schemaV1Beta1 := reflectorV1Beta1.Reflect(&v1beta1.ZarfPackage{})

	// Create ordered maps for the if conditions
	propsV1Alpha1 := orderedmap.New[string, *jsonschema.Schema]()
	propsV1Alpha1.Set("apiVersion", &jsonschema.Schema{
		Const: "zarf.dev/v1alpha1",
	})

	propsV1Beta1 := orderedmap.New[string, *jsonschema.Schema]()
	propsV1Beta1.Set("apiVersion", &jsonschema.Schema{
		Const: "zarf.dev/v1beta1",
	})

	// Namespace and merge $defs from both schemas to avoid conflicts
	mergedDefs := make(jsonschema.Definitions)

	// Add v1alpha1 definitions with namespace prefix
	if schemaV1Alpha1.Definitions != nil {
		for key, value := range schemaV1Alpha1.Definitions {
			mergedDefs["v1alpha1-"+key] = value
		}
	}

	// Add v1beta1 definitions with namespace prefix
	if schemaV1Beta1.Definitions != nil {
		for key, value := range schemaV1Beta1.Definitions {
			mergedDefs["v1beta1-"+key] = value
		}
	}

	// Update $refs in v1alpha1 schema to use namespaced definitions
	updateRefs(schemaV1Alpha1, "v1alpha1-")

	// Update $refs in v1beta1 schema to use namespaced definitions
	updateRefs(schemaV1Beta1, "v1beta1-")

	// Create a combined schema using if/then/else based on apiVersion
	combinedSchema := &jsonschema.Schema{
		Version:     "https://json-schema.org/draft/2020-12/schema",
		Title:       "Zarf Package Schema",
		Description: "Schema for Zarf packages supporting multiple API versions",
		Definitions: mergedDefs,
		AllOf: []*jsonschema.Schema{
			{
				If: &jsonschema.Schema{
					Properties: propsV1Alpha1,
				},
				Then: schemaV1Alpha1,
			},
			{
				If: &jsonschema.Schema{
					Properties: propsV1Beta1,
				},
				Then: schemaV1Beta1,
			},
		},
	}

	output, err := json.MarshalIndent(combinedSchema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to generate the Zarf config schema: %w", err)
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
