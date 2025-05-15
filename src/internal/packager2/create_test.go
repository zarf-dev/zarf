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
	path := filepath.Join("testdata/create")
	ctx := testutil.TestContext(t)
	reg := createRegistry(t, ctx)
	config.CommonOptions.PlainHTTP = true
	err := Create(ctx, path, CreateOptions{
		Output: fmt.Sprintf("oci://%s", reg.String()),
	})
	require.NoError(t, err)
	packageRef := fmt.Sprintf("%s/create-arch:0.0.1", reg.String())
	layout := pullFromRemote(t, ctx, packageRef, "amd64")
	require.Equal(t, layout.Pkg.Metadata.Architecture, "amd64")
}
