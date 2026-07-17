package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/images/archive"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/core/remotes/docker/config"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/platforms"
	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/stereoscope/internal/bus"
	containerdClient "github.com/anchore/stereoscope/internal/containerd"
	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/event"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
	stereoscopeDocker "github.com/anchore/stereoscope/pkg/image/docker"
)

const Daemon image.Source = image.ContainerdDaemonSource

// NewDaemonProvider creates a new provider instance for a specific image that will later be cached to the given directory.
func NewDaemonProvider(tmpDirGen *file.TempDirGenerator, registryOptions image.RegistryOptions, namespace string, imageStr string, platform *image.Platform) image.Provider {
	if namespace == "" {
		namespace = namespaces.Default
	}

	return &daemonImageProvider{
		imageStr:        imageStr,
		tmpDirGen:       tmpDirGen,
		platform:        platform,
		namespace:       namespace,
		registryOptions: registryOptions,
	}
}

var mb = math.Pow(2, 20)

// daemonImageProvider is an image.Provider capable of fetching and representing a docker image from the containerd daemon API
type daemonImageProvider struct {
	imageStr        string
	tmpDirGen       *file.TempDirGenerator
	platform        *image.Platform
	namespace       string
	registryOptions image.RegistryOptions
}

func (p *daemonImageProvider) Name() string {
	return Daemon
}

type daemonProvideProgress struct {
	EstimateProgress *progress.TimedProgress
	ExportProgress   *progress.Manual
	Stage            *progress.Stage
}

func (p *daemonImageProvider) Provide(ctx context.Context) (*image.Image, error) {
	startTime := time.Now()
	client, err := containerdClient.GetClient()
	if err != nil {
		return nil, fmt.Errorf("containerd not available: %w", err)
	}

	defer func() {
		if err := client.Close(); err != nil {
			log.Errorf("unable to close containerd client: %+v", err)
		}
	}()

	ctx = namespaces.WithNamespace(ctx, p.namespace)

	resolvedImage, resolvedPlatform, err := p.pullImageIfMissing(ctx, client)
	if err != nil {
		return nil, err
	}

	log.WithFields("image", p.imageStr, "time", time.Since(startTime)).Info("containerd pulled image")
	startTime = time.Now()

	tarFileName, err := p.saveImage(ctx, client, resolvedImage)
	if err != nil {
		return nil, err
	}

	log.WithFields("image", p.imageStr, "time", time.Since(startTime)).Info("containerd saved image")

	// use the existing tarball provider to process what was pulled from the containerd daemon
	return stereoscopeDocker.NewArchiveProvider(p.tmpDirGen, tarFileName, withMetadata(resolvedPlatform, p.imageStr)...).
		Provide(ctx)
}

// pull a containerd image
func (p *daemonImageProvider) pull(ctx context.Context, c *client.Client, resolvedImage string) (client.Image, error) {
	var platformStr string
	if p.platform != nil {
		platformStr = p.platform.String()
	}

	// note: if not platform is provided then containerd will default to linux/amd64 automatically. We don't override
	// this behavior here and intentionally show that the value is blank in the log.
	log.WithFields("image", resolvedImage, "platform", platformStr).Debug("pulling containerd")

	ongoing := newJobs(resolvedImage)

	// publish a pull event on the bus, allowing for read-only consumption of status
	bus.Publish(partybus.Event{
		Type:   event.PullContainerdImage,
		Source: resolvedImage,
		Value:  newPullStatus(c, ongoing).start(ctx),
	})

	h := images.HandlerFunc(func(_ context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		// as new layers (and other artifacts) are discovered, add them to the ongoing list of things to monitor while pulling
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ongoing.Add(desc)
		}
		return nil, nil
	})

	ref, err := name.ParseReference(p.imageStr, prepareReferenceOptions(p.registryOptions)...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse registry reference=%q: %+v", p.imageStr, err)
	}

	options, err := p.pullOptions(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare pull options: %w", err)
	}
	options = append(options, client.WithImageHandler(h))

	// note: this will return an image object with the platform correctly set (if it exists)
	resp, err := c.Pull(ctx, resolvedImage, options...)
	if err != nil {
		return nil, fmt.Errorf("pull failed: %w", err)
	}

	return resp, nil
}

