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

func genSchemas() (string, error) {
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

	// Generate v1alpha1 schema with DoNotReference to inline all types
	reflectorV1Alpha1 := jsonschema.Reflector{DoNotReference: true, ExpandedStruct: true}
	typePackagePathV1Alpha1 := filepath.Join("..", "..", "src", "api", "v1alpha1")
	if err := reflectorV1Alpha1.AddGoComments("github.com/zarf-dev/zarf/hack/schema", typePackagePathV1Alpha1); err != nil {
		return "", err
	}
	schemaV1Alpha1 := reflectorV1Alpha1.Reflect(&v1alpha1.ZarfPackage{})

	// Generate v1beta1 schema with DoNotReference to inline all types
	reflectorV1Beta1 := jsonschema.Reflector{DoNotReference: true, ExpandedStruct: true}
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

	// Create a combined schema using if/then/else based on apiVersion
	combinedSchema := &jsonschema.Schema{
		Version:     "https://json-schema.org/draft/2020-12/schema",
		Title:       "Zarf Package Schema",
		Description: "Schema for Zarf packages supporting multiple API versions",
		If: &jsonschema.Schema{
			Properties: propsV1Alpha1,
		},
		Then: schemaV1Alpha1,
		Else: &jsonschema.Schema{
			If: &jsonschema.Schema{
				Properties: propsV1Beta1,
			},
			Then: schemaV1Beta1,
		},
	}

	output, err := json.MarshalIndent(combinedSchema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to generate the Zarf config schema: %w", err)
	}
	return string(output), nil
}

func main() {
	schema, err := genSchemas()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(schema)
}
