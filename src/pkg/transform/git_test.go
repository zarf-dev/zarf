// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var gitURLs = []string{
	// Normal git repos and references for pushing/pulling
	"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git",
	"https://github.com/zarf-dev/zarf.git",
	"https://ghcr.io/stefanprodan/podinfo_fasd-123.git",
	"git://k3d-cluster.localhost/zarf-dev/zarf-agent",
	"http://localhost:5000/some-cool-repo",
	"ssh://ghcr.io/stefanprodan/podinfo@6.0.0",
	"https://stefanprodan/podinfo.git@adf0fasd10.1.223124123123-asdf",
	"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git@0.0.9-bb.0",
	"file:///srv/git/stefanprodan/podinfo@adf0fasd10.1.223124123123-asdf",
	"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test",
	"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test@524980951ff16e19dc25232e9aea8fd693989ba6",
	"https://github.com/zarf-dev/zarf.helm.git",
	"https://github.com/zarf-dev/zarf.git@refs/tags/v0.16.0",
	"https://github.com/DoD-Platform-One/big-bang.git@refs/heads/release-1.54.x",
	"https://github.com/prometheus-community/helm-charts.git@kube-prometheus-stack-47.3.0",
	"https://github.com/prometheus-community/",
	"https://github.com/",

	// Smart Git Protocol URLs for proxying (https://www.git-scm.com/docs/http-protocol)
	"https://github.com/zarf-dev/zarf.helm.git/info/refs",
	"https://github.com/zarf-dev/zarf.helm.git/info/refs?service=git-upload-pack",
	"https://github.com/zarf-dev/zarf.helm.git/info/refs?service=git-receive-pack",
	"https://github.com/zarf-dev/zarf.helm.git/git-upload-pack",
	"https://github.com/zarf-dev/zarf.helm.git/git-receive-pack",
}

var badGitURLs = []string{
	"i am not a url at all",
	"C:\\Users\\zarf",
}

func TestMutateGitURLsInText(t *testing.T) {
	dummyLogger := func(_ string, _ ...any) {}
	originalText := `
	# Here we handle invalid URLs (see below comment)
	# We transform https://*/*.git URLs
	https://github.com/zarf-dev/zarf.git
	# Even URLs with things on either side
	stuff https://github.com/zarf-dev/zarf.git andthings
	# Including ssh://*/*.git URLs
	ssh://git@github.com/zarf-dev/zarf.git
	# Or non .git URLs
	https://www.defenseunicorns.com/
	`

	expectedText := `
	# Here we handle invalid URLs (see below comment)
	# We transform https://*/*.git URLs
	https://gitlab.com/repo-owner/zarf-4156197301.git
	# Even URLs with things on either side
	stuff https://gitlab.com/repo-owner/zarf-4156197301.git andthings
	# Including ssh://*/*.git URLs
	https://gitlab.com/repo-owner/zarf-1231196790.git
	# Or non .git URLs
	https://www.defenseunicorns.com/
	`

	resultingText := MutateGitURLsInText(dummyLogger, "https://gitlab.com", originalText, "repo-owner")
	require.Equal(t, expectedText, resultingText)
}

func TestGitURLSplitRef(t *testing.T) {
	var expectedResult = [][]string{
		// Normal git repos and references for pushing/pulling
		{"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git", ""},
		{"https://github.com/zarf-dev/zarf.git", ""},
		{"https://ghcr.io/stefanprodan/podinfo_fasd-123.git", ""},
		{"git://k3d-cluster.localhost/zarf-dev/zarf-agent", ""},
		{"http://localhost:5000/some-cool-repo", ""},
		{"ssh://ghcr.io/stefanprodan/podinfo", "6.0.0"},
		{"https://stefanprodan/podinfo.git", "adf0fasd10.1.223124123123-asdf"},
		{"https://repo1.dso.mil/platform-one/big-bang/apps/security-tools/twistlock.git", "0.0.9-bb.0"},
		{"file:///srv/git/stefanprodan/podinfo", "adf0fasd10.1.223124123123-asdf"},
		{"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test", ""},
		{"https://me0515@dev.azure.com/me0515/zarf-public-test/_git/zarf-public-test", "524980951ff16e19dc25232e9aea8fd693989ba6"},
		{"https://github.com/zarf-dev/zarf.helm.git", ""},
		{"https://github.com/zarf-dev/zarf.git", "refs/tags/v0.16.0"},
		{"https://github.com/DoD-Platform-One/big-bang.git", "refs/heads/release-1.54.x"},
		{"https://github.com/prometheus-community/helm-charts.git", "kube-prometheus-stack-47.3.0"},
		{"https://github.com/prometheus-community", ""},
		{"https://github.com/", ""},

		// Smart Git Protocol URLs for proxying (https://www.git-scm.com/docs/http-protocol)
		{"https://github.com/zarf-dev/zarf.helm.git", ""},
		{"https://github.com/zarf-dev/zarf.helm.git", ""},
		{"https://github.com/zarf-dev/zarf.helm.git", ""},
		{"https://github.com/zarf-dev/zarf.helm.git", ""},
		{"https://github.com/zarf-dev/zarf.helm.git", ""},
	}

	for idx, url := range gitURLs {
		gitURLNoRef, refPlain, err := GitURLSplitRef(url)
		require.NoError(t, err)
		require.Equal(t, expectedResult[idx][0], gitURLNoRef)
		require.Equal(t, expectedResult[idx][1], refPlain)
	}

	for _, url := range badGitURLs {
		_, _, err := GitURLSplitRef(url)
		require.Error(t, err)
	}
}