func (p *daemonImageProvider) pullOptions(ctx context.Context, ref name.Reference) ([]client.RemoteOpt, error) {
	options := []client.RemoteOpt{
		client.WithPlatform(p.platform.String()),
	}

	dockerOptions := docker.ResolverOptions{
		Tracker: docker.NewInMemoryTracker(),
	}

	if p.registryOptions.Keychain != nil {
		log.Warn("keychain registry option provided but is not supported for containerd daemon image provider")
	}

	var hostOptions config.HostOptions

	if len(p.registryOptions.Credentials) > 0 {
		hostOptions.Credentials = func(host string) (string, string, error) {
			// TODO: how should a bearer token be handled here?

			auth := p.registryOptions.Authenticator(host)
			if auth != nil {
				cfg, err := auth.Authorization()
				if err != nil {
					return "", "", fmt.Errorf("unable to get credentials for host=%q: %w", host, err)
				}
				log.WithFields("registry", host).Trace("found credentials")
				return cfg.Username, cfg.Password, nil
			}
			log.WithFields("registry", host).Trace("no credentials found")
			return "", "", nil
		}
	}

	switch p.registryOptions.InsecureUseHTTP {
	case true:
		hostOptions.DefaultScheme = "http"
	default:
		hostOptions.DefaultScheme = "https"
	}

	registryName := ref.Context().RegistryStr()

	tlsConfig, err := p.registryOptions.TLSConfig(registryName)
	if err != nil {
		return nil, fmt.Errorf("unable to get TLS config for registry=%q: %w", registryName, err)
	}

	if tlsConfig != nil {
		hostOptions.DefaultTLS = tlsConfig
	}

	dockerOptions.Hosts = config.ConfigureHosts(ctx, hostOptions)

	options = append(options, client.WithResolver(docker.NewResolver(dockerOptions)))

	return options, nil
}

func (p *daemonImageProvider) resolveImage(ctx context.Context, client *client.Client, imageStr string) (string, *ocispec.Platform, error) {
	// check if the image exists locally

	// note: you can NEVER depend on the GetImage() call to return an object with a platform set (even if you specify
	// a reference to a specific manifest via digest... not a digest for a manifest list!).
	img, err := client.GetImage(ctx, imageStr)
	if err != nil {
		// no image found
		return imageStr, nil, err
	}

	if p.platform == nil {
		// the user is not asking for a platform-specific request -- return what containerd returns
		return imageStr, nil, nil
	}

	processManifest := func(imageStr string, manifestDesc ocispec.Descriptor) (string, *ocispec.Platform, error) {
		manifest, err := p.fetchManifest(ctx, client, manifestDesc)
		if err != nil {
			return "", nil, err
		}

		platform, err := p.fetchPlatformFromConfig(ctx, client, manifest.Config)
		if err != nil {
			return "", nil, err
		}

		return imageStr, platform, nil
	}

	// let's make certain that the image we found is for the platform we want (but the hard way!)
	desc := img.Target()
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		return processManifest(imageStr, desc)

	case images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
		img = nil

		// let's find the digest for the manifest for the specific architecture we want
		by, err := content.ReadBlob(ctx, client.ContentStore(), desc)
		if err != nil {
			return "", nil, fmt.Errorf("unable to fetch manifest list: %w", err)
		}

		var index ocispec.Index
		if err := json.Unmarshal(by, &index); err != nil {
			return "", nil, fmt.Errorf("unable to unmarshal manifest list: %w", err)
		}

		platformObj, err := platforms.Parse(p.platform.String())
		if err != nil {
			return "", nil, fmt.Errorf("unable to parse platform: %w", err)
		}
		platformMatcher := platforms.NewMatcher(platformObj)
		for _, manifestDesc := range index.Manifests {
			if manifestDesc.Platform == nil {
				continue
			}
			if platformMatcher.Match(*manifestDesc.Platform) {
				return processManifest(imageStr, manifestDesc)
			}
		}

		// no manifest found for the platform we want
		return imageStr, nil, fmt.Errorf("no manifest found in manifest list for platform %q", p.platform.String())
	}

	return "", nil, fmt.Errorf("unexpected mediaType for image: %q", desc.MediaType)
}

