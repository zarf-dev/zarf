/*
Copyright 2023 The OpenVEX Authors
SPDX-License-Identifier: Apache-2.0
*/

package vex

import "strings"

// Component abstracts the common construct shared by product and subcomponents
// allowing OpenVEX statements to point to a piece of software by referencing it
// by hash or identifier.
//
// The ID should be an IRI uniquely identifying the product. Software can be
// referenced as a VEX product or subcomponent using only its IRI or it may be
// referenced by its crptographic hashes and/or other identifiers but, in no case,
// must an IRI describe two different pieces of software or used to describe
// a range of software.
type Component struct {
	// ID is an IRI identifying the component. It is optional as the component
	// can also be identified using hashes or software identifiers.
	ID string `json:"@id,omitempty"`

	// Hashes is a map of hashes to identify the component using cryptographic
	// hashes.
	Hashes map[Algorithm]Hash `json:"hashes,omitempty"`

	// Identifiers is a list of software identifiers that describe the component.
	Identifiers map[IdentifierType]string `json:"identifiers,omitempty"`

	// Supplier is an optional machine-readable identifier for the supplier of
	// the component. Valid examples include email address or IRIs.
	Supplier string `json:"supplier,omitempty"`
}

// Matches returns true if one of the components identifiers match a string.
// All types except purl are checked string vs string. Purls are a special
// case and can match from more generic to more specific.
// Note that a future iterarion of this function will treat CPEs in the same
// way.
func (c *Component) Matches(identifier string) bool {
	// If we have an exact match in the ID, match
	if c.ID == identifier && c.ID != "" {
		return true
	} else if strings.HasPrefix(c.ID, "pkg:") {
		// ... but the identifier can be a purl. If it is, then do
		// a purl comparison:
		if PurlMatches(c.ID, identifier) {
			return true
		}
	}

	for t, id := range c.Identifiers {
		if id == identifier {
			return true
		}

		if t == PURL && strings.HasPrefix(identifier, "pkg:") {
			if PurlMatches(id, identifier) {
				return true
			}
		}
	}

	for _, hashVal := range c.Hashes {
		if hashVal == Hash(identifier) {
			return true
		}
	}

	return false
}
