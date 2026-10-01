// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestComponentValidatesValuesSchema(t *testing.T) {
	t.Parallel()
	const schema = `{"type":"object","properties":{"image":{"type":"string","pattern":"^ghcr.io/"}}}`
	for _, tt := range []struct {
		name           string
		importedSchema bool
		schema         string
		wantErr        string
	}{
		{name: "local schema rejects values", schema: schema, wantErr: "values validation failed"},
		{name: "imported schema rejects values", importedSchema: true, schema: schema, wantErr: "values validation failed"},
		{name: "malformed schema", schema: `{`, wantErr: "parsing \"schema.json\" schema file"},
		{name: "missing schema", wantErr: "unable to access local resource \"schema.json\""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("image: docker.io/library/nginx:1.27\n"), 0o600))
			if tt.schema != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.json"), []byte(tt.schema), 0o600))
			}

			schemaField := "  schema: schema.json\n"
			componentField := "component: {}\n"
			if tt.importedSchema {
				schemaField = ""
				componentField = "component:\n  import:\n    local:\n      - path: child.yaml\n"
				child := `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
values:
  schema: schema.json
component: {}
`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "child.yaml"), []byte(child), 0o600))
			}
			config := `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: app
values:
  files:
    - values.yaml
` + schemaField + componentField
			configPath := filepath.Join(dir, "component.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

			_, err := Component(testutil.TestContext(t), configPath, ComponentOptions{})
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
