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
			"defenseunicorns/zarf-game:multi-tile-dark",
			"index.docker.io/defenseunicorns/zarf-game:sha256-0b694ca1c33afae97b7471488e07968599f1d2470c629f76af67145ca64428af.sig",
		},
	}
	require.Equal(t, len(expectedImages), len(images))
	for k, v := range expectedImages {
		require.ElementsMatch(t, v, images[k])
	}
}
