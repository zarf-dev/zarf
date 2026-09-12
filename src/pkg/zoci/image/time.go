// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import "time"

var (
	// static is very first commit in github.com/zarf-dev/zarf. Every
	// timestamp an image volume records comes from it, so the same source
	// tree always produces the same digests.
	static = time.Date(2021, time.April, 13, 18, 30, 43, 0, time.UTC)
	// staticRFC3339 is static rendered as RFC3339, the form the OCI
	// org.opencontainers.image.created annotation takes.
	staticRFC3339 = static.Format(time.RFC3339)
)
