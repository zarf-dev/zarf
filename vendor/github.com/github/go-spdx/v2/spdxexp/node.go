package spdxexp

import (
	"fmt"
	"sort"
	"strings"
)

type nodePair struct {
	firstNode  *node
	secondNode *node
}

type nodeRole uint8

const (
	expressionNode nodeRole = iota
	licenseRefNode
	licenseNode
)

type node struct {
	role nodeRole
	exp  *expressionNodePartial
	lic  *licenseNodePartial
	ref  *referenceNodePartial
}

type expressionNodePartial struct {
	left        *node
	conjunction string
	right       *node
}

type licenseNodePartial struct {
	license      string
	hasPlus      bool
	hasException bool
	exception    string
}

type referenceNodePartial struct {
	hasDocumentRef bool
	documentRef    string
	licenseRef     string
}

// ---------------------- Helper Methods ----------------------

func (n *node) isExpression() bool {
	return n.role == expressionNode
}

func (n *node) isOrExpression() bool {
	if !n.isExpression() {
		return false
	}
	return *n.conjunction() == "or"
}

func (n *node) isAndExpression() bool {
	if !n.isExpression() {
		return false
	}
	return *n.conjunction() == "and"
}

func (n *node) left() *node {
	if !n.isExpression() {
		return nil
	}
	return n.exp.left
}

func (n *node) conjunction() *string {
	if !n.isExpression() {
		return nil
	}
	return &(n.exp.conjunction)
}

func (n *node) right() *node {
	if !n.isExpression() {
		return nil
	}
	return n.exp.right
}

func (n *node) isLicense() bool {
	return n.role == licenseNode
}

// license returns the value of the license field.
// See also reconstructedLicenseString()
func (n *node) license() *string {
	if !n.isLicense() {
		return nil
	}
	return &(n.lic.license)
}

func (n *node) exception() *string {
	if !n.hasException() {
		return nil
	}
	return &(n.lic.exception)
}

func (n *node) hasPlus() bool {
	if !n.isLicense() {
		return false
	}
	return n.lic.hasPlus
}

func (n *node) hasException() bool {
	if !n.isLicense() {
		return false
	}
	return n.lic.hasException
}

func (n *node) isLicenseRef() bool {
	return n.role == licenseRefNode
}

func (n *node) licenseRef() *string {
	if !n.isLicenseRef() {
		return nil
	}
	return &(n.ref.licenseRef)
}

func (n *node) documentRef() *string {
	if !n.hasDocumentRef() {
		return nil
	}
	return &(n.ref.documentRef)
}

func (n *node) hasDocumentRef() bool {
	if !n.isLicenseRef() {
		return false
	}
	return n.ref.hasDocumentRef
}

// reconstructedLicenseString returns the string representation of a license, license ref, or expression.
// TODO: Original had "NOASSERTION".  Does that still apply?
func (n *node) reconstructedLicenseString() *string {
	switch n.role {
	case expressionNode:
		return n.reconstructedExpressionString()
	case licenseNode:
		license := *n.license()
		if n.hasPlus() && !strings.HasSuffix(strings.ToLower(license), "-or-later") {
			license += "+"
		}
		if n.hasException() {
			license += " WITH " + *n.exception()
		}
		return &license
	case licenseRefNode:
		license := "LicenseRef-" + *n.licenseRef()
		if n.hasDocumentRef() {
			license = "DocumentRef-" + *n.documentRef() + ":" + license
		}
		return &license
	}
	return nil
}

func (n *node) reconstructedExpressionString() *string {
	if n == nil || !n.isExpression() {
		return nil
	}

	left := n.left()
	right := n.right()
	if left == nil || right == nil {
		return nil
	}

	leftStr := left.reconstructedLicenseString()
	rightStr := right.reconstructedLicenseString()
	if leftStr == nil || rightStr == nil {
		return nil
	}

	conj := n.conjunction()
	if conj == nil {
		return nil
	}

	operator := strings.ToUpper(*conj)
	if operator != "AND" && operator != "OR" {
		return nil
	}

	parentPrec := nodePrecedence(n)
	leftRendered := *leftStr
	if left.isExpression() && nodePrecedence(left) < parentPrec {
		leftRendered = "(" + leftRendered + ")"
	}
	rightRendered := *rightStr
	if right.isExpression() && nodePrecedence(right) < parentPrec {
		rightRendered = "(" + rightRendered + ")"
	}

	s := fmt.Sprintf("%s %s %s", leftRendered, operator, rightRendered)
	return &s
}

func nodePrecedence(n *node) int {
	if n == nil {
		return 0
	}
	if !n.isExpression() {
		// atomic (license/licenseRef)
		return 3
	}
	conj := n.conjunction()
	if conj == nil {
		return 0
	}
	switch strings.ToLower(*conj) {
	case "and":
		return 2
	case "or":
		return 1
	default:
		return 0
	}
}

// sortLicenses sorts an array of license and license reference nodes alphabetically based
// on their reconstructedLicenseString() representation.  The sort function does not expect
// expression nodes, but if one is in the nodes list, it will sort to the end.
func sortLicenses(nodes []*node) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[j].isExpression() {
			// push second license toward end by saying first license is less than
			return true
		}
		if nodes[i].isExpression() {
			// push first license toward end by saying second license is less than
			return false
		}
		return *nodes[i].reconstructedLicenseString() < *nodes[j].reconstructedLicenseString()
	})
}

