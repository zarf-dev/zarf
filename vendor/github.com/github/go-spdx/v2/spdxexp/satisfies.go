package spdxexp

import (
	"errors"
	"sort"
	"strings"
)

// ValidateLicenses checks if given licenses are valid according to spdx.
// Returns true if all licenses are valid; otherwise, false.
// Returns all the invalid licenses contained in the `licenses` argument.
func ValidateLicenses(licenses []string) (bool, []string) {
	return ValidateLicensesWithOptions(licenses, ValidateLicensesOptions{})
}

// ValidateLicensesOptions controls how ValidateLicensesWithOptions validates input.
type ValidateLicensesOptions struct {
	// FailComplexExpressions rejects SPDX license expressions (e.g. "MIT AND Apache-2.0").
	// Single license identifiers (including those with a WITH exception) are still allowed.
	FailComplexExpressions bool

	// FailDeprecatedLicenses rejects deprecated SPDX license identifiers (e.g. "eCos-2.0").
	FailDeprecatedLicenses bool

	// FailAllLicenseRefs rejects all SPDX license references (e.g. "LicenseRef-MyLicense").
	FailAllLicenseRefs bool

	// FailAllDocumentRefs rejects all SPDX document references (e.g. "DocumentRef-MyDocument").
	FailAllDocumentRefs bool
}

// ValidateLicensesWithOptions checks if given licenses are valid according to SPDX.
// Returns true if all licenses are valid; otherwise, false.
// Returns all the invalid licenses contained in the `licenses` argument.
func ValidateLicensesWithOptions(licenses []string, options ValidateLicensesOptions) (bool, []string) {
	// handle all other cases with parsing, which will cover both single and multiple licenses and expressions
	_, invalidLicenses := ValidateAndNormalizeLicensesWithOptions(licenses, options)
	return len(invalidLicenses) == 0, invalidLicenses
}

// ValidateAndNormalizeLicensesWithOptions checks if given licenses are valid according to SPDX.
// Supports validation options as defined in ValidateLicensesOptions.
// Returns all validated licenses in their normalized form as the first return value.
// Returns any invalid licenses as the second return value.
func ValidateAndNormalizeLicensesWithOptions(licenses []string, options ValidateLicensesOptions) (normalizedLicenses, invalidLicenses []string) {
	normalizedLicenses = []string{}
	invalidLicenses = []string{}
	seenNormalized := make(map[string]struct{}, len(licenses))

	addNormalized := func(license string) {
		if _, ok := seenNormalized[license]; ok {
			return
		}
		seenNormalized[license] = struct{}{}
		normalizedLicenses = append(normalizedLicenses, license)
	}

	for _, license := range licenses {
		// MIT is the most common license, so check for it first before doing any processing to optimize for this case.
		// By putting the isMIT check here, we can avoid the overhead of parsing for the most common case of MIT.
		// Having it before trimming means that licenses with leading/trailing whitespace will not be validated
		// as MIT by isMIT, but will still be correctly identified using activeLicense.  As this is uncommon, it
		// is an acceptable tradeoff to avoid the overhead of trimming for the more common case.
		if isMIT(license) {
			addNormalized("MIT")
			continue
		}

		license = strings.TrimSpace(license)

		isAtomic := isAtomicLicense(license)
		if isAtomic {
			if ok, normalizedLicense := activeLicense(license); ok {
				addNormalized(normalizedLicense)
				continue
			}

			if ok, normalizedLicense := deprecatedLicense(license); ok {
				if options.FailDeprecatedLicenses {
					invalidLicenses = append(invalidLicenses, license)
					continue
				}
				addNormalized(normalizedLicense)
				// if FailDeprecatedLicenses is false, then consider the deprecated license valid and continue
				continue
			}

			if options.FailAllLicenseRefs {
				if strings.HasPrefix(license, "LicenseRef-") {
					invalidLicenses = append(invalidLicenses, license)
					continue
				}
			}

			if options.FailAllDocumentRefs {
				if strings.HasPrefix(license, "DocumentRef-") {
					invalidLicenses = append(invalidLicenses, license)
					continue
				}
			}

			// need to let this pass through to allow parsing LicenseRef and DocumentRef if either are allowed types
		}

		if !isAtomic {
			if hasException, licensePart, exceptionPart := isLicenseWithException(license); hasException {
				// matches pattern "licensePart WITH exceptionPart", so validate both parts separately
				if ok, normalizedException := exceptionLicense(exceptionPart); ok {
					if ok, normalizedLicense := activeLicense(licensePart); ok {
						addNormalized(normalizedLicense + " WITH " + normalizedException)
						continue
					}
					if !options.FailDeprecatedLicenses {
						if ok, normalizedLicense := deprecatedLicense(licensePart); ok {
							addNormalized(normalizedLicense + " WITH " + normalizedException)
							continue
						}
					}
				}
				invalidLicenses = append(invalidLicenses, license)
				continue
			}
		}

		// all other non-atomic expressions are complex expressions with conjunctions (e.g. "MIT AND Apache-2.0"),
		// so fail if complex expressions are not allowed
		if options.FailComplexExpressions && !isAtomic {
			invalidLicenses = append(invalidLicenses, license)
			continue
		}

		// need to parse if allowing any of LicenseRef, DocumentRef, or complex expressions to be able to determine
		// whether the license expression is valid
		var parsedLicense *node
		var err error
		if parsedLicense, err = parse(license); err != nil {
			invalidLicenses = append(invalidLicenses, license)
		} else {
			normalizedLicense := *parsedLicense.reconstructedLicenseString()
			addNormalized(normalizedLicense)
		}
	}
	return normalizedLicenses, invalidLicenses
}

