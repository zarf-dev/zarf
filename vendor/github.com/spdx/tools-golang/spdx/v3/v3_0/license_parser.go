package v3_0

import (
	"fmt"
	"regexp"
	"strings"
)

// licenseIdentifierChars matches the characters allowed in an SPDX license
// identifier token: idstring characters (ALPHA / DIGIT / "-" / ".") plus ":"
// used to separate the DocumentRef portion of an external reference.
var licenseIdentifierChars = regexp.MustCompile(`^[A-Za-z0-9.:-]+$`)

// ParseLicenseExpression parses an SPDX license expression string into the corresponding model types.
// It handles AND, OR, WITH operators, the + (or-later) suffix, LicenseRef and DocumentRef references, and
// parenthesized sub-expressions with operator precedence (lowest to highest): OR, AND, WITH, + (or-later).
// This is a lenient parser, which will return all errors encountered and all parseable licenses -- strict users
// must check for errors.
//
// Examples:
//   - "MIT" → *ListedLicense
//   - "MIT OR Apache-2.0" → *DisjunctiveLicenseSet
//   - "MIT AND Apache-2.0" → *ConjunctiveLicenseSet
//   - "GPL-2.0-only WITH Classpath-exception-2.0" → *WithAdditionOperator
//   - "GPL-2.0-only+" → *OrLaterOperator wrapping *ListedLicense
//   - "LicenseRef-custom" → *CustomLicense
//   - "DocumentRef-ext:LicenseRef-custom" → *CustomLicense
//   - "NONE" → IndividualLicensingInfo_NoneLicense
//   - "NOASSERTION" → IndividualLicensingInfo_NoAssertionLicense
func ParseLicenseExpression(expression string) (AnyLicenseInfo, error) {
	expression = strings.TrimSpace(expression)
	if len(expression) == 0 {
		return nil, fmt.Errorf("expression is empty")
	}
	p := licenseParser{
		in:  expression,
		pos: 0,
	}
	for p.pos != len(expression) { // try to parse until we find a license
		result := p.parseOr()
		if result != nil { // a properly specified license will get a result the first iteration with no errors
			if p.tok != "" && len(p.errs) == 0 {
				p.err("unexpected trailing token(s)")
			}
			return result, collectErrs(&p)
		}
	}
	if len(p.errs) > 0 {
		return nil, collectErrs(&p)
	}
	return nil, fmt.Errorf("unable to parse expression: %s", expression)
}

type licenseParser struct {
	errs []ParseError
	in   string
	tok  string
	pos  int
}

// err adds a parsing error with position and token captured automatically
func (p *licenseParser) err(format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	// don't report duplicate errors
	if len(p.errs) > 0 {
		if p.errs[len(p.errs)-1].Message == msg {
			return
		}
	}
	p.errs = append(p.errs, ParseError{p.pos - len(p.tok), p.tok, msg})
}

// next consumes whitespace and captures the subsequent token in p.tok
func (p *licenseParser) next() {
	for p.pos < len(p.in) && isWhitespace(p.in[p.pos]) {
		p.pos++
	}
	start := p.pos
	for p.pos < len(p.in) && !isWhitespace(p.in[p.pos]) {
		c := p.in[p.pos]
		if c == '(' || c == ')' || c == '+' {
			if start == p.pos { // capture exactly 1 character
				p.pos++
			}
			break
		}
		p.pos++
	}
	p.tok = p.in[start:p.pos]
}

// parseOr handles: expr ("OR" expr)*; flattens consecutive & nested OR operands into a single DisjunctiveLicenseSet.
func (p *licenseParser) parseOr() AnyLicenseInfo {
	out := p.parseAnd()
	if out == nil {
		return nil // already noted the error; continue trying to parse
	}
	if opOr(p.tok) {
		s, _ := out.(*DisjunctiveLicenseSet)
		if s == nil {
			s = &DisjunctiveLicenseSet{}
			s.Members = append(s.Members, out)
		}
		for opOr(p.tok) {
			rhs := p.parseAnd() // using parseAnd here gives the correct precedence behavior
			if next, ok := rhs.(*DisjunctiveLicenseSet); ok {
				s.Members = append(s.Members, next.Members...)
			} else if rhs != nil {
				s.Members = append(s.Members, rhs)
			} // if rhs == nil, already reported an error
		}
		if len(s.Members) > 1 {
			out = s
		}
	}
	return out
}

// parseAnd handles: expr ("AND" expr)*; flattens consecutive & nested AND operands into a single ConjunctiveLicenseSet.
func (p *licenseParser) parseAnd() AnyLicenseInfo {
	out := p.parseLicense()
	if out == nil {
		return nil // already noted the error; continue trying to parse
	}
	if opAnd(p.tok) {
		s, _ := out.(*ConjunctiveLicenseSet)
		if s == nil {
			s = &ConjunctiveLicenseSet{}
			s.Members = append(s.Members, out)
		}
		for opAnd(p.tok) {
			rhs := p.parseAnd()
			if next, ok := rhs.(*ConjunctiveLicenseSet); ok {
				s.Members = append(s.Members, next.Members...)
			} else if rhs != nil {
				s.Members = append(s.Members, rhs)
			} // if rhs == nil, already reported an error
		}
		if len(s.Members) > 1 {
			out = s
		}
	}
	return out
}

