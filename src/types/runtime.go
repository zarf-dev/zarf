// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains types used globally throughout Zarf
package types

// RemoteOptions are common options when calling a remote service
type RemoteOptions struct {
	PlainHTTP             bool
	InsecureSkipTLSVerify bool
}

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	// Path to use to cache images and git repos on package create
	CachePath string
	// Location Zarf should use as a staging ground when managing files and images for package creation and deployment
	TempDirectory string
	// Whether to prefer using the structured logger over printing to stdout/stderr (i.e. in actions or git repo pulls)
	PreferLogger bool
}
