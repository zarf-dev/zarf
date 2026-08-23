// Copyright 2023 The OpenVEX Authors
// SPDX-License-Identifier: Apache-2.0

package vex

// Product abstracts the VEX product into a struct that can identify software
// through various means. The main one is the ID field which contains an IRI
// identifying the product, possibly pointing to another document with more data,
// like an SBOM. The Product struct also supports naming software using its
// identifiers and/or cryptographic hashes.
type Product struct {
	Component
	Subcomponents []Subcomponent `json:"subcomponents,omitempty"`
}

// Subcomponents are nested entries that list the product's components that are
// related to the statement's vulnerability. The main difference with Product
// and Subcomponent objects is that a Subcomponent cannot nest components.
type Subcomponent struct {
	Component
}

// Product returns true if an identifier and subcomponent identifier match any
// of the identifiers in the product and subcomponents.
func (p *Product) Matches(identifier, subIdentifier string) bool {
	if !p.Component.Matches(identifier) {
		return false
	}

	// If the product has no subcomponents or no subcomponent was specified,
	// matching the product part is enough:
	if len(p.Subcomponents) == 0 || subIdentifier == "" {
		return true
	}

	for _, s := range p.Subcomponents {
		if s.Matches(subIdentifier) {
			return true
		}
	}

	return false
}

type (
	IdentifierLocator string
	IdentifierType    string
)

const (
	PURL  IdentifierType = "purl"
	CPE22 IdentifierType = "cpe22"
	CPE23 IdentifierType = "cpe23"
)

type (
	Algorithm string
	Hash      string
)

// The following list of algorithms follows and expands the IANA list at:
// https://www.iana.org/assignments/named-information/named-information.xhtml
// It expands it, trying to keep the naming pattern.
const (
	MD5        Algorithm = "md5"
	SHA1       Algorithm = "sha1"
	SHA256     Algorithm = "sha-256"
	SHA384     Algorithm = "sha-384"
	SHA512     Algorithm = "sha-512"
	SHA3224    Algorithm = "sha3-224"
	SHA3256    Algorithm = "sha3-256"
	SHA3384    Algorithm = "sha3-384"
	SHA3512    Algorithm = "sha3-512"
	BLAKE2S256 Algorithm = "blake2s-256"
	BLAKE2B256 Algorithm = "blake2b-256"
	BLAKE2B512 Algorithm = "blake2b-512"
	BLAKE3     Algorithm = "blake3"
)
