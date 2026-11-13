package oci

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	containerregistryV1Types "github.com/google/go-containerregistry/pkg/v1/types"

	async "github.com/anchore/go-sync"
	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const Registry image.Source = image.OciRegistrySource

// effectiveURLTransport is a custom transport that captures the effective URL after following redirects
type effectiveURLTransport struct {
	base          http.RoundTripper
	effectiveURLs map[string]string // maps original host to effective host
	mutex         sync.RWMutex
}

func newEffectiveURLTransport(base http.RoundTripper) *effectiveURLTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &effectiveURLTransport{
		base:          base,
		effectiveURLs: make(map[string]string),
	}
}

func (t *effectiveURLTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	originalHost := req.URL.Host
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Check if the effective URL is different from the original
	if resp.Request != nil && resp.Request.URL != nil {
		effectiveHost := resp.Request.URL.Host
		if effectiveHost != originalHost {
			t.mutex.Lock()
			t.effectiveURLs[originalHost] = effectiveHost
			t.mutex.Unlock()
			log.Debugf("captured effective URL after redirect: %s -> %s", originalHost, effectiveHost)
		}
	}

	return resp, err
}

func (t *effectiveURLTransport) getEffectiveHost(originalHost string) string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	if effectiveHost, exists := t.effectiveURLs[originalHost]; exists {
		return effectiveHost
	}
	return originalHost
}

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
	tmpDirGen          *file.TempDirGenerator
	imageStr           string
	platform           *image.Platform
	registryOptions    image.RegistryOptions
	effectiveTransport *effectiveURLTransport
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

	// Initialize the effective URL transport if not already done
	if p.effectiveTransport == nil {
		p.effectiveTransport = newEffectiveURLTransport(nil)
	}

	options := prepareRemoteOptions(ctx, ref, p.registryOptions, platform, p.effectiveTransport)

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
	// Use the effective registry host if it differs from the original due to redirects
	originalRegistry := ref.Context().RegistryStr()
	effectiveRegistry := p.effectiveTransport.getEffectiveHost(originalRegistry)
	repoDigest := fmt.Sprintf("%s/%s@%s", effectiveRegistry, ref.Context().RepositoryStr(), descriptor.Digest.String())

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

	// registry layers arrive over the network: download them one at a time, since a second stream
	// only splits a link a single stream already saturates, while already-fetched layers are
	// indexed in parallel at the default bound. A caller that installed its own fetch executor
	// knows better than we do, so leave theirs alone.
	if !async.HasContextExecutor(ctx, image.LayerFetchExecutor) {
		ctx = async.SetContextExecutor(ctx, image.LayerFetchExecutor, async.NewExecutor(1))
	}

	out := image.New(img, p.tmpDirGen, imageTempDir, metadata...)
	err = out.Read(ctx)
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

func prepareReferenceOptions(registryOptions image.RegistryOptions) []name.Option {
	var options []name.Option
	if registryOptions.InsecureUseHTTP {
		log.Debug("HTTP transport is enabled for registry communication")
		options = append(options, name.Insecure)
	}
	return options
}

func prepareRemoteOptions(ctx context.Context, ref name.Reference, registryOptions image.RegistryOptions, p *image.Platform, effectiveTransport *effectiveURLTransport) (options []remote.Option) {
	options = append(options, remote.WithContext(ctx))

	// Set the user agent to indicate what binary is making the request
	// (e.g. syft, grype)
	userAgent := os.Args[0]
	options = append(options, remote.WithUserAgent(userAgent))

	if p != nil {
		options = append(options, remote.WithPlatform(*toContainerRegistryPlatform(p)))
	}

	registryName := ref.Context().RegistryStr()

	// note: the authn.Authenticator and authn.Keychain options are mutually exclusive, only one may be provided.
	// If no explicit authenticator can be found, check if explicit Keychain has been provided, and if not, then
	// fallback to the default keychain. With the authenticator also comes the option to configure TLS transport.
	authenticator := registryOptions.Authenticator(registryName)
	keychain := registryOptions.Keychain

	switch {
	case authenticator != nil:
		options = append(options, remote.WithAuth(authenticator))
	case keychain != nil:
		options = append(options, remote.WithAuthFromKeychain(keychain))
	default:
		// use the Keychain specified from a docker config file.
		log.Debugf("no registry credentials configured for %q, using the default keychain", registryName)
		keychain = authn.DefaultKeychain
		options = append(options, remote.WithAuthFromKeychain(keychain))
	}

	tlsConfig, err := registryOptions.TLSConfig(registryName)
	if err != nil {
		log.Warnf("unable to configure TLS transport: %v", err)
	}

	// one base transport, so the pull and any resume share a connection pool
	baseTransport := getTransport(tlsConfig)

	// a resumed blob is reissued through its own client, authenticated the same way as the pull
	var reopen reopenClientFunc
	if !registryOptions.DisableBlobResume {
		reopen = reopenClient(ref, baseTransport, userAgent, func(ctx context.Context) (authn.Authenticator, error) {
			if authenticator != nil {
				return authenticator, nil
			}
			return authn.Resolve(ctx, keychain, ref.Context())
		})
	}

	// Use our custom transport that captures effective URLs after redirects
	rt := getTransportWithEffectiveURL(baseTransport, effectiveTransport, reopen)
	options = append(options, remote.WithTransport(rt))

	return options
}

