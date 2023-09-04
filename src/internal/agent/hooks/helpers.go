// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks provides HTTP handlers for the mutating webhook.
package hooks

import (
	"errors"
	"strings"
)

func removeOCIProtocol(input string) (string, error) {
	if strings.HasPrefix(input, "oci://") {
		return strings.TrimPrefix(input, "oci://"), nil
	}
	return "", errors.New("URL does not start with 'oci://'")
}
