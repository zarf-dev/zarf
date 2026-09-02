// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package url

import (
	"net/url"
)

func parse(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
}
