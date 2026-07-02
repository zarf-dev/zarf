// Package version provides the agent version strings.
package version

import (
	_ "embed"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Pre-release builds' versions must be in the format `x.y-beta`, `x.y-beta.z` or `x.y-beta.z.a`

var (
	//go:embed VERSION
	baseVersion string

	// buildNumber is filled in by scripts/build-binary.sh by passing -ldflags
	// "-X github.com/buildkite/agent/v3/version.buildNumber=${BUILDKITE_BUILD_NUMBER}"
	buildNumber = "x"
)

func Version() string {
	return strings.TrimSpace(baseVersion)
}

// BuildNumber returns the build number of the CI pipeline that built the agent.
// You can override buildVersion at compile time by using the ldflag:
//
// "-X github.com/buildkite/agent/v3/version.buildNumber=abc"
//
// An easy way to test this is:
//
// $ go run -buildvcs=true -ldflags "-X github.com/buildkite/agent/v3/version.buildNumber=abc" . --version
//
// On CI, the binaries are always built with the buildVersion variable set.
func BuildNumber() string {
	return buildNumber
}

// commitInfo returns a string consisting of the commit hash and whether the the build was made in a
// `dirty` working directory or not. A dirty working directory is one that has uncommitted changes
// to files that git would track.
func commitInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "x"
	}

	dirty := ".dirty"
	var commit string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit = setting.Value
		case "vcs.modified":
			if setting.Value == "false" {
				dirty = ""
			}
		}
	}

	return commit + dirty
}

// FullVersion is a SemVer 2.0 compliant version string that includes
// [build metadata](https://semver.org/#spec-item-10) consisting of the build
// number (if any), the commit hash, and whether the build was made in a `dirty`
// working directory or not.
func FullVersion() string {
	return fmt.Sprintf("%s+%s.%s", Version(), BuildNumber(), commitInfo())
}

// UserAgent returns a string suitable for use as a User-Agent header.
// TODO-vNext: Include some prefix of the commit hash in the User-Agent header.
func UserAgent() string {
	return fmt.Sprintf(
		"buildkite-agent/%s.%s (%s; %s)",
		Version(),
		BuildNumber(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}