// Satisfies determines if the allowed list of licenses satisfies the test license expression.
// Returns true if allowed list satisfies test license expression; otherwise, false.
// Returns error if error occurs during processing.
func Satisfies(testExpression string, allowedList []string) (bool, error) {
	if len(allowedList) == 0 {
		return false, errors.New("allowedList requires at least one element, but is empty")
	}

	// MIT is the most common license, so check for it first before doing any processing to optimize for this case.
	// By putting the isMIT check here, we can avoid the overhead of parsing for the most common case of MIT.
	// Having it before trimming means that licenses with leading/trailing whitespace will not be validated
	// as MIT by isMIT, but will still be correctly identified using activeLicense.  As this is uncommon, it
	// is an acceptable tradeoff to avoid the overhead of trimming for the more common case.
	if isMIT(testExpression) {
		for _, allowed := range allowedList {
			if strings.EqualFold(allowed, "MIT") {
				return true, nil
			}
		}
		return false, nil
	}

	testExpression = strings.TrimSpace(testExpression)

	if isAtomicLicense(testExpression) {
		// if only one license in the test expression, check for active license to avoid the overhead of parsing
		if ok, _ := activeLicense(testExpression); ok {
			for _, allowed := range allowedList {
				if strings.EqualFold(allowed, testExpression) {
					return true, nil
				}
			}
		}

		// if only one license in the test expression, check for deprecated license to avoid the overhead of parsing
		if ok, _ := deprecatedLicense(testExpression); ok {
			for _, allowed := range allowedList {
				if strings.EqualFold(allowed, testExpression) {
					return true, nil
				}
			}
		}
	}

	// if test expression is a single license with exception, check it now to avoid the overhead of parsing
	if hasException, licensePart, exceptionPart := isLicenseWithException(testExpression); hasException {
		// matches pattern "licensePart WITH exceptionPart", so validate both parts separately
		if ok, _ := activeLicense(licensePart); ok {
			if ok, _ := exceptionLicense(exceptionPart); ok {
				for _, allowed := range allowedList {
					if strings.EqualFold(allowed, testExpression) {
						return true, nil
					}
				}
			}
		}
	}

	// handle all other cases with parsing, which will cover both single and multiple licenses and expressions
	expressionNode, err := parse(testExpression)
	if err != nil {
		return false, err
	}
	allowedNodes, err := stringsToNodes(allowedList)
	if err != nil {
		return false, err
	}
	sortAndDedup(allowedNodes)

	expandedExpression := expressionNode.expand(true)

	for _, expressionPart := range expandedExpression {
		if isCompatible(expressionPart, allowedNodes) {
			// return once any expressionPart is compatible with the allow list
			// * each part is an array of licenses that are ANDed, meaning all have to be on the allowedList
			// * the parts are ORed, meaning only one of the parts need to be compatible
			return true, nil
		}
	}
	return false, nil
}

