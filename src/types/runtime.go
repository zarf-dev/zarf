// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	// Verify that Zarf should perform an action
	Confirm bool
	// Allow insecure connections for remote packages
	Insecure bool
	// Disable checking the server TLS certificate for validity
	InsecureSkipTLSVerify bool
	// Force connections to be over http instead of https
	PlainHTTP bool
	// Path to use to cache images and git repos on package create
	CachePath string
	// Location Zarf should use as a staging ground when managing files and images for package creation and deployment
	TempDirectory string
}
