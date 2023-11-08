package lint

import (
	"os"
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
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
		file, _ := os.ReadFile(path)
		goyaml.Unmarshal(file, &unmarshalledYaml)
		return unmarshalledYaml
	}

	readAndUnmarshallZarfPackage := func(t *testing.T, path string) types.ZarfPackage {
		t.Helper()
		var unmarshalledYaml types.ZarfPackage
		file, _ := os.ReadFile(path)
		goyaml.Unmarshal(file, &unmarshalledYaml)
		return unmarshalledYaml
	}

	t.Run("validate schema success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml(t, "../../../zarf.yaml")
		zarfSchema := readSchema(t)
		err := validateSchema(unmarshalledYaml, zarfSchema)
		require.NoError(t, err)
	})

	t.Run("validate schema fail", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml(t, "unsuccessful_validation/zarf.yaml")
		zarfSchema := readSchema(t)
		err := validateSchema(unmarshalledYaml, zarfSchema)
		errorMessage := zarfInvalidPrefix + `
 - components.0.import: Additional property not-path is not allowed
 - components.1: Additional property not-import is not allowed`
		require.EqualError(t, err, errorMessage)
	})

	t.Run("Template in component import success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshallZarfPackage(t, "successful_validation/zarf.yaml")
		err := checkForVarInComponentImport(unmarshalledYaml)
		require.NoError(t, err)
	})

	t.Run("Template in component import failure", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshallZarfPackage(t, "unsuccessful_validation/zarf.yaml")
		err := checkForVarInComponentImport(unmarshalledYaml)
		errorMessage := zarfWarningPrefix + " component.2.import.path will not resolve ZARF_PKG_TMPL_* variables. " +
			"component.3.import.url will not resolve ZARF_PKG_TMPL_* variables."
		require.EqualError(t, err, errorMessage)
	})
}
