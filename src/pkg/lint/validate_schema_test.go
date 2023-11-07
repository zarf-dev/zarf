package lint

import (
	"os"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

func TestValidateSchema(t *testing.T) {
	readSchema := func(t *testing.T) []byte {
		t.Helper()
		zarfSchema, err := os.ReadFile("../../../zarf.schema.json")
		if err != nil {
			t.Fatalf("Error reading schema file: %s", err)
		}
		return zarfSchema
	}

	readAndUnmarshalYaml := func(t *testing.T, path string) interface{} {
		t.Helper()
		var unmarshalledYaml interface{}
		file, err := os.ReadFile(path)
		goyaml.Unmarshal(file, &unmarshalledYaml)
		if err != nil {
			return err
		}
		return unmarshalledYaml
	}

	// unmarshallStringYaml := func(t *testing.T, str string) (interface{}, error) {
	// 	t.Helper()
	// 	var unmarshalledYaml interface{}
	// 	err := goyaml.Unmarshal([]byte(str), &unmarshalledYaml)
	// 	return unmarshalledYaml, err
	// }

	t.Run("Read schema success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml(t, "successful_validation/zarf.yaml")
		zarfSchema := readSchema(t)
		err := validateSchema(unmarshalledYaml, zarfSchema)
		require.NoError(t, err)
		if got := validateSchema(unmarshalledYaml, zarfSchema); got != nil {
			t.Errorf("Expected successful validation, got error: %v", got)
		}
	})

	t.Run("Read schema fail", func(t *testing.T) {
		// 		var yamlContents = `
		// kind: ZarfInitConfig
		// metadata:
		//   name: init
		// components:
		//   - name: k3s
		//     import:
		//       pa324234th: test
		//   - name: test
		//     import:
		// 	  pa3th: test
		// `
		// 		unmarshalledYaml,_ := unmarshallStringYaml(t, yamlContents)
		zarfSchema := readSchema(t)
		unmarshalledYaml := readAndUnmarshalYaml(t, "unsuccessful_validation/zarf.yaml")
		err := validateSchema(unmarshalledYaml, zarfSchema)
		errorMessage := `The document is not valid:
 - components.0.import: Additional property not-path is not allowed
 - components.1: Additional property not-import is not allowed`
		require.EqualError(t, err, errorMessage)
	})

	// t.Run("bad yaml", func(t *testing.T) {
	// 	var yamlContents = "unquoted_string_with_colons: key: value"
	// 	unmarshalledYaml,err := unmarshallStringYaml(t, yamlContents)
	// 	zarfSchema := readSchema(t)
	// 	err = ValidateSchema(unmarshalledYaml, zarfSchema)
	// 	require.Contains(t, err.Error(), "components.0.import: Additional property pa324234th is not allowed")
	// 	if err == nil {
	// 		t.Errorf("Expected validation to fail, but it succeeded.")
	// 	}
	// })

	t.Run("Read schema yaml extension", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml(t, "yaml-extension/zarf.yaml")
		zarfSchema := readSchema(t)
		err := validateSchema(unmarshalledYaml, zarfSchema)
		require.NoError(t, err)
		if got := validateSchema(unmarshalledYaml, zarfSchema); got != nil {
			t.Errorf("Expected successful validation, got error: %v", got)
		}
	})
}