func TestGitURLtoFolderName(t *testing.T) {
	var expectedResult = []string{
		// Normal git repos and references for pushing/pulling
		"twistlock-1590638614",
		"zarf-3457133088",
		"podinfo_fasd-123-1478387306",
		"zarf-agent-927663661",
		"some-cool-repo-1916670310",
		"podinfo-1350532569",
		"podinfo-1853010387",
		"twistlock-1920149257",
		"podinfo-122075437",
		"zarf-public-test-612413317",
		"zarf-public-test-634307705",
		"zarf.helm-93697844",
		"zarf-2360044954",
		"big-bang-2705706079",
		"helm-charts-1319967699",
		"prometheus-community-3453166319",
		"-1276058275",

		// Smart Git Protocol URLs for proxying (https://www.git-scm.com/docs/http-protocol)
		"zarf.helm-93697844",
		"zarf.helm-93697844",
		"zarf.helm-93697844",
		"zarf.helm-93697844",
		"zarf.helm-93697844",
	}

	for idx, url := range gitURLs {
		repoFolder, err := GitURLtoFolderName(url)
		require.NoError(t, err)
		require.Equal(t, expectedResult[idx], repoFolder)
	}

	for _, url := range badGitURLs {
		_, err := GitURLtoFolderName(url)
		require.Error(t, err)
	}
}

func TestGitURLtoRepoName(t *testing.T) {
	var expectedResult = []string{
		// Normal git repos and references for pushing/pulling
		"twistlock-97328248",
		"zarf-4156197301",
		"podinfo_fasd-123-84577122",
		"zarf-agent-1776579160",
		"some-cool-repo-926913879",
		"podinfo-2985051089",
		"podinfo-2197246515",
		"twistlock-97328248",
		"podinfo-1175499642",
		"zarf-public-test-2170732467",
		"zarf-public-test-2170732467",
		"zarf.helm-693435256",
		"zarf-4156197301",
		"big-bang-2366614037",
		"helm-charts-3648076006",
		"prometheus-community-2749132599",
		"-98306241",

		// Smart Git Protocol URLs for proxying (https://www.git-scm.com/docs/http-protocol)
		"zarf.helm-693435256",
		"zarf.helm-693435256",
		"zarf.helm-693435256",
		"zarf.helm-693435256",
		"zarf.helm-693435256",
	}

	for idx, url := range gitURLs {
		repoName, err := GitURLtoRepoName(url)
		require.NoError(t, err)
		require.Equal(t, expectedResult[idx], repoName)
	}

	for _, url := range badGitURLs {
		_, err := GitURLtoRepoName(url)
		require.Error(t, err)
	}
}

func TestGitURL(t *testing.T) {
	var expectedResult = []string{
		// Normal git repos and references for pushing/pulling
		"https://gitlab.com/repo-owner/twistlock-97328248.git",
		"https://gitlab.com/repo-owner/zarf-4156197301.git",
		"https://gitlab.com/repo-owner/podinfo_fasd-123-84577122.git",
		"https://gitlab.com/repo-owner/zarf-agent-1776579160",
		"https://gitlab.com/repo-owner/some-cool-repo-926913879",
		"https://gitlab.com/repo-owner/podinfo-2985051089",
		"https://gitlab.com/repo-owner/podinfo-2197246515.git",
		"https://gitlab.com/repo-owner/twistlock-97328248.git",
		"https://gitlab.com/repo-owner/podinfo-1175499642",
		"https://gitlab.com/repo-owner/zarf-public-test-2170732467",
		"https://gitlab.com/repo-owner/zarf-public-test-2170732467",
		"https://gitlab.com/repo-owner/zarf.helm-693435256.git",
		"https://gitlab.com/repo-owner/zarf-4156197301.git",
		"https://gitlab.com/repo-owner/big-bang-2366614037.git",
		"https://gitlab.com/repo-owner/helm-charts-3648076006.git",
		"https://gitlab.com/repo-owner/prometheus-community-2749132599",
		"https://gitlab.com/repo-owner/-98306241",

		// Smart Git Protocol URLs for proxying (https://www.git-scm.com/docs/http-protocol)
		"https://gitlab.com/repo-owner/zarf.helm-693435256.git/info/refs",
		"https://gitlab.com/repo-owner/zarf.helm-693435256.git/info/refs?service=git-upload-pack",
		"https://gitlab.com/repo-owner/zarf.helm-693435256.git/info/refs?service=git-receive-pack",
		"https://gitlab.com/repo-owner/zarf.helm-693435256.git/git-upload-pack",
		"https://gitlab.com/repo-owner/zarf.helm-693435256.git/git-receive-pack",
	}

	for idx, url := range gitURLs {
		repoURL, err := GitURL("https://gitlab.com", url, "repo-owner")
		require.NoError(t, err)
		require.Equal(t, expectedResult[idx], repoURL.String())
	}

	for _, url := range badGitURLs {
		_, err := GitURL("https://gitlab.com", url, "repo-owner")
		require.Error(t, err)
	}
}
