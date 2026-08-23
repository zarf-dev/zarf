package image

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/containerd/errdefs"
)

var specifierRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Platform is a subset of the supported fields from specs "github.com/opencontainers/image-spec/specs-go/v1.Platform"
type Platform struct {
	// Architecture field specifies the CPU architecture, for example
	// `amd64` or `ppc64`.
	Architecture string `json:"architecture"`

	// OS specifies the operating system, for example `linux` or `windows`.
	OS string `json:"os"`

	// Variant is an optional field specifying a variant of the CPU, for
	// example `v7` to specify ARMv7 when architecture is `arm`.
	Variant string `json:"variant,omitempty"`
}

func NewPlatform(specifier string) (*Platform, error) {
	p, err := parse(specifier)
	if err != nil {
		return nil, fmt.Errorf("failed to parse platform %q: %w", specifier, err)
	}

	// if no OS is provided, assume linux
	if p.OS == "" {
		p.OS = "linux"
	}

	return p, nil
}

func (p *Platform) String() string {
	if p == nil {
		return ""
	}
	var fields []string
	if p.OS != "" {
		fields = append(fields, p.OS)
	}

	if p.Architecture != "" {
		fields = append(fields, p.Architecture)
	}

	if p.Variant != "" {
		fields = append(fields, p.Variant)
	}

	return strings.Join(fields, "/")
}

// parse has been extracted out from containerd (platforms/platforms.go). The behavior in containerd is to use the
// runtime package to assume default values. This might be OK for a container engine, however, syft and other consumers
// of stereoscope are at the client side, where we cannot fill default OS/arch values based on the client we
// happen to be running from.
func parse(specifier string) (*Platform, error) {
	if strings.Contains(specifier, "*") {
		// TODO(stevvooe): need to work out exact wildcard handling
		return nil, fmt.Errorf("%q: wildcards not yet supported: %w", specifier, errdefs.ErrInvalidArgument)
	}

	parts := strings.Split(specifier, "/")

	for _, part := range parts {
		if !specifierRe.MatchString(part) {
			return nil, fmt.Errorf("%q is an invalid component of %q: platform specifier component must match %q: %w", part, specifier, specifierRe.String(), errdefs.ErrInvalidArgument)
		}
	}

	p := &Platform{}
	switch len(parts) {
	case 1:

		if osGuess := normalizeOS(parts[0]); isKnownOS(osGuess) {
			p.OS = osGuess
			return p, nil
		}

		archGuess, variantGuess := normalizeArch(parts[0], "")
		if isKnownArch(archGuess) {
			p.Architecture = archGuess
			p.Variant = variantGuess
			return p, nil
		}

		return nil, fmt.Errorf("%q: unknown operating system or architecture: %w", specifier, errdefs.ErrInvalidArgument)
	case 2:
		// In this case, we treat as a regular os/arch pair or architecture/variant pair.
		var archGuess, variantGuess string
		if osGuess := normalizeOS(parts[0]); isKnownOS(osGuess) {
			p.OS = osGuess
			archGuess, variantGuess = normalizeArch(parts[1], "")
		} else {
			archGuess, variantGuess = normalizeArch(parts[0], parts[1])
		}

		if isKnownArch(archGuess) {
			p.Architecture = archGuess
			p.Variant = variantGuess
			return p, nil
		}

		return nil, fmt.Errorf("%q: unknown operating system or architecture: %w", specifier, errdefs.ErrInvalidArgument)
	case 3:
		// we have a fully specified variant, this is rare
		if osGuess := normalizeOS(parts[0]); isKnownOS(osGuess) {
			p.OS = osGuess
		}

		archGuess, variantGuess := normalizeArch(parts[1], parts[2])

		if isKnownArch(archGuess) {
			p.Architecture = archGuess
			p.Variant = variantGuess
			return p, nil
		}

		return nil, fmt.Errorf("%q: unknown operating system or architecture: %w", specifier, errdefs.ErrInvalidArgument)
	}

	return nil, fmt.Errorf("%q: cannot parse platform specifier: %w", specifier, errdefs.ErrInvalidArgument)
}

// These function are generated from https://golang.org/src/go/build/syslist.go.
//
// We use switch statements because they are slightly faster than map lookups
// and use a little less memory.

// isKnownOS returns true if we know about the operating system.
//
// The OS value should be normalized before calling this function.
func isKnownOS(os string) bool {
	switch os {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "js", "linux", "nacl", "netbsd", "openbsd", "plan9", "solaris", "windows", "zos":
		return true
	}
	return false
}

// isKnownArch returns true if we know about the architecture.
//
// The arch value should be normalized before being passed to this function.
func isKnownArch(arch string) bool {
	switch arch {
	//nolint:goconst
	case "386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be", "ppc64", "ppc64le", "mips", "mipsle", "mips64", "mips64le", "mips64p32", "mips64p32le", "ppc", "riscv", "riscv64", "s390", "s390x", "sparc", "sparc64", "wasm":
		return true
	}
	return false
}

// normalizeOS normalizes the OS.
func normalizeOS(os string) string {
	if os == "" {
		return runtime.GOOS
	}
	os = strings.ToLower(os)
	if os == "macos" {
		os = "darwin"
	}
	return os
}

// normalizeArch normalizes the architecture.
func normalizeArch(arch, variant string) (string, string) {
	arch, variant = strings.ToLower(arch), strings.ToLower(variant)
	switch arch {
	case "i386":
		arch = "386"
		variant = ""
	case "x86_64", "x86-64":
		arch = "amd64"
		variant = ""
	case "aarch64", "arm64":
		arch = "arm64"
		switch variant {
		case "8", "v8":
			variant = ""
		}
	case "armhf":
		arch = "arm"
		variant = "v7"
	case "armel":
		arch = "arm"
		variant = "v6"
	case "arm":
		switch variant {
		case "", "7":
			variant = "v7"
		case "5", "6", "8":
			variant = "v" + variant
		}
	}

	return arch, variant
}
