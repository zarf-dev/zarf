// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package proxy provides helper functions for the agent proxy
package proxy

import (
	"fmt"
	"hash/crc32"
	"net/url"
	"regexp"
	"strings"
)

const (
	// NoTransform is the URL prefix added to HTTP 3xx or text URLs that instructs Zarf not to transform on a subsequent request.
	NoTransform = "/zarf-3xx-no-transform"
)

// NoTransformTarget takes an address that Zarf should not transform, and removes the NoTransform prefix.
func NoTransformTarget(address string, path string) (*url.URL, error) {
	targetURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	targetURL.Path = strings.TrimPrefix(path, NoTransform)

	return targetURL, nil
}

// NpmTransformURL finds the npm API path on a given URL and transforms that to align with the offline registry.
func NpmTransformURL(baseURL string, reqURL string) (*url.URL, error) {
	// For further explanation: https://regex101.com/r/RRyazc/3
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L210
	npmURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)` +
		`(?P<npmPath>(\/(@[\w\.\-\~]+(\/|%2[fF]))?[\w\.\-\~]+(\/-\/([\w\.\-\~]+\/)?[\w\.\-\~]+\.[\w]+)?(\/-rev\/.+)?)|(\/-\/(npm|v1|user|package)\/.+))$`)

	return transformRegistryPath(baseURL, reqURL, npmURLRegex, "npmPath", "npm")
}

// PipTransformURL finds the pip API path on a given URL and transforms that to align with the offline registry.
func PipTransformURL(baseURL string, reqURL string) (*url.URL, error) {
	// For further explanation: https://regex101.com/r/lreZiD/1
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L210
	pipURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)(?P<pipPath>(\/(simple|files\/)[\/\w\-\.\?\=&%#]*?))?$`)

	return transformRegistryPath(baseURL, reqURL, pipURLRegex, "pipPath", "pypi")
}

// GenTransformURL finds the generic API path on a given URL and transforms that to align with the offline registry.
func GenTransformURL(packagesBaseURL string, reqURL string) (*url.URL, error) {
	// For further explanation: https://regex101.com/r/qcg6Gr/2
	genURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<host>.+?)(?P<port>:[0-9]+?)?(?P<startPath>\/[\w\-\.%]+?\/[\w\-\.%]+?)?(?P<midPath>\/.+?)??(?P<version>\/[\w\-\.%]+?)?(?P<package>\/[\w\-\.\?\=&%#]+?)$`)

	matches := genURLRegex.FindStringSubmatch(reqURL)
	idx := genURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		return nil, fmt.Errorf("unable to extract the genericPath from the url %s", reqURL)
	}

	packageName := matches[idx("startPath")]
	// NOTE: We remove the protocol, port and file name so that https://zarf.dev:443/package/package1.zip and http://zarf.dev/package/package2.zip
	// resolve to the same "folder" (as they would in real life)
	sanitizedURL := fmt.Sprintf("%s%s%s", matches[idx("host")], matches[idx("startPath")], matches[idx("midPath")])

	// Add crc32 hash of the url to the end of the package name
	table := crc32.MakeTable(crc32.IEEE)
	checksum := crc32.Checksum([]byte(sanitizedURL), table)
	packageName = fmt.Sprintf("%s-%d", strings.ReplaceAll(packageName, "/", ""), checksum)

	version := matches[idx("version")]
	if version == "" {
		version = matches[idx("package")]
	}

	// TODO: (@WSTARR) %s/api/packages/%s is very Gitea specific but with a config option this could be adapted to GitLab easily
	// transformedURL := fmt.Sprintf("%s/api/v4/projects/39799997/packages/generic/%s%s%s", baseURL, packageName, version, matches[idx("package")])
	transformedURL := fmt.Sprintf("%s/generic/%s%s%s", packagesBaseURL, packageName, version, matches[idx("package")])

	return url.Parse(transformedURL)
}

// transformRegistryPath transforms a given request path using a new base URL and regex.
// - pathGroup specifies the named group for the registry's URL path inside the regex (i.e. pipPath) and registryType specifies the registry type (i.e. pypi).
func transformRegistryPath(packagesBaseURL string, reqURL string, regex *regexp.Regexp, pathGroup string, registryType string) (*url.URL, error) {
	matches := regex.FindStringSubmatch(reqURL)
	idx := regex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		return nil, fmt.Errorf("unable to extract the %s from the url %s", pathGroup, reqURL)
	}

	// TODO: (@WSTARR) %s/api/packages/%s is very Gitea specific but with a config option this could be adapted to GitLab easily
	// transformedURL := fmt.Sprintf("%s/api/v4/projects/39799997/packages/%s%s", baseURL, regType, matches[idx("pipPath")])
	transformedURL := fmt.Sprintf("%s/%s%s", packagesBaseURL, registryType, matches[idx(pathGroup)])

	return url.Parse(transformedURL)
}
