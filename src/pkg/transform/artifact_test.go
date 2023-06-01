// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoTransformTarget(t *testing.T) {
	// Properly removes the NoTransform target
	newURL, err := NoTransformTarget("https://gitlab.com", NoTransform+"/some-path/without-query")
	require.NoError(t, err)
	require.Equal(t, "https://gitlab.com/some-path/without-query", newURL.String())

	// Passes through without the NoTransform target
	newURL, err = NoTransformTarget("https://gitlab.com", "/some-path/without-query")
	require.NoError(t, err)
	require.Equal(t, "https://gitlab.com/some-path/without-query", newURL.String())

	// Returns an error when given a bad base url
	_, err = NoTransformTarget("https*://gitlab.com", "/some-path/without-query")
	require.Error(t, err)
}

func TestNpmTransformURL(t *testing.T) {
	protocolPaths := []string{
		"/npm",
		"/npm/-rev/undefined",
		"/npm/-/8.19.2/npm-8.19.2.tgz",
		"/npm/-/8.19.2/npm-8.19.2.tgz/-rev/undefined",
		"/lodash",
		"/lodash/-rev/1",
		"/lodash/-/4.17.21/lodash-4.17.21.tgz",
		"/lodash/-/4.17.21/lodash-4.17.21.tgz/-rev/1",
		"/@types/node",
		"/@types%2fnode",
		"/@types%2Fnode",
		"/@types/node/-rev/100",
		"/@types%2fnode/-rev/100",
		"/@types%2Fnode/-rev/100",
		"/@types/node/-/18.11.2/types-node-18.11.2.tgz",
		"/@types%2fnode/-/18.11.2/types-node-18.11.2.tgz",
		"/@types%2Fnode/-/18.11.2/types-node-18.11.2.tgz",
		"/@types/node/-/18.11.2/types-node-18.11.2.tgz/-rev/100",
		"/-/package/lodash/dist-tags",
		"/-/package/lodash/dist-tags/latest",
		"/-/package/@types/node/dist-tags",
		"/-/package/@types/node/dist-tags/release",
		"/-/npm/v1/security/advisories/bulk",
		"/-/npm/v1/security/audits/quick",
		"/-/user/org.couchdb.user:username",
		"/-/v1/login",
		"/-/v1/search",
	}

	protocolHosts := []string{
		"https://git.privatemirror.com/api/packages/zarf-mirror-user/npm",
		"https://registry.npmjs.org",
	}

	for _, host := range protocolHosts {
		for _, path := range protocolPaths {
			newURL, err := NpmTransformURL("https://gitlab.com/project", host+path)
			require.NoError(t, err)
			// For each host/path swap them and add `npm` for compatibility with Gitea/Gitlab
			require.Equal(t, "https://gitlab.com/project/npm"+path, newURL.String())
		}
	}

	// Returns an error when given a bad base url
	_, err := NpmTransformURL("https*://gitlab.com/project", "https://registry.npmjs.org/npm")
	require.Error(t, err)
}

func TestPipTransformURL(t *testing.T) {
	protocolPaths := []string{
		"",
		"/",
		"/simple",
		"/simple/",
		"/simple/numpy",
		"/simple/numpy/",
		"/files/numpy/1.23.4/numpy-1.23.4-pp38-pypy38_pp73-win_amd64.whl#sha256-4d52914c88b4930dafb6c48ba5115a96cbab40f45740239d9f4159c4ba779962",
	}

	protocolHosts := []string{
		"https://git.privatemirror.com/api/packages/zarf-mirror-user/pip",
		"https://pypi.org",
	}

	for _, host := range protocolHosts {
		for _, path := range protocolPaths {
			newURL, err := PipTransformURL("https://gitlab.com/project", host+path)
			require.NoError(t, err)
			// For each host/path swap them and add `pypi` for compatibility with Gitea/Gitlab
			require.Equal(t, "https://gitlab.com/project/pypi"+path, newURL.String())
		}
	}

	// Returns an error when given a bad base url
	_, err := PipTransformURL("https*://gitlab.com/project", "https://pypi.org/simple")
	require.Error(t, err)
}