// parseLicense handles: ("(" expr ")")? | license (+)? ("WITH" exception)?
func (p *licenseParser) parseLicense() AnyLicenseInfo {
	p.next()
	if p.tok == "(" {
		lic := p.parseOr()
		if p.tok == ")" {
			p.next()
		} else if len(p.errs) == 0 {
			p.err("unclosed parenthesis")
		}
		return lic
	}
	lic := makeLicense(p.tok)
	if lic == nil {
		p.err("unexpected token")
		if p.pos < len(p.in) {
			return p.parseLicense()
		}
		return nil
	}
	p.next()
	if p.tok == "+" {
		if p.pos > 2 && isWhitespace(p.in[p.pos-2]) { // whitespace is not allowed per spec
			p.err("illegal whitespace before +")
		}
		if l, ok := lic.(AnyLicense); ok {
			lic = &OrLaterOperator{SubjectLicense: l}
		} else {
			p.err("invalid license used with + operator: %v", lic)
		}
		p.next()
	}
	if opWith(p.tok) {
		p.next()
		addition := makeAddition(p.tok)
		if addition == nil {
			p.err("unable to parse addition")
			return lic
		}
		if l, ok := lic.(AnyExtendableLicense); ok {
			lic = &WithAdditionOperator{
				SubjectAddition:          addition,
				SubjectExtendableLicense: l,
			}
		} else {
			p.err("invalid license used with WITH addition operator: %v", lic)
		}
		p.next()
	}
	return lic
}

// makeLicense creates the appropriate license type based on the identifier:
// NONE and NOASSERTION return appropriate values
// LicenseRef-* and DocumentRef-* identifiers produce CustomLicense, all others produce ListedLicense.
func makeLicense(ident string) AnyLicenseInfo {
	switch {
	case strings.EqualFold(ident, "NONE"):
		return IndividualLicensingInfo_NoneLicense
	case strings.EqualFold(ident, NOASSERTION):
		return IndividualLicensingInfo_NoAssertionLicense
	case strings.HasPrefix(ident, "LicenseRef-") || strings.HasPrefix(ident, "DocumentRef-"):
		return &CustomLicense{
			ID: ident,
		}
	case invalidIdentifier(ident):
		return nil // operators are not allowed as licenses, e.g. "MIT OR OR", or other malformed expression
	}
	// it's possible we should set the ID to ID: fmt.Sprintf("https://spdx.org/licenses/%s", ident) but for now
	// we do not set the license ID to avoid a user editing shared objects and unexpectedly affecting an entire graph
	// These will not have all the required information such as license Text
	return &ListedLicense{
		Name: ident,
	}
}

// makeAddition creates the license addition based on the identifier:
// AdditionRef-*, LicenseRef-*, and DocumentRef-* identifiers produce CustomLicenseAddition, all others produce ListedLicenseException.
func makeAddition(ident string) AnyLicenseAddition {
	switch {
	case strings.HasPrefix(ident, "AdditionRef-") || strings.HasPrefix(ident, "LicenseRef-") || strings.HasPrefix(ident, "DocumentRef-"):
		return &CustomLicenseAddition{
			ID: ident,
		}
	case invalidIdentifier(ident):
		return nil
	}
	return &ListedLicenseException{
		AdditionText: ident,
	}
}

func opOr(tok string) bool {
	return strings.EqualFold(tok, "or")
}

func opAnd(tok string) bool {
	return strings.EqualFold(tok, "and")
}

func opWith(tok string) bool {
	return strings.EqualFold(tok, "with")
}

func invalidIdentifier(tok string) bool {
	return opOr(tok) || opAnd(tok) || opWith(tok) || !licenseIdentifierChars.MatchString(tok)
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

type ParseError struct {
	Position int
	Token    string
	Message  string
}

type ParseErrors struct {
	Expression string
	Errors     []ParseError
}

// collectErrs returns nil if there are no errors or a ParseErrors struct
func collectErrs(p *licenseParser) error {
	if len(p.errs) == 0 {
		return nil
	}
	return ParseErrors{
		Expression: p.in,
		Errors:     p.errs,
	}
}

// Error renders all parse errors against the full expression, with a caret
// line under each error pointing at the offending token:
//
//	MIT OR (BLAH AND) OR OR
//	                ^ unexpected token: )
func (p ParseErrors) Error() string {
	var b strings.Builder
	b.WriteString(p.Expression)
	for _, e := range p.Errors {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", e.Position))
		b.WriteString("^ ")
		b.WriteString(e.Message)
		b.WriteString(": ")
		b.WriteString(e.Token)
	}
	return b.String()
}
