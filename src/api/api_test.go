// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
)

func TestPackageSetBuildDataPreservesSourceVersion(t *testing.T) {
	pkg := api.Package{Build: api.BuildData{OriginalAPIVersion: "zarf.dev/v1beta1"}}
	pkg.SetBuildData(api.BuildData{Version: "test", AggregateChecksum: "checksum"})

	require.Equal(t, "zarf.dev/v1beta1", pkg.OriginalAPIVersion())
	require.Equal(t, "checksum", pkg.Metadata.AggregateChecksum)
}
