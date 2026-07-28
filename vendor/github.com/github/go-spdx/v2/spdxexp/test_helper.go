package spdxexp

// getLicenseNode is a test helper method that is expected to create a valid
// license node.  Use this function when the test data is known to be a valid
// license that would parse successfully.
func getLicenseNode(license string, hasPlus bool) *node {
	return &node{
		role: licenseNode,
		exp:  nil,
		lic: &licenseNodePartial{
			license:      license,
			hasPlus:      hasPlus,
			hasException: false,
			exception:    "",
		},
		ref: nil,
	}
}

// getParsedNode is a test helper method that is expected to create a valid node
// and swallow errors.  This allows test structures to use parsed node data.
// Use this function when the test data is expected to parse successfully.
func getParsedNode(expression string) *node {
	// swallows errors
	n, _ := parse(expression)
	return n
}
