// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions
package utils

import (
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// ValidHostname returns a boolean indicating if the provided string is a valid hostname
func ValidHostname(hostname string) bool {
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

// IsValidHostName returns a boolean indicating if the system default hostname is valid.
func IsValidHostName() bool {
	message.Debug("Preflight check: validating hostname")
	// Quick & dirty character validation instead of a complete RFC validation since the OS is already allowing it
	hostname, err := os.Hostname()

	if err != nil {
		return false
	}

	return ValidHostname(hostname)
}

// IsRHEL returns a boolean indicating if the system is running RHEL.
func IsRHEL() bool {
	return !InvalidPath("/etc/redhat-release")
}

// RunPreFlightChecks runs all established validation checks and exits the process if any fail.
func RunPreflightChecks() {
	if !IsValidHostName() {
		message.Fatal(nil, "Please ensure this hostname is valid according to https://www.ietf.org/rfc/rfc1123.txt.")
	}
}
