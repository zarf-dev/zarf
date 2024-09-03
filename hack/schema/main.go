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

var apiVersionToObject = map[string]interface{}{
	"v1alpha1": &v1alpha1.ZarfPackage{},
	"v1beta1":  &v1beta1.ZarfPackage{},
}

func genSchema(apiVersion string) (string, error) {
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

	typePackagePath := filepath.Join("..", "..", "src", "api", apiVersion)

	if err := reflector.AddGoComments("github.com/zarf-dev/zarf/hack/schema", typePackagePath); err != nil {
		return "", err
	}

	schema := reflector.Reflect(apiVersionToObject[apiVersion])
	output, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to generate the Zarf config schema: %w", err)
	}
	return string(output), nil
}

func main() {
	var apiVersions = []string{"v1alpha1", "v1beta1"}
	if len := len(os.Args); len != 2 {
		fmt.Println("This program must be called with the apiVersion, options are", apiVersions)
		os.Exit(1)
	}
	schema, err := genSchema(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(schema)
}
