package spdxexp

import (
	"maps"
	"slices"
)

// ExtractLicenses extracts licenses from the given expression without duplicates.
// Returns an array of licenses or error if error occurs during processing.
func ExtractLicenses(expression string) ([]string, error) {
	node, err := parse(expression)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	collectExtractedLicenses(node, seen)
	return slices.Collect(maps.Keys(seen)), nil
}

func collectExtractedLicenses(n *node, seen map[string]struct{}) {
	if n == nil {
		return
	}

	if n.isExpression() {
		collectExtractedLicenses(n.left(), seen)
		collectExtractedLicenses(n.right(), seen)
		return
	}

	reconstructed := n.reconstructedLicenseString()
	if reconstructed == nil {
		return
	}

	license := *reconstructed
	if _, ok := seen[license]; ok {
		return
	}
	seen[license] = struct{}{}
}
