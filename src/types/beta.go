// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// FeatureFlag is an enum of the different feature flags that can be set on a Zarf package.
type FeatureFlag string

const (
	// DefaultRequired changes the default state for all components in a package to be required.
	DefaultRequired FeatureFlag = "default-required"
)

// AllFeatureFlags returns a list of all available feature flags.
func AllFeatureFlags() []FeatureFlag {
	return []FeatureFlag{
		DefaultRequired,
	}
}
