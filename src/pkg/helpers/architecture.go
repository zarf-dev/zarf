// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helpers

import "runtime"

// ResolveArch returns the first nonempty architecture, falling back to the host.
func ResolveArch(archs ...string) string {
	for _, arch := range archs {
		if arch != "" {
			return arch
		}
	}
	return runtime.GOARCH
}
