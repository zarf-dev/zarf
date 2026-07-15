package version

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"

	"github.com/aquasecurity/go-version/pkg/part"
	"github.com/aquasecurity/go-version/pkg/prerelease"
)

var (
	// ErrInvalidSemVer is returned when a given version is invalid
	ErrInvalidSemVer = errors.New("invalid semantic version")
)

var versionRegex *regexp.Regexp

// regex is the regular expression used to parse a SemVer string.
// See: https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
const regex string = `^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)` +
	`(?:-(?P<revision>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))` +
	`?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`

func init() {
	versionRegex = regexp.MustCompile(regex)
}

// Version represents a semantic version.
type Version struct {
	major, minor, patch part.Part
	revision            part.Parts
	buildMetadata       string
	original            string
}

// New returns an instance of Version
func New(major, minor, patch part.Part, pre part.Parts, metadata string) Version {
	return Version{
		major:         major,
		minor:         minor,
		patch:         patch,
		revision:      pre,
		buildMetadata: metadata,
	}
}

// Parse parses a given version and returns a new instance of Version
func Parse(v string) (Version, error) {
	m := versionRegex.FindStringSubmatch(v)
	if m == nil {
		return Version{}, ErrInvalidSemVer
	}

	major, err := part.NewUint64(m[versionRegex.SubexpIndex("major")])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := part.NewUint64(m[versionRegex.SubexpIndex("minor")])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %w", err)
	}

	patch, err := part.NewUint64(m[versionRegex.SubexpIndex("patch")])
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch version: %w", err)
	}

	return Version{
		major:         major,
		minor:         minor,
		patch:         patch,
		revision:      part.NewParts(m[versionRegex.SubexpIndex("revision")]),
		buildMetadata: m[versionRegex.SubexpIndex("buildmetadata")],
		original:      v,
	}, nil
}

// String converts a Version object to a string.
func (v Version) String() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%d.%d.%d", v.major, v.minor, v.patch)
	if !v.revision.IsNull() {
		fmt.Fprintf(&buf, "-%s", v.revision)
	}
	if v.buildMetadata != "" {
		fmt.Fprintf(&buf, "+%s", v.buildMetadata)
	}

	return buf.String()
}

// IsAny returns true if major, minor or patch is wild card
func (v Version) IsAny() bool {
	return v.major.IsAny() || v.minor.IsAny() || v.patch.IsAny()
}

// IncMajor produces the next major version.
// e.g. 1.2.3 => 2.0.0
func (v Version) IncMajor() Version {
	v.major = v.major.(part.Uint64) + 1
	v.minor = part.Zero
	v.patch = part.Zero
	v.revision = part.Parts{}
	v.buildMetadata = ""
	v.original = v.String()
	return v
}

// IncMinor produces the next minor version.
func (v Version) IncMinor() Version {
	v.minor = v.minor.(part.Uint64) + 1
	v.patch = part.Zero
	v.revision = part.Parts{}
	v.buildMetadata = ""
	v.original = v.String()
	return v
}

// IncPatch produces the next patch version.
func (v Version) IncPatch() Version {
	v.patch = v.patch.(part.Uint64) + 1
	v.revision = part.Parts{}
	v.buildMetadata = ""
	v.original = v.String()
	return v
}

// Min produces the minimum version if it includes wild card.
// 1.2.* => 1.2.0
// 1.*.* => 1.0.0
func (v Version) Min() Version {
	if v.major.IsAny() {
		v.major = part.Zero
	}
	if v.minor.IsAny() {
		v.minor = part.Zero
	}
	if v.patch.IsAny() {
		v.patch = part.Zero
	}
	if v.revision.IsAny() {
		v.revision = part.Parts{}
	}
	v.buildMetadata = ""
	v.original = v.String()
	return v
}

// Original returns the original value.
func (v Version) Original() string {
	return v.original
}

// Major returns the major version.
func (v Version) Major() part.Part {
	return v.major
}

// Minor returns the minor version.
func (v Version) Minor() part.Part {
	return v.minor
}

// Patch returns the patch version.
func (v Version) Patch() part.Part {
	return v.patch
}

// Revision returns the revision.
func (v Version) Revision() part.Parts {
	return v.revision
}

