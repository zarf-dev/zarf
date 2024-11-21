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
	output, err := json.MarshalIndent(schema, "", "  ")
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
