// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

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

const (
	// OCIURLPrefix is the scheme prefix for OCI registry URLs.
	OCIURLPrefix = "oci://"
	// IPV4Localhost is the IPv4 loopback address.
	IPV4Localhost = "127.0.0.1"
)

// IsURL reports whether source is a URL with a scheme and host.
func IsURL(source string) bool {
	parsed, err := url.Parse(source)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

// IsOCIURL reports whether source uses the OCI URL scheme.
func IsOCIURL(source string) bool {
	parsed, err := url.Parse(source)
	return err == nil && parsed.Scheme == "oci"
}

// DoHostnamesMatch reports whether the hostnames in first and second match.
func DoHostnamesMatch(first, second string) (bool, error) {
	firstURL, err := url.Parse(first)
	if err != nil {
		return false, fmt.Errorf("unable to parse the url (%s): %w", first, err)
	}
	secondURL, err := url.Parse(second)
	if err != nil {
		return false, fmt.Errorf("unable to parse the url (%s): %w", second, err)
	}
	return firstURL.Hostname() == secondURL.Hostname(), nil
}

// ExtractBasePathFromURL returns the final path component of source.
func ExtractBasePathFromURL(source string) (string, error) {
	if !IsURL(source) {
		return "", fmt.Errorf("%s is not a valid URL", source)
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	return path.Base(parsed.Path), nil
}

// IsValidHostName reports whether the local hostname is valid and not localhost.
func IsValidHostName() bool {
	hostname, err := os.Hostname()
	if err != nil {
		return false
	}
	// Explanation: https://regex101.com/r/zUGqjP/1/
	return regexp.MustCompile(`^[a-zA-Z0-9\-.]+$`).MatchString(hostname) &&
		// Explanation: https://regex101.com/r/vPGnzR/1/
		!regexp.MustCompile(`\\.?localhost$`).MatchString(hostname)
}

// GetAvailablePort listens on an ephemeral TCP port and returns its port number.
func GetAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close() //nolint:errcheck
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(port)
}
