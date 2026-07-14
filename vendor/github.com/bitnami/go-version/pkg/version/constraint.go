package version

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aquasecurity/go-version/pkg/part"
)

const cvRegex string = `v?([0-9|x|X|\*]+)(\.[0-9|x|X|\*]+)?(\.[0-9|x|X|\*]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

var (
	constraintOperators = map[string]operatorFunc{
		"":   constraintEqual,
		"=":  constraintEqual,
		"==": constraintEqual,
		"!=": constraintNotEqual,
		">":  constraintGreaterThan,
		"<":  constraintLessThan,
		">=": constraintGreaterThanEqual,
		"=>": constraintGreaterThanEqual,
		"<=": constraintLessThanEqual,
		"=<": constraintLessThanEqual,
		"~":  constraintTilde,
		"^":  constraintCaret,
	}
	constraintRegexp      *regexp.Regexp
	validConstraintRegexp *regexp.Regexp
)

type operatorFunc func(v, c Version) bool

func init() {
	ops := make([]string, 0, len(constraintOperators))
	for k := range constraintOperators {
		ops = append(ops, regexp.QuoteMeta(k))
	}

	constraintRegexp = regexp.MustCompile(fmt.Sprintf(
		`(%s)\s*(%s)`,
		strings.Join(ops, "|"),
		cvRegex))

	validConstraintRegexp = regexp.MustCompile(fmt.Sprintf(
		`^\s*(\s*(%s)\s*(%s)\s*\,?)*\s*$`,
		strings.Join(ops, "|"),
		cvRegex))
}

type Constraints struct {
	constraints [][]constraint
	conf        conf
}

// NewConstraints parses a given constraint and returns a new instance of Constraints
func NewConstraints(v string, opts ...ConstraintOption) (Constraints, error) {
	c := new(conf)

	// Apply options
	for _, o := range opts {
		o.apply(c)
	}

	var css [][]constraint
	for _, vv := range strings.Split(v, "||") {
		// Validate the segment
		if !validConstraintRegexp.MatchString(vv) {
			return Constraints{}, fmt.Errorf("improper constraint: %s", vv)
		}

		ss := constraintRegexp.FindAllString(vv, -1)
		if ss == nil {
			ss = append(ss, strings.TrimSpace(vv))
		}

		var cs []constraint
		for _, single := range ss {
			c, err := newConstraint(single, *c)
			if err != nil {
				return Constraints{}, err
			}
			cs = append(cs, c)
		}
		css = append(css, cs)
	}

	return Constraints{
		constraints: css,
		conf:        *c,
	}, nil
}

// Returns the string format of the constraints
func (cs Constraints) String() string {
	var csStr []string
	for _, orC := range cs.constraints {
		var cstr []string
		for _, andC := range orC {
			cstr = append(cstr, andC.String())
		}
		csStr = append(csStr, strings.Join(cstr, ","))
	}

	return strings.Join(csStr, "||")
}

// Constraints is one or more constraint that a semantic version can be
// checked against.
type constraint struct {
	version  Version
	operator operatorFunc
	original string
}

func newConstraint(c string, conf conf) (constraint, error) {
	if c == "" {
		return constraint{
			version: Version{
				major:    part.Any(true),
				minor:    part.Any(true),
				patch:    part.Any(true),
				revision: part.NewParts("*"),
			},
			operator: constraintOperators[""],
		}, nil
	}

	m := constraintRegexp.FindStringSubmatch(c)
	if m == nil {
		return constraint{}, fmt.Errorf("improper constraint: %s", c)
	}

	major := m[3]
	minor := strings.TrimPrefix(m[4], ".")
	patch := strings.TrimPrefix(m[5], ".")

	v := Version{
		major:    newPart(major, conf),
		minor:    newPart(minor, conf),
		patch:    newPart(patch, conf),
		original: c,
	}

	revision := part.NewParts(strings.TrimPrefix(m[6], "-"))
	if revision.IsNull() && v.IsAny() {
		revision = append(revision, part.Any(true))
	}
	v.revision = revision

	return constraint{
		version:  v,
		operator: constraintOperators[m[1]],
		original: c,
	}, nil
}

func newPart(p string, conf conf) part.Part {
	if p == "" {
		return part.NewEmpty(!conf.zeroPadding)
	}
	return part.NewPart(p)
}

func (c constraint) String() string {
	return c.original
}

// Check tests if a version satisfies all the constraints.
func (cs Constraints) Check(v Version) bool {
	for _, c := range cs.constraints {
		if andCheck(v, c, cs.conf) {
			return true
		}
	}

	return false
}

func (c constraint) check(v Version, conf conf) bool {
	return c.operator(v, c.version)
}

func andCheck(v Version, constraints []constraint, conf conf) bool {
	for _, c := range constraints {
		if !c.check(v, conf) {
			return false
		}
	}
	return true
}

//-------------------------------------------------------------------
// Constraint functions
//-------------------------------------------------------------------

func constraintEqual(v, c Version) bool {
	return v.Equal(c)
}

func constraintNotEqual(v, c Version) bool {
	return !v.Equal(c)
}

func constraintGreaterThan(v, c Version) bool {
	return v.GreaterThan(c)
}

func constraintLessThan(v, c Version) bool {
	return v.LessThan(c.Min())
}

func constraintGreaterThanEqual(v, c Version) bool {
	return v.GreaterThanOrEqual(c)
}

func constraintLessThanEqual(v, c Version) bool {
	return v.LessThanOrEqual(c)
}

func constraintTilde(v, c Version) bool {
	return v.GreaterThanOrEqual(c) && v.LessThan(c.TildeBump())
}

func constraintCaret(v, c Version) bool {
	return v.GreaterThanOrEqual(c) && v.LessThan(c.CaretBump())
}
