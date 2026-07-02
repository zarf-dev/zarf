package pipeline

import (
	"fmt"
	"regexp"
	"strings"
)

// Match double curly bois containing any whitespace, "matrix", then maybe
// a dot and a dimension name, ending with any whitespace and closing curlies.
var matrixTokenRE = regexp.MustCompile(`\{\{\s*matrix(\.[\w-\.]+)?\s*\}\}`)

// matrixInterpolator is a string transform that interpolates matrix tokens.
type matrixInterpolator struct {
	replacements map[string]string
}

// Transform interpolates matrix tokens.
func (m matrixInterpolator) Transform(src string) (string, error) {
	var unknown []string

	out := matrixTokenRE.ReplaceAllStringFunc(src, func(s string) string {
		sub := matrixTokenRE.FindStringSubmatch(s)
		repl, ok := m.replacements[sub[1]]
		if !ok {
			unknown = append(unknown, sub[1])
		}
		return repl
	})

	if len(unknown) > 0 {
		for i, f := range unknown {
			unknown[i] = "matrix" + f
		}
		return out, fmt.Errorf("unknown matrix tokens in input: %s", strings.Join(unknown, ", "))
	}
	return out, nil
}

// newMatrixInterpolator creates a reusable string transformer that applies
// matrix interpolation.
func newMatrixInterpolator(mp MatrixPermutation) matrixInterpolator {
	replacements := make(map[string]string)
	for dim, val := range mp {
		if dim == "" {
			replacements[""] = val
		} else {
			replacements["."+dim] = val
		}
	}

	return matrixInterpolator{
		replacements: replacements,
	}
}
