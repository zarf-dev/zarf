package helpers

import (
	"os"
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

func TestValidateZarfSchema(t *testing.T) {
	t.Run("basic read schema", func(t *testing.T) {
		want := true
		var unmarshalledYaml interface{}
		readYaml("../../../../zarf.yaml", &unmarshalledYaml)
		if got := ValidateZarfSchema(unmarshalledYaml, "../../../../zarf.schema.json"); got != want {
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
