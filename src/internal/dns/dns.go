// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package dns contains DNS related functionality.
package dns

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	// localClusterServiceRegex is used to match the local cluster service format:
	localClusterServiceRegex = regexp.MustCompile(`^(?P<name>[^\.]+)\.(?P<namespace>[^\.]+)\.svc\.cluster\.local$`)
)

// IsServiceURL returns true if the give url complies with the service url format.
func IsServiceURL(serviceURL string) bool {
	_, _, _, err := ParseServiceURL(serviceURL)
	return err == nil
}

// ParseServiceURL takes a serviceURL and parses it to find the service info for connecting to the cluster. The string is expected to follow the following format:
// Example serviceURL: http://{SERVICE_NAME}.{NAMESPACE}.svc.cluster.local:{PORT}.
func ParseServiceURL(serviceURL string) (string, string, int, error) {
	if serviceURL == "" {
		return "", "", 0, errors.New("service url cannot be empty")
	}
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return "", "", 0, err
	}
	if parsedURL.Port() == "" {
		return "", "", 0, errors.New("service url does not have a port")
	}
	remotePort, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return "", "", 0, err
	}
	matches := localClusterServiceRegex.FindStringSubmatch(parsedURL.Hostname())
	if len(matches) != 3 {
		return "", "", 0, fmt.Errorf("invalid service url %s", serviceURL)
	}
	return matches[2], matches[1], remotePort, nil
}

// Inspired from https://github.com/google/go-containerregistry/blob/098045d5e61ff426a61a0eecc19ad0c433cd35a9/pkg/name/registry.go

// Detect more complex forms of local references.
var reLocal = regexp.MustCompile(`.*\.local(?:host)?(?::\d{1,5})?$`)

// Detect the loopback IP (127.0.0.1)
var reLoopback = regexp.MustCompile(regexp.QuoteMeta("127.0.0.1"))

// Detect the loopback IPV6 (::1)
var reipv6Loopback = regexp.MustCompile(regexp.QuoteMeta("::1"))

func isRFC1918(URL string) bool {
	ipStr := strings.Split(URL, ":")[0]
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"} {
		_, block, _ := net.ParseCIDR(cidr) //nolint:errcheck
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// IsLocalhost returns whether or not a URL without an existing scheme is on localhost
func IsLocalhost(URL string) bool {
	if isRFC1918(URL) {
		return true
	}
	if strings.HasPrefix(URL, "localhost:") {
		return true
	}
	if reLocal.MatchString(URL) {
		return true
	}
	if reLoopback.MatchString(URL) {
		return true
	}
	if reipv6Loopback.MatchString(URL) {
		return true
	}
	return false
}
