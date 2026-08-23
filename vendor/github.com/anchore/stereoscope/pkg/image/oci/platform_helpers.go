package oci

import (
	"fmt"
	"runtime"

	containerregistryV1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/anchore/stereoscope/pkg/image"
)

func validatePlatform(platform *image.Platform, givenOs, givenArch, givenVariant string) error {
	if platform == nil {
		return nil
	}
	if givenArch == "" || givenOs == "" {
		return newErrPlatformMismatch(platform, fmt.Errorf("missing architecture or OS from image config when user specified platform=%q", platform.String()))
	}
	platformStr := fmt.Sprintf("%s/%s", givenOs, givenArch)
	if givenVariant != "" {
		platformStr += "/" + givenVariant
	}
	actualPlatform, err := containerregistryV1.ParsePlatform(platformStr)
	if err != nil {
		return newErrPlatformMismatch(platform, fmt.Errorf("failed to parse platform from image config: %w", err))
	}
	if actualPlatform == nil {
		return newErrPlatformMismatch(platform, fmt.Errorf("not platform from image config (from %q)", platformStr))
	}
	if !matchesPlatform(*actualPlatform, *toContainerRegistryPlatform(platform)) {
		return newErrPlatformMismatch(platform, fmt.Errorf("image platform=%q does not match user specified platform=%q", actualPlatform.String(), platform.String()))
	}
	return nil
}

func newErrPlatformMismatch(platform *image.Platform, err error) *image.ErrPlatformMismatch {
	return &image.ErrPlatformMismatch{
		ExpectedPlatform: platform.String(),
		Err:              err,
	}
}

func toContainerRegistryPlatform(p *image.Platform) *containerregistryV1.Platform {
	if p == nil {
		return nil
	}
	return &containerregistryV1.Platform{
		Architecture: p.Architecture,
		OS:           p.OS,
		Variant:      p.Variant,
	}
}

// defaultPlatformIfNil sets the platform to use the host's architecture
// if no platform was specified. The OCI registry NewProvider uses "linux/amd64"
// as a hard-coded default platform, which has surprised customers
// running stereoscope on non-amd64 hosts. If platform is already
// set on the config, or the code can't generate a matching platform,
// do nothing.
func defaultPlatformIfNil(platform *image.Platform) *image.Platform {
	if platform == nil {
		p, err := image.NewPlatform(fmt.Sprintf("linux/%s", runtime.GOARCH))
		if err == nil {
			return p
		}
	}
	return platform
}

// matchesPlatform checks if the given platform matches the required platforms.
// The given platform matches the required platform if
// - architecture and OS are identical.
// - OS version and variant are identical if provided.
// - features and OS features of the required platform are subsets of those of the given platform.
// note: this function was copied from the GGCR repo, as it is not exported.
func matchesPlatform(given, required containerregistryV1.Platform) bool {
	// Required fields that must be identical.
	if given.Architecture != required.Architecture || given.OS != required.OS {
		return false
	}

	// Optional fields that may be empty, but must be identical if provided.
	if required.OSVersion != "" && given.OSVersion != required.OSVersion {
		return false
	}
	if required.Variant != "" && given.Variant != required.Variant {
		return false
	}

	// Verify required platform's features are a subset of given platform's features.
	if !isSubset(given.OSFeatures, required.OSFeatures) {
		return false
	}
	if !isSubset(given.Features, required.Features) {
		return false
	}

	return true
}

// isSubset checks if the required array of strings is a subset of the given lst.
// note: this function was copied from the GGCR repo, as it is not exported.
func isSubset(lst, required []string) bool {
	set := make(map[string]bool)
	for _, value := range lst {
		set[value] = true
	}

	for _, value := range required {
		if _, ok := set[value]; !ok {
			return false
		}
	}

	return true
}
