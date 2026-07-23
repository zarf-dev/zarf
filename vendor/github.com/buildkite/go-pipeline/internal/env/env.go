// Package env contains data structures and methods to assist with managing environment variables.
package env

import (
	"runtime"
	"strings"
)

// Options are functional options for creating a new Env.
type Options func(*Env)

// CaseSensitive is an option that overrides previous case-sensitivity whether
// set by default or as an `Option`.
func CaseSensitive(actuallyCaseSensitive bool) Options {
	return func(e *Env) {
		e.caseInsensitive = !actuallyCaseSensitive
	}
}

// FromMap is an option that sets the Env to have the key-values pairs from the `source` map.
// The key-value pair will be inserted with the case sensitivity of the Env, which by default is
// case-insensitive on Windows and case-sensitive on other platforms.
// Note that random map iteration will cause the result to be non-deterministic if there are
// multiple keys in `source`, which are equivalent under case insensitivity, that have different
// corresponding values.
func FromMap(source map[string]string) Options {
	return func(e *Env) {
		if e.env == nil {
			e.env = make(map[string]string, len(source))
		}
		for k, v := range source {
			e.Set(k, v)
		}
	}
}

// Env represents a map of environment variables. By default, the keys are case-insensitive on
// Windows and case-sensitive on other platforms. If they are case-insensitive, the original casing
// is lost.
type Env struct {
	env             map[string]string
	caseInsensitive bool
}

// New return a new `Env`. By default, it is case-insensitive on Windows and case-sensitive on
// other platforms. See `Options` for available options.
func New(opts ...Options) *Env {
	e := &Env{
		caseInsensitive: runtime.GOOS == "windows",
	}
	for _, o := range opts {
		o(e)
	}
	if e.env == nil {
		e.env = make(map[string]string)
	}
	return e
}

// Set adds an environment variable to the Env or updates an existing one by overwriting its value.
// If the Env was created as case-insensitive, the keys are case normalised.
func (e *Env) Set(key, value string) {
	e.env[e.normaliseCase(key)] = value
}

// Get returns the value of an environment variable and whether it was found.
// If the Env was created as case-insensitive, the key's case is normalised.
func (e *Env) Get(key string) (string, bool) {
	if e == nil {
		return "", false
	}
	v, found := e.env[e.normaliseCase(key)]
	return v, found
}

func (e *Env) normaliseCase(key string) string {
	if e.caseInsensitive {
		return strings.ToUpper(key)
	}
	return key
}