// stringsToNodes converts an array of single license strings to to an array of license nodes.
func stringsToNodes(licenseStrings []string) ([]*node, error) {
	nodes := make([]*node, len(licenseStrings))
	for i, s := range licenseStrings {
		node, err := parse(s)
		if err != nil {
			return nil, err
		}
		if node.isExpression() {
			return nil, errors.New("expressions are not supported in the allowedList")
		}
		nodes[i] = node
	}
	return nodes, nil
}

// isMIT checks if the test expression is MIT, ignoring case.
// NOTE: Caller should trim the test expression before calling this function to avoid false
// negatives (e.g. " MIT " would not match "MIT").
func isMIT(testExpression string) bool {
	return strings.EqualFold(testExpression, "MIT")
}

// isAtomicLicense checks if the test expression is a single license identifier (e.g. "MIT").
// NOTE: Caller should trim the test expression before calling this function to avoid false
// negatives (e.g. " MIT " would not be considered a single license).
func isAtomicLicense(testExpression string) bool {
	return !strings.Contains(testExpression, " ")
}

// isException checks if the test expression contains two licenses separated by WITH
// (e.g. "GPL-2.0-or-later WITH Bison-exception-2.2").
// NOTE: Caller should trim the test expression before calling this function to avoid false
// negatives (e.g. " MIT " would not be considered a single license).
func isLicenseWithException(testExpression string) (bool, string, string) {
	// split by " " and check if there are exactly 3 parts and the middle part is "WITH"
	parts := strings.Fields(testExpression)
	if len(parts) == 3 && strings.EqualFold(parts[1], "WITH") {
		return true, parts[0], parts[2]
	}
	return false, "", ""
}

// isCompatible checks if expressionPart is compatible with allowed list.
// Expression part is an array of licenses that are ANDed together.
// Allowed is an array of licenses that can fulfill the expression.
func isCompatible(expressionPart, allowed []*node) bool {
	for _, expLicense := range expressionPart {
		compatible := false
		for _, allowedLicense := range allowed {
			nodes := &nodePair{firstNode: expLicense, secondNode: allowedLicense}
			if nodes.licensesAreCompatible() || nodes.licenseRefsAreCompatible() {
				compatible = true
				break
			}
		}
		if !compatible {
			// no compatible license found for one of the required licenses
			return false
		}
	}
	// found a compatible license in test for each required license
	return true
}

