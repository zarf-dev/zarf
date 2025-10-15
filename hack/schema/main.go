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

	// Create a combined schema using oneOf based on apiVersion
	combinedSchema := &jsonschema.Schema{
		Title:       "Zarf Package Schema",
		Description: "Schema for Zarf packages supporting multiple API versions",
		OneOf: []*jsonschema.Schema{
			schemaV1Alpha1,
			schemaV1Beta1,
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