func TestGenTransformURL(t *testing.T) {
	urls := []string{
		"https://git.example.com/api/packages/zarf-git-user/generic",
		"https://git.example.com/api/packages/zarf-git-user/generic/packageVersion/packageName",
		"https://git.example.com/api/packages/zarf-git-user/generic/package+name/packageVersion/filename",
		"https://git.example.com/api/packages/zarf-git-user/generic/some%20generic%20package/0.0.1/superGeneric.tar.gz",
		"https://git.example.com/archive.zip",
		"https://git.example.com:443/archive.zip",
		"https://git.example.com/api/packages/username/generic/some-package-name/file.1.4.4.tar.gz",
		"https://git.example.com/facebook/zstd/releases/download/v1.4.4/zstd-1.4.4.tar.gz",
		"https://some.host.com/mirror/some-package-release/github.com/baselbuild/bazel-toolchains/archive/someshasum9318490392.tar.gz",
		"https://some.host.com/mirror/some-package-release/some-org/some-library/archive/refs/tags/file.zip",
		"https://i.am.a.weird.php.thing:8080/~/and/rfc/3986/loves/me?array[]=1&array[]=2&pie=(1)",
		"http://why.microsoft/did/you/do/this/Foo.aspx?id=1,2,3,4",
		"http://this.is.legal.too/according/to/spec/?world=!$;,*'",
		"http://i.end.in.nothing.com",
		"http://i.end.in.slash.com/",
	}

	expectedURLs := []string{
		// We want most of these to exist in the form of /project/generic/packageName/version/filename
		"https://gitlab.com/project/generic/apipackages-3151594639/zarf-git-user/generic",
		"https://gitlab.com/project/generic/apipackages-2561175711/packageVersion/packageName",
		"https://gitlab.com/project/generic/apipackages-2265319408/packageVersion/filename",
		"https://gitlab.com/project/generic/apipackages-4040139506/0.0.1/superGeneric.tar.gz",
		"https://gitlab.com/project/generic/archive.zip-2052577494/archive.zip/archive.zip",
		"https://gitlab.com/project/generic/archive.zip-2052577494/archive.zip/archive.zip",
		"https://gitlab.com/project/generic/apipackages-764706626/some-package-name/file.1.4.4.tar.gz",
		"https://gitlab.com/project/generic/facebookzstd-1475713874/v1.4.4/zstd-1.4.4.tar.gz",
		"https://gitlab.com/project/generic/mirrorsome-package-release-1448769245/archive/someshasum9318490392.tar.gz",
		"https://gitlab.com/project/generic/mirrorsome-package-release-2849990407/tags/file.zip",
		"https://gitlab.com/project/generic/~and-1138178232/loves/me",
		"https://gitlab.com/project/generic/didyou-2239567090/this/Foo.aspx",
		"https://gitlab.com/project/generic/accordingto-29577836/spec/this.is.legal.too",
		"https://gitlab.com/project/generic/i.end.in.nothing.com-2766891503/i.end.in.nothing.com/i.end.in.nothing.com",
		"https://gitlab.com/project/generic/i.end.in.slash.com-1566625415/i.end.in.slash.com/i.end.in.slash.com",
	}

	for idx, url := range urls {
		newURL, err := GenTransformURL("https://gitlab.com/project", url)
		require.NoError(t, err)
		// For each host/path swap them and add `pypi` for compatibility with Gitea/Gitlab
		require.Equal(t, expectedURLs[idx], newURL.String())
	}

	// Returns an error when given a bad base url
	_, err := GenTransformURL("https*://gitlab.com/project", "http://i.end.in.nothing.com")
	require.Error(t, err)
}