func (p *daemonImageProvider) fetchManifest(ctx context.Context, client *client.Client, desc ocispec.Descriptor) (*ocispec.Manifest, error) {
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		// pass
	default:
		return nil, fmt.Errorf("unexpected mediaType for image manifest: %q", desc.MediaType)
	}

	by, err := content.ReadBlob(ctx, client.ContentStore(), desc)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch image manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(by, &manifest); err != nil {
		return nil, fmt.Errorf("unable to unmarshal image manifest: %w", err)
	}

	return &manifest, nil
}

func (p *daemonImageProvider) fetchPlatformFromConfig(ctx context.Context, client *client.Client, desc ocispec.Descriptor) (*platforms.Platform, error) {
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
		// pass
	default:
		return nil, fmt.Errorf("unexpected mediaType for image config: %q", desc.MediaType)
	}

	by, err := content.ReadBlob(ctx, client.ContentStore(), desc)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch image config: %w", err)
	}

	var cfg ocispec.Platform
	if err := json.Unmarshal(by, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal platform info from image config: %w", err)
	}

	return &cfg, nil
}

func (p *daemonImageProvider) pullImageIfMissing(ctx context.Context, client *client.Client) (string, *platforms.Platform, error) {
	p.imageStr = ensureRegistryHostPrefix(p.imageStr)

	// try to get the image first before pulling
	resolvedImage, resolvedPlatform, err := p.resolveImage(ctx, client, p.imageStr)

	imageStr := resolvedImage
	if imageStr == "" {
		imageStr = p.imageStr
	}

	if err != nil {
		_, err := p.pull(ctx, client, imageStr)
		if err != nil {
			return "", nil, err
		}

		resolvedImage, resolvedPlatform, err = p.resolveImage(ctx, client, imageStr)
		if err != nil {
			return "", nil, fmt.Errorf("unable to resolve image after pull: %w", err)
		}
	}

	if err := validatePlatform(p.platform, resolvedPlatform); err != nil {
		return "", nil, fmt.Errorf("platform validation failed: %w", err)
	}

	return resolvedImage, resolvedPlatform, nil
}

func validatePlatform(expected *image.Platform, given *platforms.Platform) error {
	if expected == nil {
		return nil
	}

	if given == nil {
		return newErrPlatformMismatch(expected, fmt.Errorf("image has no platform information (might be a manifest list)"))
	}

	if given.OS != expected.OS {
		return newErrPlatformMismatch(expected, fmt.Errorf("image has unexpected OS %q, which differs from the user specified PS %q", given.OS, expected.OS))
	}

	if given.Architecture != expected.Architecture {
		return newErrPlatformMismatch(expected, fmt.Errorf("image has unexpected architecture %q, which differs from the user specified architecture %q", given.Architecture, expected.Architecture))
	}

	if given.Variant != expected.Variant {
		return newErrPlatformMismatch(expected, fmt.Errorf("image has unexpected architecture %q, which differs from the user specified architecture %q", given.Variant, expected.Variant))
	}

	return nil
}

func newErrPlatformMismatch(expected *image.Platform, err error) *image.ErrPlatformMismatch {
	return &image.ErrPlatformMismatch{
		ExpectedPlatform: expected.String(),
		Err:              err,
	}
}

