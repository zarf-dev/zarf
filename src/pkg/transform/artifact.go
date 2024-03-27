// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/defenseunicorns/pkg/helpers"
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
func NpmTransformURL(targetBaseURL string, sourceURL string) (*url.URL, error) {
	// For further explanation: https://regex101.com/r/RRyazc/3
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L210
	npmURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)` +
		`(?P<npmPath>(\/(@[\w\.\-\~]+(\/|%2[fF]))?[\w\.\-\~]+(\/-\/([\w\.\-\~]+\/)?[\w\.\-\~]+\.[\w]+)?(\/-rev\/.+)?)|(\/-\/(npm|v1|user|package)\/.+))$`)

	return transformRegistryPath(targetBaseURL, sourceURL, npmURLRegex, "npmPath", "npm")
}

// PipTransformURL finds the pip API path on a given URL and transforms that to align with the offline registry.
func PipTransformURL(targetBaseURL string, sourceURL string) (*url.URL, error) {
	// For further explanation: https://regex101.com/r/lreZiD/2
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L267
	pipURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)(?P<pipPath>\/((simple|files\/)[\/\w\-\.\?\=&%#]*?)?)?$`)

	return transformRegistryPath(targetBaseURL, sourceURL, pipURLRegex, "pipPath", "pypi")
}

// GenTransformURL finds the generic API path on a given URL and transforms that to align with the offline registry.
func GenTransformURL(targetBaseURL string, sourceURL string) (*url.URL, error) {
	// For further explanation: https://regex101.com/r/bwMkCm/5
	// This regex was created with information from https://www.rfc-editor.org/rfc/rfc3986#section-2
	genURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<host>[a-zA-Z0-9\-\.]+)(?P<port>:[0-9]+?)?(?P<startPath>\/[\w\-\.+~%]+?\/[\w\-\.+~%]+?)?(?P<midPath>\/.+?)??(?P<version>\/[\w\-\.+~%]+?)??(?P<fileName>\/[\w\-\.+~%]*)?(?P<query>[\w\-\.\?\=,;+~!$'*&%#()\[\]]*?)?$`)

	matches := genURLRegex.FindStringSubmatch(sourceURL)
	idx := genURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		return nil, fmt.Errorf("unable to extract the genericPath from the url %s", sourceURL)
	}

	fileName := strings.ReplaceAll(matches[idx("fileName")], "/", "")
	if fileName == "" {
		fileName = matches[idx("host")]
	}

	// NOTE: We remove the protocol, port and file name so that https://zarf.dev:443/package/package1.zip and http://zarf.dev/package/package2.zip
	// resolve to the same "folder" (as they would in real life)
	sanitizedURL := fmt.Sprintf("%s%s%s", matches[idx("host")], matches[idx("startPath")], matches[idx("midPath")])

	packageName := strings.ReplaceAll(matches[idx("startPath")], "/", "")
	if packageName == "" {
		packageName = fileName
	}
	// Add crc32 hash of the url to the end of the package name
	packageNameGlobal := fmt.Sprintf("%s-%d", packageName, helpers.GetCRCHash(sanitizedURL))

	version := strings.ReplaceAll(matches[idx("version")], "/", "")
	if version == "" {
		version = fileName
	}

	// Rebuild the generic URL
	transformedURL := fmt.Sprintf("%s/generic/%s/%s/%s", targetBaseURL, packageNameGlobal, version, fileName)

	url, err := url.Parse(transformedURL)
	if err != nil {
		return url, err
	}

	// Drop the RawQuery and Fragment to avoid them being interpreted for generic packages
	url.RawQuery = ""
	url.Fragment = ""

	return url, err
}

// transformRegistryPath transforms a given request path using a new base URL and regex.
// - pathGroup specifies the named group for the registry's URL path inside the regex (i.e. pipPath) and registryType specifies the registry type (i.e. pypi).
func transformRegistryPath(targetBaseURL string, sourceURL string, regex *regexp.Regexp, pathGroup string, registryType string) (*url.URL, error) {
	matches := regex.FindStringSubmatch(sourceURL)
	idx := regex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		return nil, fmt.Errorf("unable to extract the %s from the url %s", pathGroup, sourceURL)
	}

	// Rebuild the URL based on registry type
	transformedURL := fmt.Sprintf("%s/%s%s", targetBaseURL, registryType, matches[idx(pathGroup)])

	return url.Parse(transformedURL)
}
