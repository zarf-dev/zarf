// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageCreatePublishArch(t *testing.T) {
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	// TODO set plainHTTP as a create option
	config.CommonOptions.PlainHTTP = true
	ctx := testutil.TestContext(t)
	tests := []struct {
		name string
		path string
		arch string
	}{
		{
			name: "empty arch; should use pkg.metadata.architecture",
			path: filepath.Join("testdata/create"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := createRegistry(t, ctx)
			config.CLIArch = tt.arch
			err := Create(ctx, tt.path, CreateOptions{
				Output: fmt.Sprintf("oci://%s", reg.String()),
			})
			require.NoError(t, err)
			layout := pullFromRemote(t, ctx, fmt.Sprintf("%s/create-arch:0.0.1", reg.String()), tt.arch)
			require.Equal(t, layout.Pkg.Metadata.Architecture, tt.arch)
		})
	}
}
