// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

package misc

import "net/url"

// JoinURL joins the two URLs while preserving the query and fragment part of the latter.
func JoinURL(baseURL *url.URL, relativeURL *url.URL) *url.URL {
	u := baseURL.JoinPath(relativeURL.Path)
	u.RawQuery = relativeURL.RawQuery
	u.RawFragment = relativeURL.RawFragment
	// Enforce https, this is required if the base url was only a domain
	u.Scheme = "https"
	return u
}
