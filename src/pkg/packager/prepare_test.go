// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestFindImages(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

	cfg := &types.PackagerConfig{
		CreateOpts: types.ZarfCreateOptions{
			BaseDir: "../../../examples/dos-games/",
		},
	}
	p, err := New(cfg)
	require.NoError(t, err)
	images, err := p.FindImages(ctx)
	require.NoError(t, err)
	expectedImages := map[string][]string{
		"baseline": {
			"ghcr.io/zarf-dev/doom-game:0.0.1",
			"ghcr.io/zarf-dev/doom-game:sha256-7464ecc8a7172fce5c2ad631fc2a1b8572c686f4bf15c4bd51d7d6c9f0c460a7.sig",
		},
	}
	require.Equal(t, len(expectedImages), len(images))
	for k, v := range expectedImages {
		require.ElementsMatch(t, v, images[k])
	}
}
