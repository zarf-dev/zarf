// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package creator

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestRemoveCopiesFromComponents(t *testing.T) {
	components := []types.ZarfComponent{
		{
			Images: []string{
				"example.com/include-image-tag:latest",
				"example.com/image-with-tag:v1",
				"example.com/diff-image-with-tag:v1",
				"example.com/image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				"example.com/diff-image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				"example.com/image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				"example.com/diff-image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			Repos: []string{
				"https://example.com/no-ref.git",
				"https://example.com/branch.git@refs/heads/main",
				"https://example.com/tag.git@v1",
				"https://example.com/diff-tag.git@v1",
				"https://example.com/commit.git@524980951ff16e19dc25232e9aea8fd693989ba6",
				"https://example.com/diff-commit.git@524980951ff16e19dc25232e9aea8fd693989ba6",
			},
		},
	}
	loadedDiffData := types.DifferentialData{
		DifferentialImages: map[string]bool{
			"example.com/include-image-tag:latest": true,
			"example.com/diff-image-with-tag:v1":   true,
			"example.com/diff-image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855":            true,
			"example.com/diff-image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855": true,
		},
		DifferentialRepos: map[string]bool{
			"https://example.com/no-ref.git":                                               true,
			"https://example.com/branch.git@refs/heads/main":                               true,
			"https://example.com/diff-tag.git@v1":                                          true,
			"https://example.com/diff-commit.git@524980951ff16e19dc25232e9aea8fd693989ba6": true,
		},
	}
	diffComponents, err := removeCopiesFromComponents(components, &loadedDiffData)
	require.NoError(t, err)

	expectedImages := []string{
		"example.com/include-image-tag:latest",
		"example.com/image-with-tag:v1",
		"example.com/image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"example.com/image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	require.ElementsMatch(t, expectedImages, diffComponents[0].Images)
	expectedRepos := []string{
		"https://example.com/no-ref.git",
		"https://example.com/branch.git@refs/heads/main",
		"https://example.com/tag.git@v1",
		"https://example.com/commit.git@524980951ff16e19dc25232e9aea8fd693989ba6",
	}
	require.ElementsMatch(t, expectedRepos, diffComponents[0].Repos)
}
