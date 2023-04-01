// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"net/url"
)

func isLocal(source string) bool {
	parsedURL, err := url.Parse(source)
	if err == nil && parsedURL.Scheme == "file" {
		return true
	}
	return err == nil && parsedURL.Scheme == "" && parsedURL.Host == ""
}
