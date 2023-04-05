// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMutateGitURLsInText(t *testing.T) {
	originalText := `
	# We handle invalid URLs (see below comment)
	# We transform https://*/*.git URLs
	https://github.com/defenseunicorns/zarf.git
	# Even URLs with things on either side
	stuffhttps://github.com/defenseunicorns/zarf.gitandthings
	# But not ssh://*/*.git URLs
	ssh://git@github.com:defenseunicorns/zarf.git
	# Or non .git URLs
	https://www.defenseunicorns.com/
	`

	expectedText := `
	# We handle invalid URLs (see below comment)
	# We transform https://*/*.git URLs
	https://gitlab.com/repo-owner/zarf-1211668992.git
	# Even URLs with things on either side
	stuffhttps://gitlab.com/repo-owner/zarf-1211668992.gitandthings
	# But not ssh://*/*.git URLs
	ssh://git@github.com:defenseunicorns/zarf.git
	# Or non .git URLs
	https://www.defenseunicorns.com/
	`

	resultingText := MutateGitURLsInText("https://gitlab.com", originalText, "repo-owner")
	assert.Equal(t, expectedText, resultingText)
}
