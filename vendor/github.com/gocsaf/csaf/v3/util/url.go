// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package util

import (
	"net/url"
	"strings"
)

// BaseURL returns the base URL for a given URL.
func BaseURL(u *url.URL) (string, error) {
	ep := u.EscapedPath()
	if idx := strings.LastIndexByte(ep, '/'); idx != -1 {
		ep = ep[:idx+1]
	}
	user := u.User.String()
	if user != "" {
		user += "@"
	}
	if !strings.HasPrefix(ep, "/") {
		ep = "/" + ep
	}
	return u.Scheme + "://" + user + u.Host + ep, nil
}