// save the image from the containerd daemon to a tar file
func (p *daemonImageProvider) saveImage(ctx context.Context, client *client.Client, resolvedImage string) (string, error) {
	imageTempDir, err := p.tmpDirGen.NewDirectory("containerd-daemon-image")
	if err != nil {
		return "", err
	}

	// create a file within the temp dir
	tempTarFile, err := os.Create(path.Join(imageTempDir, "image.tar"))
	if err != nil {
		return "", fmt.Errorf("unable to create temp file for image: %w", err)
	}
	defer func() {
		err := tempTarFile.Close()
		if err != nil {
			log.Errorf("unable to close temp file (%s): %w", tempTarFile.Name(), err)
		}
	}()

	is := client.ImageService()
	exportOpts := []archive.ExportOpt{
		archive.WithImage(is, resolvedImage),
	}

	img, err := client.GetImage(ctx, resolvedImage)
	if err != nil {
		return "", fmt.Errorf("unable to fetch image from containerd: %w", err)
	}

	size, err := img.Size(ctx)
	if err != nil {
		log.WithFields("error", err).Trace("unable to fetch image size from containerd, progress will not be tracked accurately")
		size = int64(50 * mb)
	}

	platformComparer, err := exportPlatformComparer(p.platform)
	if err != nil {
		return "", err
	}

	exportOpts = append(exportOpts, archive.WithPlatform(platformComparer))

	providerProgress := p.trackSaveProgress(size)
	defer func() {
		// NOTE: progress trackers should complete at the end of this function
		// whether the function errors or succeeds.
		providerProgress.EstimateProgress.SetCompleted()
		providerProgress.ExportProgress.SetCompleted()
	}()

	providerProgress.Stage.Current = "requesting image from containerd"

	// containerd export (save) does not return till fully complete
	err = client.Export(ctx, tempTarFile, exportOpts...)
	if err != nil {
		return "", fmt.Errorf("unable to save image tar for image=%q: %w", img.Name(), err)
	}

	return tempTarFile.Name(), nil
}

func exportPlatformComparer(platform *image.Platform) (platforms.MatchComparer, error) {
	// it is important to only export a single architecture. Default to linux/amd64. Without specifying a specific
	// architecture then the export may include multiple architectures (if the tag points to a manifest list)
	platformStr := "linux/amd64"
	if platform != nil {
		platformStr = platform.String()
	}

	platformObj, err := platforms.Parse(platformStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse platform: %w", err)
	}

	// important: we require OnlyStrict() to ensure that when arm64 is provided that other arm variants are NOT selected
	return platforms.OnlyStrict(platformObj), nil
}

func (p *daemonImageProvider) trackSaveProgress(size int64) *daemonProvideProgress {
	// docker image save clocks in at ~40MB/sec on my laptop... mileage may vary, of course :shrug:
	sec := float64(size) / (mb * 40)
	approxSaveTime := time.Duration(sec*1000) * time.Millisecond

	estimateSaveProgress := progress.NewTimedProgress(approxSaveTime)
	exportProgress := progress.NewManual(1)
	aggregateProgress := progress.NewAggregator(progress.DefaultStrategy, estimateSaveProgress, exportProgress)

	// let consumers know of a monitorable event (image save + copy stages)
	stage := &progress.Stage{}

	bus.Publish(partybus.Event{
		Type:   event.FetchImage,
		Source: p.imageStr,
		Value: progress.StagedProgressable(&struct {
			progress.Stager
			progress.Progressable
		}{
			Stager:       progress.Stager(stage),
			Progressable: aggregateProgress,
		}),
	})

	return &daemonProvideProgress{
		EstimateProgress: estimateSaveProgress,
		ExportProgress:   exportProgress,
		Stage:            stage,
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

func withMetadata(platform *platforms.Platform, ref string) (metadata []image.AdditionalMetadata) {
	if platform != nil {
		metadata = append(metadata,
			image.WithArchitecture(platform.Architecture, platform.Variant),
			image.WithOS(platform.OS),
		)
	}

	if strings.Contains(ref, ":") {
		// remove digest from ref
		metadata = append(metadata, image.WithTags(strings.Split(ref, "@")[0]))
	}
	return metadata
}

// if imageName doesn't have an identifiable hostname prefix set,
// add docker hub by default
func ensureRegistryHostPrefix(imageName string) string {
	parts := strings.Split(imageName, "/")
	if len(parts) == 1 {
		return fmt.Sprintf("docker.io/library/%s", imageName)
	}
	if isRegistryHostname(parts[0]) {
		return imageName
	}
	return fmt.Sprintf("docker.io/%s", imageName)
}

// isRegistryHostname returns true if the string passed in can be interpreted
// as a container registry hostname
func isRegistryHostname(s string) bool {
	return s == "localhost" || strings.Contains(s, ".") || strings.Contains(s, ":")
}
