package packager2

// Borrow from "github.com/containers/image" with love <3
// https://github.com/containers/image/blob/aa915b75e867d14f6cb486a4fcc7d7c91cf4ca0a/docker/reference/regexp.go

import (
	"regexp"
	"strings"
)

const (
	// alphaNumeric defines the alpha numeric atom, typically a
	// component of names. This only allows lower case characters and digits.
	alphaNumeric = `[a-z0-9]+`

	// separator defines the separators allowed to be embedded in name
	// components. This allow one period, one or two underscore and multiple
	// dashes. Repeated dashes and underscores are intentionally treated
	// differently. In order to support valid hostnames as name components,
	// supporting repeated dash was added. Additionally double underscore is
	// now allowed as a separator to loosen the restriction for previously
	// supported names.
	separator = `(?:[._]|__|[-]*)`

	// repository name to start with a component as defined by DomainRegexp
	// and followed by an optional port.
	domainComponent = `(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])`

	// The string counterpart for TagRegexp.
	tag = `[\w][\w.-]{0,127}`

	// The string counterpart for DigestRegexp.
	digestPat = `[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}`
)

var (
	// nameComponent restricts registry path component names to start
	// with at least one letter or number, with following parts able to be
	// separated by one period, one or two underscore and multiple dashes.
	nameComponent = expression(
		alphaNumeric,
		optional(repeated(separator, alphaNumeric)))

	domain = expression(
		domainComponent,
		optional(repeated(literal(`.`), domainComponent)),
		optional(literal(`:`), `[0-9]+`))

	namePat = expression(
		optional(domain, literal(`/`)),
		nameComponent,
		optional(repeated(literal(`/`), nameComponent)))

	referencePat = anchored(capture(namePat),
		optional(literal(":"), capture(tag)),
		optional(literal("@"), capture(digestPat)))

	ReferenceRegexp = re(referencePat)
)

// re compiles the string to a regular expression.
var re = regexp.MustCompile

// literal compiles s into a literal regular expression, escaping any regexp
// reserved characters.
func literal(s string) string {
	return regexp.QuoteMeta(s)
}

// expression defines a full expression, where each regular expression must
// follow the previous.
func expression(res ...string) string {
	return strings.Join(res, "")
}

// optional wraps the expression in a non-capturing group and makes the
// production optional.
func optional(res ...string) string {
	return group(expression(res...)) + `?`
}

// repeated wraps the regexp in a non-capturing group to get one or more
// matches.
func repeated(res ...string) string {
	return group(expression(res...)) + `+`
}

// group wraps the regexp in a non-capturing group.
func group(res ...string) string {
	return `(?:` + expression(res...) + `)`
}

// capture wraps the expression in a capturing group.
func capture(res ...string) string {
	return `(` + expression(res...) + `)`
}

// anchored anchors the regular expression by adding start and end delimiters.
func anchored(res ...string) string {
	return `^` + expression(res...) + `$`
}