// HasRevision returns if version has revision.
// 1.2.3   => false
// 1.2.3-1 => true
func (v Version) HasRevision() bool {
	return !v.revision.IsNull()
}

// Metadata returns the metadata on the version.
func (v Version) Metadata() string {
	return v.buildMetadata
}

// LessThan tests if one version is less than another one.
func (v Version) LessThan(o Version) bool {
	return v.Compare(o) < 0
}

// LessThanOrEqual tests if this version is less than or equal to another version.
func (v Version) LessThanOrEqual(o Version) bool {
	return v.Compare(o) <= 0
}

// GreaterThan tests if one version is greater than another one.
func (v Version) GreaterThan(o Version) bool {
	return v.Compare(o) > 0
}

// GreaterThanOrEqual tests if this version is greater than or equal to another version.
func (v Version) GreaterThanOrEqual(o Version) bool {
	return v.Compare(o) >= 0
}

// Equal tests if two versions are equal to each other.
// Note, versions can be equal with different metadata since metadata
// is not considered part of the comparable version.
func (v Version) Equal(o Version) bool {
	return v.Compare(o) == 0
}

// Compare compares this version to another one. It returns -1, 0, or 1 if
// the version smaller, equal, or larger than the other version.
//
// Versions are compared by X.Y.Z. Build metadata is ignored. Revision is
// greater than the version without a revision.
func (v Version) Compare(o Version) int {
	// Compare the major, minor, and patch version for differences. If a
	// difference is found return the comparison.
	result := v.major.Compare(o.major)
	if result != 0 || v.major.IsAny() || o.major.IsAny() {
		return result
	}
	result = v.minor.Compare(o.minor)
	if result != 0 || v.minor.IsAny() || o.minor.IsAny() {
		return result
	}
	result = v.patch.Compare(o.patch)
	if result != 0 || v.patch.IsAny() || o.patch.IsAny() {
		return result
	}

	// At this point the major, minor, and patch versions are the same.
	// We consider any revision to be greater than a version without a revision.
	if v.HasRevision() && !o.HasRevision() {
		return 1
	} else if !v.HasRevision() && o.HasRevision() {
		return -1
	}
	return prerelease.Compare(v.revision, o.revision)
}

// TildeBump returns the maximum version of tilde ranges
// e.g. ~1.2.3 := >=1.2.3 <1.3.0
// In this case, it returns 1.3.0
// ref. https://docs.npmjs.com/cli/v6/using-npm/semver#tilde-ranges-123-12-1
func (v Version) TildeBump() Version {
	switch {
	case v.major.IsAny(), v.major.IsEmpty():
		v.major = part.Uint64(math.MaxUint64)
		return v
	case v.minor.IsAny(), v.minor.IsEmpty():
		// e.g. 1 => 2.0.0
		return v.IncMajor()
	case v.patch.IsAny(), v.patch.IsEmpty():
		// e.g. 1.2 => 1.3.0
		return v.IncMinor()
	default:
		// e.g. 1.2.3 => 1.3.0
		return v.IncMinor()
	}
}

// CaretBump returns the maximum version of caret ranges
// e.g. ^1.2.3 := >=1.2.3 <2.0.0
// In this case, it returns 2.0.0
// ref. https://docs.npmjs.com/cli/v6/using-npm/semver#caret-ranges-123-025-004
func (v Version) CaretBump() Version {
	switch {
	case v.major.IsAny(), v.major.IsEmpty():
		v.major = part.Uint64(math.MaxUint64)
		return v
	case v.major.(part.Uint64) != 0:
		// e.g. 1 => 2.0.0
		return v.IncMajor()
	case v.minor.IsAny(), v.minor.IsEmpty():
		// e.g. 0 => 1.0.0
		return v.IncMajor()
	case v.minor.(part.Uint64) != 0:
		// e.g. 0.2.3 => 0.3.0
		return v.IncMinor()
	case v.patch.IsAny(), v.patch.IsEmpty():
		// e.g. 0.0 => 0.1.0
		return v.IncMinor()
	default:
		// e.g. 0.0.3 => 0.0.4
		return v.IncPatch()
	}
}
