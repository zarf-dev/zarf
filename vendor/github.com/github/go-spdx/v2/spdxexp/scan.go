package spdxexp

/* Translation to Go from javascript code: https://github.com/clearlydefined/spdx-expression-parse.js/blob/master/scan.js */

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type expressionStream struct {
	expression string
	index      int
	err        error
}

type token struct {
	role  tokenrole
	value string
}

type tokenrole uint8

const (
	operatorToken tokenrole = iota
	documentRefToken
	licenseRefToken
	licenseToken
	exceptionToken
)

// Scan scans a string expression gathering valid SPDX expression tokens.  Returns error if any tokens are invalid.
func scan(expression string) ([]token, error) {
	var tokens []token
	var token *token

	exp := &expressionStream{expression: expression, index: 0, err: nil}

	for exp.hasMore() {
		exp.skipWhitespace()
		if !exp.hasMore() {
			break
		}

		token = exp.parseToken()
		if exp.err != nil {
			// stop processing at first error and return
			return nil, exp.err
		}

		if token == nil {
			// TODO: shouldn't happen ???
			return nil, errors.New("got nil token when expecting more")
		}

		tokens = append(tokens, *token)
	}
	return tokens, nil
}

// Determine if expression has more to process.
func (exp *expressionStream) hasMore() bool {
	return exp.index < len(exp.expression)
}

// Try to read the next token starting at index. Returns error if no token is recognized.
func (exp *expressionStream) parseToken() *token {
	// Ordering matters
	op := exp.readOperator()
	if exp.err != nil {
		return nil
	}
	if op != nil {
		return op
	}

	dref := exp.readDocumentRef()
	if exp.err != nil {
		return nil
	}
	if dref != nil {
		return dref
	}

	lref := exp.readLicenseRef()
	if exp.err != nil {
		return nil
	}
	if lref != nil {
		return lref
	}

	identifier := exp.readLicense()
	if exp.err != nil {
		return nil
	}
	if identifier != nil {
		return identifier
	}

	errmsg := fmt.Sprintf("unexpected '%c' at offset %d", exp.expression[exp.index], exp.index)
	exp.err = errors.New(errmsg)
	return nil
}

// Read more from expression if the next substring starting at index matches the regex pattern.
func (exp *expressionStream) readRegex(pattern string) string {
	expressionSlice := exp.expression[exp.index:]

	r, _ := regexp.Compile(pattern)
	i := r.FindStringIndex(expressionSlice)
	if i != nil && i[1] > 0 && i[0] == 0 {
		// match found in expression at index
		exp.index += i[1]
		return expressionSlice[0:i[1]]
	}
	return ""
}

// Read more from expression if the substring starting at index is the next expected string.
func (exp *expressionStream) read(next string) string {
	expressionSlice := exp.expression[exp.index:]

	if strings.HasPrefix(expressionSlice, next) {
		// next found in expression at index
		exp.index += len(next)
		return next
	}
	return ""
}

// Skip whitespace in expression starting at index
func (exp *expressionStream) skipWhitespace() {
	exp.readRegex("[ ]*")
}

// Read operator in expression starting at index if it exists
func (exp *expressionStream) readOperator() *token {
	possibilities := []string{"WITH", "AND", "OR", "(", ")", ":", "+"}

	var op string
	for _, p := range possibilities {
		op = exp.read(p)
		if len(op) > 0 {
			break
		}
	}
	if len(op) == 0 {
		// not an error if an operator isn't found
		return nil
	}

	if op == "+" && exp.index > 1 && exp.expression[exp.index-2:exp.index-1] == " " {
		exp.err = errors.New("unexpected space before +")
		exp.index--
		return nil
	}

	return &token{role: operatorToken, value: op}
}

// Get id from expression starting at index.  Raise error if id not found.
func (exp *expressionStream) readID() string {
	id := exp.readRegex("[A-Za-z0-9-.]+")
	if len(id) == 0 {
		errmsg := fmt.Sprintf("expected id at offset %d", exp.index)
		exp.err = errors.New(errmsg)
		return ""
	}
	return id
}

