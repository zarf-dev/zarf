/*
Copyright 2023 The OpenVEX Authors
SPDX-License-Identifier: Apache-2.0
*/

package vex

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/package-url/packageurl-go"
)

const (
	// TypeURI is the type used to describe VEX documents, e.g. within [in-toto
	// statements].
	//
	// [in-toto statements]: https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
	TypeURI = "https://openvex.dev/ns"

	// SpecVersion is the latest released version of the openvex. This constant
	// is used to form the context URL when generating new documents.
	SpecVersion = "0.2.0"

	// DefaultAuthor is the default value for a document's Author field.
	DefaultAuthor = "Unknown Author"

	// DefaultRole is the default value for a document's AuthorRole field.
	DefaultRole = ""

	// Context is the URL of the json-ld context definition
	Context = "https://openvex.dev/ns"

	// PublicNamespace is the public openvex namespace for common @ids
	PublicNamespace = "https://openvex.dev/docs"

	// NoActionStatementMsg is the action statement that informs that there is no action statement :/
	NoActionStatementMsg = "No action statement provided"

	errMsgParse = "error"
)

// DefaultNamespace is the URL that will be used to generate new IRIs for generated
// documents and nodes. It is set to the OpenVEX public namespace by default.
var DefaultNamespace = PublicNamespace

// The VEX type represents a VEX document and all of its contained information.
type VEX struct {
	Metadata
	Statements []Statement `json:"statements"`
}

// The Metadata type represents the metadata associated with a VEX document.
type Metadata struct {
	// Context is the URL pointing to the jsonld context definition
	Context string `json:"@context"`

	// ID is the identifying string for the VEX document. This should be unique per
	// document.
	ID string `json:"@id"`

	// Author is the identifier for the author of the VEX statement, ideally a common
	// name, may be a URI. [author] is an individual or organization. [author]
	// identity SHOULD be cryptographically associated with the signature of the VEX
	// statement or document or transport.
	Author string `json:"author"`

	// AuthorRole describes the role of the document Author.
	AuthorRole string `json:"role,omitempty"`

	// Timestamp defines the time at which the document was issued.
	Timestamp *time.Time `json:"timestamp"`

	// LastUpdated marks the time when the document had its last update. When the
	// document changes both version and this field should be updated.
	LastUpdated *time.Time `json:"last_updated,omitempty"`

	// Version is the document version. It must be incremented when any content
	// within the VEX document changes, including any VEX statements included within
	// the VEX document.
	Version int `json:"version"`

	// Tooling expresses how the VEX document and contained VEX statements were
	// generated. It's optional. It may specify tools or automated processes used in
	// the document or statement generation.
	Tooling string `json:"tooling,omitempty"`

	// Supplier is an optional field.
	Supplier string `json:"supplier,omitempty"`
}

// New returns a new, initialized VEX document.
func New() VEX {
	now := time.Now()
	t, err := DateFromEnv()
	if err != nil {
		slog.Warn(err.Error())
	}
	if t != nil {
		now = *t
	}
	return VEX{
		Metadata: Metadata{
			Context:    ContextLocator(),
			Author:     DefaultAuthor,
			AuthorRole: DefaultRole,
			Version:    1,
			Timestamp:  &now,
		},
		Statements: []Statement{},
	}
}

// ToJSON serializes the VEX document to JSON and writes it to the passed writer.
func (vexDoc *VEX) ToJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(vexDoc); err != nil {
		return fmt.Errorf("encoding vex document: %w", err)
	}
	return nil
}

// MarshalJSON the document object overrides its marshaling function to normalize
// the timezones in all dates to Zulu.
func (vexDoc *VEX) MarshalJSON() ([]byte, error) {
	type alias VEX
	var ts, lu string

	if vexDoc.Timestamp != nil {
		ts = vexDoc.Timestamp.UTC().Format(time.RFC3339)
	}
	if vexDoc.LastUpdated != nil {
		lu = vexDoc.LastUpdated.UTC().Format(time.RFC3339)
	}

	return json.Marshal(&struct {
		*alias
		TimeZonedTimestamp   string `json:"timestamp"`
		TimeZonedLastUpdated string `json:"last_updated,omitempty"`
	}{
		TimeZonedTimestamp:   ts,
		TimeZonedLastUpdated: lu,
		alias:                (*alias)(vexDoc),
	})
}