// expand will expand the given expression into an equivalent array representing ANDed licenses
// grouped in an array and ORed licenses each in a separate array.
//
// Example:
//
//	License node: "MIT" becomes [["MIT"]]
//	OR Expression: "MIT OR Apache-2.0" becomes [["MIT"], ["Apache-2.0"]]
//	AND Expression: "MIT AND Apache-2.0" becomes [["MIT", "Apache-2.0"]]
//	OR-AND Expression: "MIT OR Apache-2.0 AND GPL-2.0" becomes [["MIT"], ["Apache-2.0", "GPL-2.0"]]
//	OR(AND) Expression: "MIT OR (Apache-2.0 AND GPL-2.0)" becomes [["MIT"], ["Apache-2.0", "GPL-2.0"]]
//	AND-OR Expression: "MIT AND Apache-2.0 OR GPL-2.0" becomes [["Apache-2.0", "MIT], ["GPL-2.0"]]
//	AND(OR) Expression: "MIT AND (Apache-2.0 OR GPL-2.0)" becomes [["Apache-2.0", "MIT], ["GPL-2.0", "MIT"]]
//	OR-AND-OR Expression: "MIT OR ISC AND Apache-2.0 OR GPL-2.0" becomes
//	    [["MIT"], ["Apache-2.0", "ISC"], ["GPL-2.0"]]
//	(OR)AND(OR) Expression: "(MIT OR ISC) AND (Apache-2.0 OR GPL-2.0)" becomes
//	    [["Apache-2.0", "MIT"], ["GPL-2.0", "MIT"], ["Apache-2.0", "ISC"], ["GPL-2.0", "ISC"]]
//	OR(AND)OR Expression: "MIT OR (ISC AND Apache-2.0) OR GPL-2.0" becomes
//	    [["MIT"], ["Apache-2.0", "ISC"], ["GPL-2.0"]]
//	AND-OR-AND Expression: "MIT AND ISC OR Apache-2.0 AND GPL-2.0" becomes
//	    [["ISC", "MIT"], ["Apache-2.0", "GPL-2.0"]]
//	(AND)OR(AND) Expression: "(MIT AND ISC) OR (Apache-2.0 AND GPL-2.0)" becomes
//	    [["ISC", "MIT"], ["Apache-2.0", "GPL-2.0"]]
//	AND(OR)AND Expression: "MIT AND (ISC OR Apache-2.0) AND GPL-2.0" becomes
//	    [["GPL-2.0", "ISC", "MIT"], ["Apache-2.0", "GPL-2.0", "MIT"]]
func (n *node) expand(withDeepSort bool) [][]*node {
	if n.isLicense() || n.isLicenseRef() {
		return [][]*node{{n}}
	}

	var expanded [][]*node
	if n.isOrExpression() {
		expanded = n.expandOr()
	} else {
		expanded = n.expandAnd()
	}

	if withDeepSort {
		expanded = deepSort(expanded)
	}
	return expanded
}

// expandOr expands the given expression into an equivalent array representing ORed licenses each in a separate array.
//
// Example:
//
//	OR Expression: "MIT OR Apache-2.0" becomes [["MIT"], ["Apache-2.0"]]
func (n *node) expandOr() [][]*node {
	var result [][]*node
	result = expandOrTerm(n.left(), result)
	result = expandOrTerm(n.right(), result)
	return result
}

// expandOrTerm expands the terms of an OR expression.
func expandOrTerm(term *node, result [][]*node) [][]*node {
	if term.isLicense() {
		result = append(result, []*node{term})
	} else if term.isExpression() {
		if term.isOrExpression() {
			left := term.expandOr()
			result = append(result, left...)
		} else if term.isAndExpression() {
			left := term.expandAnd()[0]
			result = append(result, left)
		}
	}
	return result
}

// expandAnd expands the given expression into an equivalent array representing ANDed licenses
// grouped in an array.  When an ORed expression is combined with AND, the ORed
// expressions are combined with the ANDed expressions.
//
// Example:
//
//	AND Expression: "MIT AND Apache-2.0" becomes [["MIT", "Apache-2.0"]]
//	AND(OR) Expression: "MIT AND (Apache-2.0 OR GPL-2.0)" becomes [["Apache-2.0", "MIT], ["GPL-2.0", "MIT"]]
//
// See more examples under func expand.
func (n *node) expandAnd() [][]*node {
	left := expandAndTerm(n.left())
	right := expandAndTerm(n.right())

	if len(left) > 1 || len(right) > 1 {
		// an OR expression has been processed
		// somewhere on the left and/or right node path
		return appendTerms(left, right)
	}

	// only AND expressions have been processed
	return mergeTerms(left, right)
}

