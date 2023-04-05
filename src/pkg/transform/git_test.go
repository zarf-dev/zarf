// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var gitURLs = []string{
	// Normal git repos and references for pushing/pulling
	"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git",
	"https://github.com/defenseunicorns/zarf.git",
	"https://ghcr.io/stefanprodan/podinfo_fasd-123.git",
	"git://k3d-cluster.localhost/defenseunicorns/zarf-agent",
	"http://localhost:5000/some-cool-repo",
	"ssh://ghcr.io/stefanprodan/podinfo@6.0.0",
	"https://stefanprodan/podinfo.git@adf0fasd10.1.223124123123-asdf",
	"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git@0.0.9-bb.0",
	"file:///srv/git/stefanprodan/podinfo.git@adf0fasd10.1.223124123123-asdf",
	"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test",
	"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test@524980951ff16e19dc25232e9aea8fd693989ba6",
	"https://github.com/defenseunicorns/zarf.helm.git",
	"https://github.com/defenseunicorns/zarf.git@refs/tags/v0.16.0",
	"https://github.com/DoD-Platform-One/big-bang.git@refs/heads/release-1.54.x",

	// Smart Git Protocol URLs for proxying (https://www.git-scm.com/docs/http-protocol)
	"https://github.com/defenseunicorns/zarf.helm.git/info/refs",
	"https://github.com/defenseunicorns/zarf.helm.git/info/refs?service=git-upload-pack",
	"https://github.com/defenseunicorns/zarf.helm.git/info/refs?service=git-receive-pack",
	"https://github.com/defenseunicorns/zarf.helm.git/git-upload-pack",
	"https://github.com/defenseunicorns/zarf.helm.git/git-receive-pack",
}

var badURLs = []string{
	"i am not a url at all",
	"C:\\Users\\zarf",
}

func TestMutateGitURLsInText(t *testing.T) {
	originalText := `
	# Here we handle invalid URLs (see below comment)
	# We transform https://*/*.git URLs
	https://github.com/defenseunicorns/zarf.git
	# Even URLs with things on either side
	stuffhttps://github.com/defenseunicorns/zarf.gitandthings
	# But not ssh://*/*.git URLs
	ssh://git@github.com/defenseunicorns/zarf.git
	# Or non .git URLs
	https://www.defenseunicorns.com/
	`

	expectedText := `
	# Here we handle invalid URLs (see below comment)
	# We transform https://*/*.git URLs
	https://gitlab.com/repo-owner/zarf-1211668992.git
	# Even URLs with things on either side
	stuffhttps://gitlab.com/repo-owner/zarf-1211668992.gitandthings
	# But not ssh://*/*.git URLs
	ssh://git@github.com/defenseunicorns/zarf.git
	# Or non .git URLs
	https://www.defenseunicorns.com/
	`

	resultingText := MutateGitURLsInText("https://gitlab.com", originalText, "repo-owner")
	assert.Equal(t, expectedText, resultingText)
}

func TestGitTransformURLSplitRef(t *testing.T) {
	var expectedResult = [][]string{
		// Normal git repos and references for pushing/pulling
		{"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git", ""},
		{"https://github.com/defenseunicorns/zarf.git", ""},
		{"https://ghcr.io/stefanprodan/podinfo_fasd-123.git", ""},
		{"git://k3d-cluster.localhost/defenseunicorns/zarf-agent", ""},
		{"http://localhost:5000/some-cool-repo", ""},
		{"ssh://ghcr.io/stefanprodan/podinfo", "6.0.0"},
		{"https://stefanprodan/podinfo.git", "adf0fasd10.1.223124123123-asdf"},
		{"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git", "0.0.9-bb.0"},
		{"file:///srv/git/stefanprodan/podinfo.git", "adf0fasd10.1.223124123123-asdf"},
		{"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test", ""},
		{"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test", "524980951ff16e19dc25232e9aea8fd693989ba6"},
		{"https://github.com/defenseunicorns/zarf.helm.git", ""},
		{"https://github.com/defenseunicorns/zarf.git", "refs/tags/v0.16.0"},
		{"https://github.com/DoD-Platform-One/big-bang.git", "refs/heads/release-1.54.x"},

		// Smart Git Protocol URLs for proxying (https://www.git-scm.com/docs/http-protocol)
		{"https://github.com/defenseunicorns/zarf.helm.git", ""},
		{"https://github.com/defenseunicorns/zarf.helm.git", ""},
		{"https://github.com/defenseunicorns/zarf.helm.git", ""},
		{"https://github.com/defenseunicorns/zarf.helm.git", ""},
		{"https://github.com/defenseunicorns/zarf.helm.git", ""},
	}

	for idx, url := range gitURLs {
		gitURLNoRef, refPlain, err := GitTransformURLSplitRef(url)
		assert.NoError(t, err)
		assert.Equal(t, expectedResult[idx][0], gitURLNoRef)
		assert.Equal(t, expectedResult[idx][1], refPlain)
	}

	for _, url := range badURLs {
		_, _, err := GitTransformURLSplitRef(url)
		assert.Error(t, err)
	}
}
