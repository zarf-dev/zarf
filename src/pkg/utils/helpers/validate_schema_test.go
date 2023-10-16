package helpers

import (
	"log"
	"os"
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

func TestValidateZarfSchema(t *testing.T) {
	t.Run("basic read schema", func(t *testing.T) {
		want := true
		var unmarshalledYaml interface{}
		readYaml("../../../../zarf.yaml", &unmarshalledYaml)
		zarfSchema, err := os.ReadFile("../../../../zarf.schema.json")
		if err != nil {
			log.Fatalf("Error reading file: %s", err)
		}
		if err != nil {
			panic(err)
		}
		if got := ValidateZarfSchema(unmarshalledYaml, zarfSchema); got != want {
			t.Errorf("ValidateZarfSchema = %v, want %v", got, want)
		}
	})
}

func readYaml(path string, destConfig any) error {
	file, err := os.ReadFile(path)

	if err != nil {
		return err
	}

	return goyaml.Unmarshal(file, destConfig)
}
