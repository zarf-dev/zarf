// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"fmt"
	"net/url"
)

// Nonstandard URL schemes or prefixes
const (
	OCIURLScheme = "oci"
	OCIURLPrefix = "oci://"
)

// IsURL is a helper function to check if a URL is valid.
func IsURL(source string) bool {
	parsedURL, err := url.Parse(source)
	return err == nil && parsedURL.Scheme != "" && parsedURL.Host != ""
}

// IsOCIURL returns true if the given URL is an OCI URL.
func IsOCIURL(source string) bool {
	parsedURL, err := url.Parse(source)
	return err == nil && parsedURL.Scheme == "oci"
}

// DoHostnamesMatch returns a boolean indicating if the hostname of two different URLs are the same.
func DoHostnamesMatch(url1 string, url2 string) (bool, error) {
	parsedURL1, err := url.Parse(url1)
	if err != nil {
		return false, fmt.Errorf("unable to parse the url (%s): %w", url1, err)
	}
	parsedURL2, err := url.Parse(url2)
	if err != nil {
		return false, fmt.Errorf("unable to parse the url (%s): %w", url2, err)
	}

	return parsedURL1.Hostname() == parsedURL2.Hostname(), nil
}
