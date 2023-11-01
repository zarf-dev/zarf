package lint

import (
	"os"
	"testing"

	goyaml "github.com/goccy/go-yaml"
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

	t.Run("Read schema success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml(t, "successful_validation/zarf.yaml")
		zarfSchema := readSchema(t)
		if got := ValidateSchema(unmarshalledYaml, zarfSchema); got != nil {
			t.Errorf("Expected successful validation, got error: %v", got)
		}
	})

	t.Run("Read schema fail", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml(t, "unsuccessful_validation/zarf.yaml")
		zarfSchema := readSchema(t)
		if err := ValidateSchema(unmarshalledYaml, zarfSchema); err == nil {
			t.Errorf("Expected validation to fail, but it succeeded.")
		}
	})

	t.Run("Read schema yaml extension", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml(t, "yaml-extension/zarf.yaml")
		zarfSchema := readSchema(t)
		if got := ValidateSchema(unmarshalledYaml, zarfSchema); got != nil {
			t.Errorf("Expected successful validation, got error: %v", got)
		}
	})
}
