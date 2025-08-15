// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package feature provides feature flags.
package feature

import (
	"fmt"
	"maps"
	"sync/atomic"
)

// Atoms wrapping the default and user-set collections of Features. These do not require mutexes because they will each
// be modified very few times, with the public API for each ensuring that only an empty set can be written to.
// Alternatively these could be sync.Maps but they're not written to enough where it matters.
// e.g. These atoms are write once, ready many (WORM).
var defaultFeatures = atomic.Value{} // map[Name]Feature
var userFeatures = atomic.Value{}    // map[Name]Feature

// Mode describes the two different ways that Features can be set. These are used as keys for All()'s return map.
type Mode string

// Default identifies features from Zarf's system defaults.
var Default Mode = "default"

// User identifies user-specified features.
var User Mode = "user"

// Name describes the canonical identifier for the feature. It must be globally unique across all features for the full
// lifespan of the Zarf project. Once created, names are considered immutable and may not be removed or redacted. Should
// a feature evolve enough to require renaming, then a new feature can be created and the original marked as Deprecated.
// The Deprecated feature's Description should provide context (like the ZEP or ADR) and point users to the new feature.
type Name string

// Description is an explanation of the feature, what to expect when it's enabled or disabled, the associated proposal,
// and any commentary or context appropriate for its stage. Descirptions are mutable, and are intended to be updated
// throughout the feature's development lifecycle.
type Description string

// Enabled describes the state of the feature. In cases where the Default feature state and User feature states do not
// match, User feature state takes precedence.
type Enabled bool

// Since marks the Zarf version that a feature is first released in Alpha. By convention Since is semver starting with
// "v": "v1.0.0".
type Since string

// Until marks the intended Zarf version that a Deprecated feature will or has been fully removed. If Deprecation must
// be delayed, then this version be updated. By convention Until is semver starting with "v": "v1.0.0".
type Until string

// Stage describes the lifecycle of a feature, as well as its production-readiness.
type Stage string

var (
	// Alpha features are experimental and are highly subject to change.
	Alpha Stage = "alpha"
	// Beta features have solidified into a release candidate and are ready for both user feedback and realistic
	// workloads. Beta features are open to change before release but should be considered nearly complete.
	Beta Stage = "beta"
	// GA features have been fully released and are available for production usage.
	GA Stage = "ga"
	// Deprecated features wrap functionality that is intended to be removed in a future version. They will start as
	// enabled by default with a warning, and eventually be disabled by default with the Until field documenting when
	// the feature is expected to be fully removed. Even after the feature is, Deprecated feature structs should be
	// kept for documentation purposes.
	Deprecated Stage = "deprecated"
)

// Feature models a Default or User-configured feature flag and its metadata.
type Feature struct {
	// Name stores the name of the feature flag.
	Name `json:"name,omitempty"`
	// Description describes how the flag is used.
	Description `json:"description,omitempty"`
	// Enabled describes whether a feature is explicitly enabled or disabled. A feature that does not exist in any set
	// is considered disabled.
	Enabled `json:"enabled,omitempty"`
	// Since is the version a feature is first introduced in alpha stage.
	Since `json:"since,omitempty"`
	// Until is the version when a deprecated feature is fully removed. Historical versions included.
	Until `json:"until,omitempty"`
	Stage `json:"stage,omitempty"`
}

func (f Feature) String() string {
	s := "disabled"
	if f.Enabled {
		s = "enabled"
	}
	return fmt.Sprintf("%s:%s", f.Name, s)
}

// IsEnabled allows users to optimistically check for a feature. Useful for control flow. Any user-enabled or disabled
// features take precedence over the default setting.
func IsEnabled(name Name) bool {
	f, err := Get(name)
	if err != nil {
		// We don't actually need to check the error here because an empty f will have the same truthiness
		return false
	}
	return bool(f.Enabled)
}

// Set takes a slice of one or many flags, inserting the features onto user-configured features. If a feature name is
// provided that is already a part of the set, then Set will return an error.
func Set(features []Feature) error {
	// Ensure user features haven't been set
	m := AllUser()
	if len(m) > 0 {
		return fmt.Errorf("user features have already been set")
	}
	userFeatures.Store(featuresToMap(features))
	return nil
}

// setDefault takes a slice of one or many flags, inserting the features onto the default feature set. If
// a feature name is provided that is already a part of the set, then SetDefault will return an error. This function
// can only be called once.
func setDefault(features []Feature) error {
	// Ensure default features haven't been set
	m := AllDefault()
	if len(m) > 0 {
		return fmt.Errorf("default features have already been set")
	}
	defaultFeatures.Store(featuresToMap(features))
	return nil
}

// Get takes a flag Name and returns the Feature struct. If the doesn't exist then it will error. It will check both the
// default set and the user set, and if a flag exists in both it will return the user data for it.
func Get(name Name) (Feature, error) {
	// Get from user set
	fu, uErr := GetUser(name)
	if uErr == nil {
		return fu, nil
	}

	// Fallback to default set
	fd, dErr := GetDefault(name)
	if dErr == nil {
		return fd, nil
	}

	// Feature not found in either set
	return Feature{}, fmt.Errorf("feature not found: %s", name)
}

// GetDefault takes a flag Name and returns the Feature struct from the default set.
func GetDefault(name Name) (Feature, error) {
	f, ok := AllDefault()[name]
	if !ok {
		return f, fmt.Errorf("default feature not found: %s", name)
	}
	return f, nil
}

// GetUser takes a flag Name and returns the Feature struct from the user set.
func GetUser(name Name) (Feature, error) {
	f, ok := AllUser()[name]
	if !ok {
		return f, fmt.Errorf("user-configured feature not found: %s", name)
	}
	return f, nil
}

// All returns all flags from both Default and User.
func All() map[Mode]map[Name]Feature {
	m := make(map[Mode]map[Name]Feature)
	m[Default] = AllDefault()
	m[User] = AllUser()
	return m
}

// AllDefault returns all features from the Default set for this version of Zarf.
func AllDefault() map[Name]Feature {
	m, ok := defaultFeatures.Load().(map[Name]Feature)
	// Default set is nil, so it's empty
	if !ok {
		return map[Name]Feature{}
	}
	return maps.Clone(m)
}

// AllUser returns all features that have been enabled by users.
func AllUser() map[Name]Feature {
	m, ok := userFeatures.Load().(map[Name]Feature)
	// User set is nil, so it's empty
	if !ok {
		return map[Name]Feature{}
	}
	return maps.Clone(m)
}

func featuresToMap(fs []Feature) map[Name]Feature {
	m := make(map[Name]Feature)
	for _, f := range fs {
		m[f.Name] = f
	}
	return m
}

// List of feature names
var (
	// AxolotlMode declares the "axolotl-mode" feature
	AxolotlMode Name = "axolotl-mode"
)

func init() {
	features := []Feature{
		// NOTE: Here is an example default feature flag
		// // Owner: @zarf-maintainers
		// {
		// 	Name:        "foo",
		// 	Description: "foo does the thing of course",
		// 	Enabled:     true,
		// 	Since:       "v0.60.0",
		// 	Stage:       GA,
		// },
		{
			Name: AxolotlMode,
			Description: "Enabling \"axolotl-mode\" runs `zarf say` at the beginning of each CLI command." +
				"This fun feature is intended to help with testing feature flags.",
			Enabled: false,
			Since:   "v0.60.0",
			Stage:   Alpha,
		},
	}

	err := setDefault(features)
	if err != nil {
		panic(err)
	}
}