// Read DocumentRef in expression starting at index if it exists. Raise error if found and id doesn't follow.
func (exp *expressionStream) readDocumentRef() *token {
	ref := exp.read("DocumentRef-")
	if len(ref) == 0 {
		// not an error if a DocumentRef isn't found
		return nil
	}

	id := exp.readID()
	if exp.err != nil {
		return nil
	}
	return &token{role: documentRefToken, value: id}
}

// Read LicenseRef in expression starting at index if it exists. Raise error if found and id doesn't follow.
func (exp *expressionStream) readLicenseRef() *token {
	ref := exp.read("LicenseRef-")
	if len(ref) == 0 {
		// not an error if a LicenseRef isn't found
		return nil
	}

	id := exp.readID()
	if exp.err != nil {
		return nil
	}
	return &token{role: licenseRefToken, value: id}
}

// Read a LICENSE/EXCEPTION in expression starting at index if it exists. Raise error if found and id doesn't follow.
func (exp *expressionStream) readLicense() *token {
	// because readID matches broadly, save the index so it can be reset if an actual license is not found
	index := exp.index

	license := exp.readID()
	if exp.err != nil {
		return nil
	}

	if token := exp.normalizeLicense(license); token != nil {
		return token
	}

	// license not found in indices, need to reset index since readID advanced it
	exp.index = index
	errmsg := fmt.Sprintf("unknown license '%s' at offset %d", license, exp.index)
	exp.err = errors.New(errmsg)
	return nil
}

// Generate a token using the normalized form of the license name.
//
// License name can be in the form:
//   - a_license-2.0, a_license, a_license-ab - there is variability in the form of the base license.  a_license-2.0 is used for these
//     examples, but any base license form can have the suffixes described.
//   - a_license-2.0-only - normalizes to a_license-2.0 if the -only form is not specifically in the set of licenses
//   - a_license-2.0-or-later - normalizes to a_license-2.0+ if the -or-later form is not specifically in the set of licenses
//   - a_license-2.0+ - normalizes to a_license-2.0-or-later if the -or-later form is specifically in the set of licenses
func (exp *expressionStream) normalizeLicense(license string) *token {
	if token := licenseLookup(license); token != nil {
		// checks active and exception license lists
		// deprecated list is checked at the end to avoid a deprecated license being used for +
		// (example: GPL-1.0 is on the deprecated list, but GPL-1.0+ should become GPL-1.0-or-later)
		return token
	}

	lenLicense := len(license)
	if strings.HasSuffix(license, "-only") {
		adjustedLicense := license[0 : lenLicense-5]
		if token := licenseLookup(adjustedLicense); token != nil {
			// no need to remove the -only from the expression stream; it is ignored
			return token
		}
	}
	if exp.hasMore() && exp.expression[exp.index:exp.index+1] == "+" {
		adjustedLicense := license[0:lenLicense] + "-or-later"
		if token := licenseLookup(adjustedLicense); token != nil {
			// need to consume the + to avoid a + operator token being added
			exp.index++
			return token
		}
	}
	if strings.HasSuffix(license, "-or-later") {
		adjustedLicense := license[0 : lenLicense-9]
		if token := licenseLookup(adjustedLicense); token != nil {
			// replace `-or-later` with `+`
			newExpression := exp.expression[0:exp.index-len("-or-later")] + "+"
			if exp.hasMore() {
				newExpression += exp.expression[exp.index+1:]
			}
			exp.expression = newExpression
			// update index to remove `-or-later`; now pointing at the `+` operator
			exp.index -= len("-or-later")

			return token
		}
	}

	return deprecatedLicenseLookup(license)
}

// Lookup license identifier in active and exception lists to determine if it is a supported SPDX id
func licenseLookup(license string) *token {
	active, preferredLicense := activeLicense(license)
	if active {
		return &token{role: licenseToken, value: preferredLicense}
	}
	exception, preferredLicense := exceptionLicense(license)
	if exception {
		return &token{role: exceptionToken, value: preferredLicense}
	}
	return nil
}

// Lookup license identifier in deprecated list to determine if it is a supported SPDX id
func deprecatedLicenseLookup(license string) *token {
	deprecated, preferredLicense := deprecatedLicense(license)
	if deprecated {
		return &token{role: licenseToken, value: preferredLicense}
	}
	return nil
}
