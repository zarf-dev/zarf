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
		name         string
		path         string
		expectedArch string
		packageName  string
	}{
		{
			name:         "should use pkg.metadata.architecture when global arch not set",
			path:         filepath.Join("testdata", "create", "create-publish-arch"),
			packageName:  "create-arch",
			expectedArch: "amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := createRegistry(t, ctx)
			err := Create(ctx, tt.path, CreateOptions{
				Output: fmt.Sprintf("oci://%s", reg.String()),
			})
			require.NoError(t, err)
			packageURL := fmt.Sprintf("%s/%s:0.0.1", reg.String(), tt.packageName)
			layout := pullFromRemote(t, ctx, packageURL, tt.expectedArch)
			require.Equal(t, layout.Pkg.Metadata.Architecture, tt.expectedArch)
		})
	}
}
