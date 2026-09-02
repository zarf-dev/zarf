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

// Originally inspired from https://github.com/google/go-containerregistry/blob/098045d5e61ff426a61a0eecc19ad0c433cd35a9/pkg/name/registry.go

// IsLocalOrPrivate reports whether hostURL (without a scheme) refers to a host
// reachable without a public network round-trip - loopback/localhost, a
// private-network IP (RFC1918/RFC4193), or a .local/.localhost domain - used for
// choosing to access services over plain HTTP. hostURL may include a port.
func IsLocalOrPrivate(hostURL string) bool {
	host := hostURL
	if h, _, err := net.SplitHostPort(hostURL); err == nil {
		host = h
	}
	if IsLocalhost(host) {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsPrivate() {
		return true
	}
	return strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localhost")
}

// IsLocalhost reports whether host is the localhost hostname or a loopback IP
// (127.0.0.0/8 or ::1). host must be a bare hostname or IP, without a port.
func IsLocalhost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