// expandAndTerm expands the terms of an AND expression.
func expandAndTerm(term *node) [][]*node {
	var result [][]*node
	if term.isLicense() || term.isLicenseRef() {
		result = append(result, []*node{term})
	} else if term.isExpression() {
		if term.isAndExpression() {
			result = term.expandAnd()
		} else if term.isOrExpression() {
			result = term.expandOr()
		}
	}
	return result
}

// appendTerms appends results from expanding the right expression into the results
// from expanding the left expression.  When at least one of the left/right
// nodes includes an OR expression, the values are spread across at times
// producing more results than exists in the left or right results.
//
// Example:
//
//	left: {{"MIT"}} right: {{"ISC"}, {"Apache-2.0"}} becomes
//	  {{"MIT", "ISC"}, {"MIT", "Apache-2.0"}}
func appendTerms(left, right [][]*node) [][]*node {
	var result [][]*node
	for _, r := range right {
		for _, l := range left {
			tmp := l
			tmp = append(tmp, r...)
			result = append(result, tmp)
		}
	}
	return result
}

// mergeTerms merges results from expanding left and right expressions.
// When neither left/right nodes includes an OR expression, the values
// are merged left and right results.
//
// Example:
//
//	left: {{"MIT"}} right: {{"ISC", "Apache-2.0"}} becomes
//	  {{"MIT", "ISC", "Apache-2.0"}}
func mergeTerms(left, right [][]*node) [][]*node {
	results := left
	for _, r := range right {
		for j, l := range results {
			results[j] = append(l, r...)
		}
	}
	return results
}

// sortAndDedup sorts an array of license nodes and then removes duplicates.
func sortAndDedup(nodes []*node) []*node {
	if len(nodes) <= 1 {
		return nodes
	}

	sortLicenses(nodes)
	prev := 1
	for curr := 1; curr < len(nodes); curr++ {
		if *nodes[curr-1].reconstructedLicenseString() != *nodes[curr].reconstructedLicenseString() {
			nodes[prev] = nodes[curr]
			prev++
		}
	}

	return nodes[:prev]
}

// deepSort sorts a two-dimensional array of license nodes.  Internal arrays are sorted first.
// Then each array of nodes are sorted relative to the other arrays.
//
// Example:
//
//	BEFORE {{"MIT", "GPL-2.0"}, {"ISC", "Apache-2.0"}}
//	AFTER  {{"Apache-2.0", "ISC"}, {"GPL-2.0", "MIT"}}
func deepSort(nodes2d [][]*node) [][]*node {
	if len(nodes2d) == 0 || len(nodes2d) == 1 && len(nodes2d[0]) <= 1 {
		return nodes2d
	}

	// sort each array internally
	// Example:
	//   BEFORE {{"MIT", "GPL-2.0"}, {"ISC", "Apache-2.0"}}
	//   AFTER  {{"GPL-2.0", "MIT"}, {"Apache-2.0", "ISC"}}
	for _, nodes := range nodes2d {
		if len(nodes) > 1 {
			sortLicenses(nodes)
		}
	}

	// sort arrays relative to each other
	// Example:
	//   BEFORE {{"GPL-2.0", "MIT"}, {"Apache-2.0", "ISC"}}
	//   AFTER  {{"Apache-2.0", "ISC"}, {"GPL-2.0", "MIT"}}
	sort.Slice(nodes2d, func(i, j int) bool {
		// TODO: Consider refactor to map nodes to licenseString before processing.
		for k := range nodes2d[j] {
			if k >= len(nodes2d[i]) {
				// if the first k elements are equal and the second array is
				// longer than the first, the first is considered less than
				return true
			}
			iLicense := *nodes2d[i][k].reconstructedLicenseString()
			jLicense := *nodes2d[j][k].reconstructedLicenseString()
			if iLicense != jLicense {
				// when elements are not equal, return true if first is less than
				return iLicense < jLicense
			}
		}
		// all elements are equal, return false to avoid a swap
		return false
	})

	return nodes2d
}
