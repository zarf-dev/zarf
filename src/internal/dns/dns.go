// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package dns contains DNS related functionality.
package dns

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
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
