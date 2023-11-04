// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
)

// Nonstandard URL schemes or prefixes
const (
	OCIURLPrefix = "oci://"

	SGETURLPrefix = "sget://"
	SGETURLScheme = "sget"

	IPV4Localhost = "127.0.0.1"
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

// ExtractBasePathFromURL returns filename from URL string
func ExtractBasePathFromURL(urlStr string) (string, error) {
	if !IsURL(urlStr) {
		return "", fmt.Errorf("%s is not a valid URL", urlStr)
	}
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	filename := path.Base(parsedURL.Path)
	return filename, nil
}

// IsValidHostName returns a boolean indicating if the hostname of the host machine is valid according to https://www.ietf.org/rfc/rfc1123.txt.
func IsValidHostName() bool {
	// Quick & dirty character validation instead of a complete RFC validation since the OS is already allowing it
	hostname, err := os.Hostname()

	if err != nil {
		return false
	}

	return validHostname(hostname)
}

func validHostname(hostname string) bool {
	// Explanation: https://regex101.com/r/zUGqjP/1/
	rfcDomain := regexp.MustCompile(`^[a-zA-Z0-9\-.]+$`)
	// Explanation: https://regex101.com/r/vPGnzR/1/
	localhost := regexp.MustCompile(`\.?localhost$`)
	isValid := rfcDomain.MatchString(hostname)
	if isValid {
		isValid = !localhost.MatchString(hostname)
	}
	return isValid
}

// GetAvailablePort retrieves an available port on the host machine. This delegates the port selection to the golang net
// library by starting a server and then checking the port that the server is using.
func GetAvailablePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func(l net.Listener) {
		// ignore this error because it won't help us to tell the user
		_ = l.Close()
	}(l)

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return port, err
}
