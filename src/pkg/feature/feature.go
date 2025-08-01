package feature

import (
	"fmt"
	"sync/atomic"
)

// Atoms wrapping the default and user-set collections of Features. These do not require mutexes because they will each
// be modified very few times, with the public API for each ensuring there only an empty set can be written to.
// Alternatively these could be sync.Maps but they're not written to enough where it matters.
// e.g. These atoms are write once, ready many (WORM).
var defaultFeatures = atomic.Value{} // map[Name]Feature
var userFeatures = atomic.Value{}    // map[Name]Feature

// Mode describes the two different ways that Features can be set. These are used as keys for All()'s return map.
type Mode string

var DefaultMode Mode = "default"
var UserMode Mode = "user"

type Name string
type Description string
type Enabled bool
type Since string
type Until string

type Stage string

var (
	Alpha      Stage = "alpha"
	Beta       Stage = "beta"
	GA         Stage = "ga"
	Deprecated Stage = "deprecated"
)

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
	// Stage describes what level of done-ness a feature is. TODO describe this better
	Stage `json:"stage,omitempty"`
}

// IsEnabled allows users to optimistically check for a feature. Useful for control flow. Any user-enabled or disabled
// features take precedence over the default setting.
func IsEnabled(name Name) bool {
	_, err := Get(name)
	return err == nil
}

// Set takes a slice of one or many flags, inserting the features onto user-configured features. If a feature name is
// provided that is already a part of the set, then Set will return an error.
// TODO: Should we allow users to call this multiple times even if we don't allow them to overwrite features?
func Set(features []Feature) error {
	// Ensure user features haven't been set
	m := AllUser()
	if m != nil && len(m) > 0 {
		return fmt.Errorf("user features have already been set")
	}
	userFeatures.Store(featuresToMap(features))
	return nil
}

// SetDefault takes a slice of one or many flags, inserting the features onto the default feature set. If
// a feature name is provided that is already a part of the set, then SetDefault will return an error. This function
// can only be called once.
func SetDefault(features []Feature) error {
	// Ensure default features haven't been set
	m := AllDefault()
	if m != nil && len(m) > 0 {
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
		return f, fmt.Errorf("feature not found: %s", name)
	}
	return f, nil
}

// GetUser takes a flag Name and returns the Feature struct from the user set.
func GetUser(name Name) (Feature, error) {
	f, ok := AllUser()[name]
	if !ok {
		return f, fmt.Errorf("feature not found: %s", name)
	}
	return f, nil
}

// All returns all flags from both Default and User.
func All() map[Mode]map[Name]Feature {
	m := make(map[Mode]map[Name]Feature)
	m[DefaultMode] = defaultFeatures.Load().(map[Name]Feature)
	m[UserMode] = userFeatures.Load().(map[Name]Feature)
	return m
}

// AllDefault returns all features with from the Default set for this version of Zarf.
func AllDefault() map[Name]Feature {
	return defaultFeatures.Load().(map[Name]Feature)
}

// AllUser returns all features that have been enabled by users.
func AllUser() map[Name]Feature {
	return userFeatures.Load().(map[Name]Feature)
}

func featuresToMap(fs []Feature) map[Name]Feature {
	m := make(map[Name]Feature)
	for _, f := range fs {
		m[f.Name] = f
	}
	return m
}

func init() {
	features := []Feature{
		// FIXME: Example feature
		// Owner: @zarf-maintainers
		{
			Name:        "foo",
			Description: "foo does the thing of course",
			Enabled:     true,
			Since:       "v0.60.0",
			Stage:       GA,
		},
		// FIXME: Example feature
		// Owner: @zarf-maintainers
		{
			Name:        "bar",
			Description: "bar was honestly always a bit buggy, use baz instead",
			Enabled:     false,
			Since:       "v0.52.0",
			Until:       "v0.62.0",
			Stage:       Deprecated,
		},
	}

	err := SetDefault(features)
	if err != nil {
		panic(err)
	}
}