// EffectiveStatement returns the latest VEX statement for a given product and
// vulnerability, that is the statement that contains the latest data about
// impact to a given product.
func (vexDoc *VEX) EffectiveStatement(product, vulnID string) (s *Statement) {
	statements := vexDoc.Statements
	var t time.Time
	if vexDoc.Timestamp != nil {
		t = *vexDoc.Timestamp
	}

	SortStatements(statements, t)

	for i := len(statements) - 1; i >= 0; i-- {
		if statements[i].Matches(vulnID, product, nil) {
			return &statements[i]
		}
	}
	return nil
}

// StatementFromID returns a statement for a given vulnerability if there is one.
//
// Deprecated: vex.StatementFromID is deprecated and will be removed in an upcoming version
func (vexDoc *VEX) StatementFromID(id string) *Statement {
	slog.Warn("vex.StatementFromID is deprecated and will be removed in an upcoming version")
	for i := range vexDoc.Statements {
		if string(vexDoc.Statements[i].Vulnerability.Name) == id && len(vexDoc.Statements[i].Products) > 0 {
			return vexDoc.EffectiveStatement(vexDoc.Statements[i].Products[0].ID, id)
		}
	}
	return nil
}

// Matches returns the latest VEX statement for a given product and
// vulnerability. That is, the statement that contains the latest data with
// impact data of a vulnerability on a given product.
func (vexDoc *VEX) Matches(vulnID, product string, subcomponents []string) []Statement {
	statements := vexDoc.Statements
	var t time.Time
	if vexDoc.Timestamp != nil {
		t = *vexDoc.Timestamp
	}

	matches := []Statement{}

	for i := len(statements) - 1; i >= 0; i-- {
		if statements[i].Matches(vulnID, product, subcomponents) {
			matches = append(matches, statements[i])
		}
	}

	SortStatements(matches, t)
	return matches
}