// ---------------------- Comparator Methods ----------------------

// licensesAreCompatible returns true if two licenses are compatible; otherwise, false.
// Two licenses are compatible if they are the same license or if they are in the same
// license group and they meet one of the following rules:
//
// * both licenses have the `hasPlus` flag set to true
// * the first license has the `hasPlus` flag and the second license is in the first license's range or greater
// * the second license has the `hasPlus` flag and the first license is in the second license's range or greater
// * both licenses are in the same range
func (nodes *nodePair) licensesAreCompatible() bool {
	// checking ranges is expensive, so check for simple cases first
	if !nodes.firstNode.isLicense() || !nodes.secondNode.isLicense() {
		return false
	}
	if !nodes.exceptionsAreCompatible() {
		return false
	}
	if nodes.licensesExactlyEqual() {
		return true
	}

	// simple cases don't apply, so check license ranges
	// NOTE: Ranges are organized into groups (referred to as license groups) of the same base license (e.g. GPL).
	//       Groups have sub-groups of license versions (referred to as the range) where each member is considered
	//       to be the same version (e.g. {GPL-2.0, GPL-2.0-only}). The sub-groups are in ascending order within
	//       the license group, such that the first sub-group is considered to be less than the second sub-group,
	//       and so on. (e.g. {{GPL-1.0}, {GPL-2.0, GPL-2.0-only}} implies {GPL-1.0} < {GPL-2.0, GPL-2.0-only}).
	if nodes.secondNode.hasPlus() {
		if nodes.firstNode.hasPlus() {
			// first+, second+ just need to be in same range group
			return nodes.rangesAreCompatible()
		}
		// first, second+ requires first to be in range of second
		return nodes.identifierInRange()
	}
	// else secondNode does not have plus
	if nodes.firstNode.hasPlus() {
		// first+, second requires second to be in range of first
		revNodes := &nodePair{firstNode: nodes.secondNode, secondNode: nodes.firstNode}
		return revNodes.identifierInRange()
	}
	// first, second requires both to be in same range group
	return nodes.rangesEqual()
}

// licenseRefsAreCompatible returns true if two license references are compatible; otherwise, false.
func (nodes *nodePair) licenseRefsAreCompatible() bool {
	if !nodes.firstNode.isLicenseRef() || !nodes.secondNode.isLicenseRef() {
		return false
	}

	compatible := *nodes.firstNode.licenseRef() == *nodes.secondNode.licenseRef()
	compatible = compatible && (nodes.firstNode.hasDocumentRef() == nodes.secondNode.hasDocumentRef())
	if compatible && nodes.firstNode.hasDocumentRef() {
		compatible = compatible && (*nodes.firstNode.documentRef() == *nodes.secondNode.documentRef())
	}
	return compatible
}

// licenseRefsAreCompatible returns true if two licenses are in the same license group (e.g. all "GPL" licenses are in the same
// license group); otherwise, false.
func (nodes *nodePair) rangesAreCompatible() bool {
	firstNode := *nodes.firstNode
	secondNode := *nodes.secondNode

	firstRange := getLicenseRange(*firstNode.license())
	secondRange := getLicenseRange(*secondNode.license())

	// When both licenses allow later versions (i.e. hasPlus==true), being in the same license
	// group is sufficient for compatibility, as long as, any exception is also compatible
	// Example: All Apache licenses (e.g. Apache-1.0, Apache-2.0) are in the same license group
	return sameLicenseGroup(firstRange, secondRange)
}

// identifierInRange returns true if the (first) simple license is in range of the (second)
// ranged license; otherwise, false.
func (nodes *nodePair) identifierInRange() bool {
	simpleLicense := nodes.firstNode
	plusLicense := nodes.secondNode

	return compareGT(simpleLicense, plusLicense) || compareEQ(simpleLicense, plusLicense)
}

// exceptionsAreCompatible returns true if neither license has an exception or they have
// the same exception; otherwise, false
func (nodes *nodePair) exceptionsAreCompatible() bool {
	firstNode := *nodes.firstNode
	secondNode := *nodes.secondNode

	if !firstNode.hasException() && !secondNode.hasException() {
		// if neither has an exception, then licenses are compatible
		return true
	}

	if firstNode.hasException() != secondNode.hasException() {
		// if one has and exception and the other does not, then the license are NOT compatible
		return false
	}

	return *nodes.firstNode.exception() == *nodes.secondNode.exception()
}

// rangesEqual returns true if the licenses are in the same range; otherwise, false
// (e.g. GPL-2.0-only == GPL-2.0)
func (nodes *nodePair) rangesEqual() bool {
	return compareEQ(nodes.firstNode, nodes.secondNode)
}

// licensesExactlyEqual returns true if the licenses are the same; otherwise, false
func (nodes *nodePair) licensesExactlyEqual() bool {
	return strings.EqualFold(*nodes.firstNode.reconstructedLicenseString(), *nodes.secondNode.reconstructedLicenseString())
}
