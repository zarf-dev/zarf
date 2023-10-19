package helpers

import (
	"log"
	"os"
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

func TestValidateZarfSchema(t *testing.T) {
	t.Run("Read schema success", func(t *testing.T) {
		var unmarshalledYaml interface{}
		readYaml("successful_validation/zarf.yaml", &unmarshalledYaml)
		zarfSchema, err := os.ReadFile("../../../../zarf.schema.json")
		if err != nil {
			log.Fatalf("Error reading file: %s", err)
		}
		if got := ValidateZarfSchema(unmarshalledYaml, zarfSchema); got != nil {
			t.Errorf("ValidateZarfSchema = %v, want %v", got, nil)
		}
	})

	t.Run("Read schema fail", func(t *testing.T) {
		var unmarshalledYaml interface{}
		readYaml("unsuccessful_validation/bad_zarf.yaml", &unmarshalledYaml)
		zarfSchema, err := os.ReadFile("../../../../zarf.schema.json")
		if err != nil {
			log.Fatalf("Error reading file: %s", err)
		}
		err = ValidateZarfSchema(unmarshalledYaml, zarfSchema)
		if err == nil {
			t.Errorf("ValidateZarfSchema worked on a bad file")
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
