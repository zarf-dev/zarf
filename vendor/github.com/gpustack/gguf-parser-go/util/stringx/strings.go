package stringx

import "strings"

// CutFromLeft is the same as strings.Cut,
// which starts from left to right,
// slices s around the first instance of sep,
// returning the text before and after sep.
// The found result reports whether sep appears in s.
// If sep does not appear in s, cut returns s, "", false.
func CutFromLeft(s, sep string) (before, after string, found bool) {
	return strings.Cut(s, sep)
}

// CutFromRight takes the same arguments as CutFromLeft,
// but starts from right to left,
// slices s around the last instance of sep,
// return the text before and after sep.
// The found result reports whether sep appears in s.
// If sep does not appear in s, cut returns s, "", false.
func CutFromRight(s, sep string) (before, after string, found bool) {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

// ReplaceAllFunc is similar to strings.ReplaceAll,
// but it replaces each rune in s with the result of f(r).
func ReplaceAllFunc(s string, f func(rune) rune) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(f(r))
	}
	return b.String()
}

// HasSuffixes checks if s has any of the suffixes in prefixes.
func HasSuffixes(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}
