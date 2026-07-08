package oci

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	containerregistryV1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	containerregistryV1Types "github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const Registry image.Source = image.OciRegistrySource

// NewRegistryProvider creates a new provider instance for a specific image that will later be cached to the given directory.
func NewRegistryProvider(tmpDirGen *file.TempDirGenerator, registryOptions image.RegistryOptions, imageStr string, platform *image.Platform) image.Provider {
	return &registryImageProvider{
		tmpDirGen:       tmpDirGen,
		imageStr:        imageStr,
		platform:        platform,
		registryOptions: registryOptions,
	}
}

// registryImageProvider is an image.Provider capable of fetching and representing a container image fetched from a remote registry (described by the OCI distribution spec).
type registryImageProvider struct {
	tmpDirGen       *file.TempDirGenerator
	imageStr        string
	platform        *image.Platform
	registryOptions image.RegistryOptions
}

func (p *registryImageProvider) Name() string {
	return Registry
}

// Provide an image object that represents the cached docker image tar fetched a registry.
func (p *registryImageProvider) Provide(ctx context.Context) (*image.Image, error) {
	log.Debugf("pulling image info directly from registry image=%q", p.imageStr)

	startTime := time.Now()
	imageTempDir, err := p.tmpDirGen.NewDirectory("oci-registry-image")
	if err != nil {
		return nil, err
	}

	ref, err := name.ParseReference(p.imageStr, prepareReferenceOptions(p.registryOptions)...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse registry reference=%q: %+v", p.imageStr, err)
	}

	platform := defaultPlatformIfNil(p.platform)

	options := prepareRemoteOptions(ctx, ref, p.registryOptions, platform)

	descriptor, err := remote.Get(ref, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to get image descriptor from registry: %+v", err)
	}

	p.finalizePlatform(descriptor, &platform)

	img, err := descriptor.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to get image from registry: %+v", err)
	}

	c, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get image config from registry: %+v", err)
	}

	if err := validatePlatform(platform, c.OS, c.Architecture, c.Variant); err != nil {
		return nil, err
	}

	log.WithFields("image", p.imageStr, "time", time.Since(startTime)).Info("completed downloading manifest")

	// craft a repo digest from the registry reference and the known digest
	// note: the descriptor is fetched from the registry, and the descriptor digest is the same as the repo digest
	repoDigest := fmt.Sprintf("%s/%s@%s", ref.Context().RegistryStr(), ref.Context().RepositoryStr(), descriptor.Digest.String())

	metadata := []image.AdditionalMetadata{
		image.WithRepoDigests(repoDigest),
	}

	// make a best effort to get the manifest, should not block getting an image though if it fails
	if manifestBytes, err := img.RawManifest(); err == nil {
		metadata = append(metadata, image.WithManifest(manifestBytes))
	}

	if platform != nil {
		metadata = append(metadata,
			image.WithArchitecture(platform.Architecture, platform.Variant),
			image.WithOS(platform.OS),
		)
	}

	out := image.New(img, p.tmpDirGen, imageTempDir, metadata...)
	err = out.Read()
	if err != nil {
		cleanErr := out.Cleanup()
		return nil, errors.Join(err, cleanErr)
	}
	return out, err
}

func (p *registryImageProvider) finalizePlatform(descriptor *remote.Descriptor, platform **image.Platform) {
	if p.platform != nil {
		return
	}

	// no platform was specified by the user. There are two cases we want to cover:
	// 1. there is a manifest list, in which case we want to default the architecture to the host's architecture
	// 2. there is a single platform image, in which case we want to use that architecture (specify no default)
	switch descriptor.MediaType {
	case containerregistryV1Types.OCIManifestSchema1, containerregistryV1Types.DockerManifestSchema1, containerregistryV1Types.DockerManifestSchema2:
		// this is not for a multi-platform image, do not force the architecture if a platform was not specified explicitly by the user
		*platform = nil
		descriptor.Platform = nil
	}
}

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

func prepareReferenceOptions(registryOptions image.RegistryOptions) []name.Option {
	var options []name.Option
	if registryOptions.InsecureUseHTTP {
		log.Debug("HTTP transport is enabled for registry communication")
		options = append(options, name.Insecure)
	}
	return options
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

func prepareRemoteOptions(ctx context.Context, ref name.Reference, registryOptions image.RegistryOptions, p *image.Platform) (options []remote.Option) {
	options = append(options, remote.WithContext(ctx))

	// Set the user agent to indicate what binary is making the request
	// (e.g. syft, grype)
	options = append(options, remote.WithUserAgent(os.Args[0]))

	if p != nil {
		options = append(options, remote.WithPlatform(*toContainerRegistryPlatform(p)))
	}

	registryName := ref.Context().RegistryStr()

	// note: the authn.Authenticator and authn.Keychain options are mutually exclusive, only one may be provided.
	// If no explicit authenticator can be found, check if explicit Keychain has been provided, and if not, then
	// fallback to the default keychain. With the authenticator also comes the option to configure TLS transport.
	authenticator := registryOptions.Authenticator(registryName)

	switch {
	case authenticator != nil:
		options = append(options, remote.WithAuth(authenticator))
	case registryOptions.Keychain != nil:
		options = append(options, remote.WithAuthFromKeychain(registryOptions.Keychain))
	default:
		// use the Keychain specified from a docker config file.
		log.Debugf("no registry credentials configured for %q, using the default keychain", registryName)
		options = append(options, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	tlsConfig, err := registryOptions.TLSConfig(registryName)
	if err != nil {
		log.Warn("unable to configure TLS transport: %w", err)
	} else if tlsConfig != nil {
		options = append(options, remote.WithTransport(getTransport(tlsConfig)))
	}

	return options
}

func getTransport(tlsConfig *tls.Config) *http.Transport {
	// use the default transport to inherit existing default options (including proxy options)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return transport
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
