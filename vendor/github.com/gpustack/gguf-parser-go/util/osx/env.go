package osx

import (
	"os"
)

// ExistEnv checks if the environment variable named by the key exists.
func ExistEnv(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}

// Getenv retrieves the value of the environment variable named by the key.
// It returns the default, which will be empty if the variable is not present.
// To distinguish between an empty value and an unset value, use LookupEnv.
func Getenv(key string, def ...string) string {
	e, ok := os.LookupEnv(key)
	if !ok && len(def) != 0 {
		return def[0]
	}

	return e
}

// ExpandEnv is similar to Getenv,
// but replaces ${var} or $var in the result.
func ExpandEnv(key string, def ...string) string {
	return os.ExpandEnv(Getenv(key, def...))
}