// reopenClient lazily builds the client a resumed blob request is reissued through. it authenticates
// against the registry the way go-containerregistry does, so a resume gets a fresh token and a freshly
// signed redirect rather than ones that lapsed during a long transfer. unlike go-containerregistry's
// client it has no retry transport, since the resume's own stall budget already retries.
func reopenClient(ref name.Reference, base http.RoundTripper, userAgent string, auth func(context.Context) (authn.Authenticator, error)) reopenClientFunc {
	// one slot held across the registry ping, so concurrent first resumes wait on a single build. a
	// channel rather than a mutex, so a resume closed or cancelled meanwhile stops waiting
	slot := make(chan struct{}, 1)
	var (
		client *http.Client
		// a refusal is settled once for the whole pull. it will not mend, and every blob that drops
		// would otherwise repeat the ping and token exchange to be told the same thing
		refused error
	)

	return func(ctx context.Context) (*http.Client, error) {
		select {
		case slot <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		defer func() { <-slot }()

		if client != nil || refused != nil {
			return client, refused
		}

		// only the registry rejecting our credentials is settled for the pull. anything else spends a
		// stall and is tried again, and is quoted by resume when it does. the pull already resolved
		// credentials once, so a helper failing now is most likely the same network trouble that dropped
		// the blob. a refusal is quoted here, since this is where text the registry chose crosses into
		// ours, and go-containerregistry puts up to 64 KiB of an error response body into its message
		authenticator, err := auth(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve credentials: %w", err)
		}

		repo := ref.Context()
		rt, err := transport.NewWithContext(ctx, repo.Registry, authenticator, transport.NewUserAgent(base, userAgent),
			[]string{repo.Scope(transport.PullScope)})
		// matched on status rather than Temporary: a registry pinged over both https and http reports
		// both failures, and a transient one on the scheme that works can sit beside a permanent-looking
		// one (a TLS port answering plain http with 400) on the scheme that does not
		var regErr *transport.Error
		if errors.As(err, &regErr) && (regErr.StatusCode == http.StatusUnauthorized || regErr.StatusCode == http.StatusForbidden) {
			refused = fmt.Errorf("%w: unable to authenticate: %w", errResumeRefused, quoted(err))
			return nil, refused
		}
		if err != nil {
			return nil, err
		}

		client = &http.Client{Transport: rt, CheckRedirect: checkRedirect}

		return client, nil
	}
}

func getTransport(tlsConfig *tls.Config) *http.Transport {
	// use the default transport to inherit existing default options (including proxy options)
	rt := http.DefaultTransport.(*http.Transport).Clone()
	rt.TLSClientConfig = tlsConfig
	return rt
}

func getTransportWithEffectiveURL(baseTransport *http.Transport, effectiveTransport *effectiveURLTransport, reopen reopenClientFunc) http.RoundTripper {
	// resume large blob downloads that the registry drops mid-stream. this must sit closest to
	// the base transport so that it wraps the real response body, and below go-containerregistry's
	// digest verification so that a reassembled layer is still verified end to end
	var base http.RoundTripper = baseTransport
	if reopen != nil {
		base = newResumableTransport(baseTransport, reopen)
	}

	// wrap it with our effective URL capturing transport
	effectiveTransport.base = base
	return effectiveTransport
}