// CanonicalHash returns a hash representing the state of impact statements
// expressed in it. This hash should be constant as long as the impact
// statements are not modified. Changes in extra information and metadata
// will not alter the hash.
func (vexDoc *VEX) CanonicalHash() (string, error) {
	// Here's the algo:

	// 1. Start with the document date. In unixtime to avoid format variance.
	cString := fmt.Sprintf("%d", vexDoc.Timestamp.Unix())

	// 2. Document version
	cString += fmt.Sprintf(":%d", vexDoc.Version)

	// 3. Author identity
	cString += fmt.Sprintf(":%s", vexDoc.Author)

	// 4. Sort the statements
	stmts := vexDoc.Statements
	SortStatements(stmts, *vexDoc.Timestamp)

	// 5. Now add the data from each statement
	//nolint:gocritic
	for _, s := range stmts {
		// 5a. Vulnerability
		cString += cstringFromVulnerability(s.Vulnerability)
		// 5b. Status + Justification
		cString += fmt.Sprintf(":%s:%s", s.Status, s.Justification)
		// 5c. Statement time, in unixtime. If it exists, if not the doc's
		if s.Timestamp != nil {
			cString += fmt.Sprintf(":%d", s.Timestamp.Unix())
		} else {
			cString += fmt.Sprintf(":%d", vexDoc.Timestamp.Unix())
		}
		// 5d. Sorted product strings
		prods := []string{}
		for _, p := range s.Products {
			prodString := cstringFromComponent(p.Component)
			if len(p.Subcomponents) > 0 {
				for _, sc := range p.Subcomponents {
					prodString += cstringFromComponent(sc.Component)
				}
			}
			prods = append(prods, prodString)
		}
		sort.Strings(prods)
		cString += strings.Join(prods, ":")
	}

	// 6. Hash the string in sha256 and return
	h := sha256.New()
	if _, err := h.Write([]byte(cString)); err != nil {
		return "", fmt.Errorf("hashing canonicalization string: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// cstringFromComponent returns a string concatenating the data of a component
// this internal function is meant to generate a predicatable string to generate
// the document's CanonicalHash
func cstringFromComponent(c Component) string {
	s := fmt.Sprintf(":%s", c.ID)

	for algo, val := range c.Hashes {
		s += fmt.Sprintf(":%s@%s", algo, val)
	}

	for t, id := range c.Identifiers {
		s += fmt.Sprintf(":%s@%s", t, id)
	}

	return s
}

// cstringFromVulnerability returns a string concatenating the vulnerability
// elements into a reproducible string that can be used to hash or index the
// vulnerability data or the statement.
func cstringFromVulnerability(v Vulnerability) string {
	cString := fmt.Sprintf(":%s:%s", v.ID, v.Name)
	list := []string{}
	for i := range v.Aliases {
		list = append(list, string(v.Aliases[i]))
	}
	sort.Strings(list)
	cString += fmt.Sprintf(":%s", strings.Join(list, ":"))
	return cString
}

// GenerateCanonicalID generates an ID for the document. The ID will be
// based on the canonicalization hash. This means that documents
// with the same impact statements will always get the same ID.
// Trying to generate the id of a doc with an existing ID will
// not do anything.
func (vexDoc *VEX) GenerateCanonicalID() (string, error) {
	if vexDoc.ID != "" {
		return vexDoc.ID, nil
	}
	cHash, err := vexDoc.CanonicalHash()
	if err != nil {
		return "", fmt.Errorf("getting canonical hash: %w", err)
	}

	// For common namespaced documents we namespace them into /public
	vexDoc.ID = fmt.Sprintf("%s/public/vex-%s", DefaultNamespace, cHash)
	return vexDoc.ID, nil
}

// DateFromEnv returns a time object representing the time specified in the
// `SOURCE_DATE_EPOCH` environment variable, whose value can be specified as
// either UNIX seconds or as a RFC3339 value.
func DateFromEnv() (*time.Time, error) {
	// Support env var for reproducible vexing
	d := os.Getenv("SOURCE_DATE_EPOCH")
	if d == "" {
		return nil, nil
	}

	var t time.Time
	sec, err := strconv.ParseInt(d, 10, 64)
	if err == nil {
		t = time.Unix(sec, 0)
	} else {
		t, err = time.Parse(time.RFC3339, d)
		if err != nil {
			return nil, fmt.Errorf("failed to parse env var SOURCE_DATE_EPOCH: %w", err)
		}
	}
	return &t, nil
}

// ContextLocator returns the locator string for the current OpenVEX version.
func ContextLocator() string {
	return fmt.Sprintf("%s/v%s", Context, SpecVersion)
}

// PurlMatches returns true if purl1 matches the more specific purl2. It takes into
// account all segments of the pURL, including qualifiers. purl1 is considered to
// be more general and purl2 more specific and thus, the following considerations
// are made when matching:
//
//   - If purl1 does not have a version, it will match any version in purl2
//   - If purl1 has qualifers, purl2 must have the same set of qualifiers to match.
//   - Inversely, purl2 can have any number of qualifiers not found on purl1 and
//     still match.
//   - If any of the purls is invalid, the function returns false.
//
// Purl version ranges are not supported yet but they will be in a future version
// of this matching function.
func PurlMatches(purl1, purl2 string) bool {
	p1, err := packageurl.FromString(purl1)
	if err != nil {
		return false
	}
	p2, err := packageurl.FromString(purl2)
	if err != nil {
		return false
	}

	if p1.Type != p2.Type {
		return false
	}

	if p1.Namespace != p2.Namespace {
		return false
	}

	if p1.Name != p2.Name {
		return false
	}

	if p1.Version != "" && p2.Version == "" {
		return false
	}

	if p1.Version != p2.Version && p1.Version != "" && p2.Version != "" {
		return false
	}

	p1q := p1.Qualifiers.Map()
	p2q := p2.Qualifiers.Map()

	// All qualifiers in p1 must be in p2 to match
	for k, v1 := range p1q {
		if v2, ok := p2q[k]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}

// StatementsByVulnerability returns a list of statements that apply to a
// vulnerability ID. These are guaranteed to be ordered according to the VEX
// history.
func (vexDoc *VEX) StatementsByVulnerability(id string) []Statement {
	ret := []Statement{}
	for i := range vexDoc.Statements {
		if vexDoc.Statements[i].Vulnerability.Matches(id) {
			ret = append(ret, vexDoc.Statements[i])
		}
	}
	SortStatements(ret, *vexDoc.Timestamp)
	return ret
}

// ExtractStatements extracts the statements from the document with the dates
// inherited from the encapsuling doc to make them stand alone.
func (vexDoc *VEX) ExtractStatements() []*Statement {
	ret := []*Statement{}

	// Cycle the VEX statements, copy each and complete the dates
	for i := range vexDoc.Statements {
		nstatement := vexDoc.Statements[i].DeepCopy()

		// Carry over the dates from the doc
		if nstatement.Timestamp == nil {
			nstatement.Timestamp = vexDoc.Timestamp
		}
		if nstatement.LastUpdated == nil {
			nstatement.LastUpdated = vexDoc.LastUpdated
		}
		ret = append(ret, nstatement)
	}
	return ret
}
