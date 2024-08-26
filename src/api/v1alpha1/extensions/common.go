// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package extensions contains the types for all official extensions.
package extensions

// ZarfComponentExtensions is a struct that contains all the official extensions.
type ZarfComponentExtensions struct {
	// Configurations for installing Big Bang and Flux in the cluster.
	BigBang *BigBang `json:"bigbang,omitempty"`
}
