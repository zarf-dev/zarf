package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/cli/cli/config"
	configTypes "github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/stereoscope/internal/bus"
	"github.com/anchore/stereoscope/internal/docker"
	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/event"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

const Daemon image.Source = image.DockerDaemonSource

// NewDaemonProvider creates a new provider instance for a specific image that will later be cached to the given directory
func NewDaemonProvider(tmpDirGen *file.TempDirGenerator, imageStr string, platform *image.Platform) image.Provider {
	return NewAPIClientProvider(Daemon, tmpDirGen, imageStr, platform, func() (client.APIClient, error) {
		return docker.GetClient()
	})
}

// NewAPIClientProvider creates a new provider for the provided Docker client.APIClient
func NewAPIClientProvider(name string, tmpDirGen *file.TempDirGenerator, imageStr string, platform *image.Platform, newClient apiClientCreator) image.Provider {
	return &daemonImageProvider{
		name:         name,
		tmpDirGen:    tmpDirGen,
		newAPIClient: newClient,
		imageStr:     imageStr,
		platform:     platform,
	}
}

type apiClientCreator func() (client.APIClient, error)

// daemonImageProvider is an image.Provider capable of fetching and representing a docker image from the docker daemon API
type daemonImageProvider struct {
	name         string
	tmpDirGen    *file.TempDirGenerator
	newAPIClient apiClientCreator
	imageStr     string
	platform     *image.Platform
}

func (p *daemonImageProvider) Name() string {
	return p.name
}

// ociPlatform converts the stereoscope platform to an OCI platform spec, for use with Docker API options.
func (p *daemonImageProvider) ociPlatform() *ocispec.Platform {
	if p.platform == nil {
		return nil
	}
	return &ocispec.Platform{
		OS:           p.platform.OS,
		Architecture: p.platform.Architecture,
		Variant:      p.platform.Variant,
	}
}

// platformInspect calls ImageInspect with platform selection when a platform is configured.
// Docker 29+ stores multi-platform images as manifest lists; without the platform parameter,
// inspect returns empty OS/Architecture fields for foreign-platform images. This method
// passes the platform to the API (available since API v1.49) and falls back to a plain
// inspect for older daemons.
func (p *daemonImageProvider) platformInspect(ctx context.Context, apiClient client.APIClient, imageRef string) (client.ImageInspectResult, error) {
	if p.platform != nil {
		result, err := apiClient.ImageInspect(ctx, imageRef, client.ImageInspectWithPlatform(p.ociPlatform()))
		if err == nil {
			return result, nil
		}
		// if the daemon doesn't support the platform option (older API), fall back to a plain inspect
		log.Debugf("platform-aware inspect failed, falling back to plain inspect: %v", err)
	}
	return apiClient.ImageInspect(ctx, imageRef)
}

// platformSave calls ImageSave with platform selection when a platform is configured.
// Docker 29+ stores multi-platform images as manifest lists; without the platform parameter,
// save may fail or return the wrong platform. This method passes the platform to the API
// (available since API v1.48) and falls back to a plain save for older daemons.
func (p *daemonImageProvider) platformSave(ctx context.Context, apiClient client.APIClient, imageRef string) (io.ReadCloser, error) {
	if p.platform != nil {
		result, err := apiClient.ImageSave(ctx, []string{imageRef}, client.ImageSaveWithPlatforms(*p.ociPlatform()))
		if err == nil {
			return result, nil
		}
		// if the daemon doesn't support the platform option (older API), fall back to a plain save
		log.Debugf("platform-aware save failed, falling back to plain save: %v", err)
	}
	return apiClient.ImageSave(ctx, []string{imageRef})
}

type daemonProvideProgress struct {
	SaveProgress *progress.TimedProgress
	CopyProgress *progress.Writer
	Stage        *progress.AtomicStage
}

func (p *daemonImageProvider) trackSaveProgress(ctx context.Context, apiClient client.APIClient, imageRef string) (*daemonProvideProgress, error) {
	// fetch the expected image size to estimate and measure progress
	inspect, err := p.platformInspect(ctx, apiClient, imageRef)
	if err != nil {
		return nil, fmt.Errorf("unable to inspect image: %w", err)
	}

	// docker image save clocks in at ~125MB/sec on my laptop... mileage may vary, of course :shrug:
	mb := math.Pow(2, 20)
	sec := float64(inspect.Size) / (mb * 125)
	approxSaveTime := time.Duration(sec*1000) * time.Millisecond

	estimateSaveProgress := progress.NewTimedProgress(approxSaveTime)
	copyProgress := progress.NewSizedWriter(inspect.Size)
	aggregateProgress := progress.NewAggregator(progress.NormalizeStrategy, estimateSaveProgress, copyProgress)

	// let consumers know of a monitorable event (image save + copy stages)
	stage := progress.NewAtomicStage("")

	bus.Publish(partybus.Event{
		Type:   event.FetchImage,
		Source: imageRef,
		Value: progress.StagedProgressable(&struct {
			progress.Stager
			*progress.Aggregator
		}{
			Stager:     progress.Stager(stage),
			Aggregator: aggregateProgress,
		}),
	})

	return &daemonProvideProgress{
		SaveProgress: estimateSaveProgress,
		CopyProgress: copyProgress,
		Stage:        stage,
	}, nil
}

// pull a docker image
func (p *daemonImageProvider) pull(ctx context.Context, client client.APIClient, imageRef string) error {
	log.Debugf("pulling %s image=%q", p.name, imageRef)

	status := newPullStatus()
	defer func() {
		status.complete = true
	}()

	// publish a pull event on the bus, allowing for read-only consumption of status
	bus.Publish(partybus.Event{
		Type:   event.PullDockerImage,
		Source: imageRef,
		Value:  status,
	})

	options, err := p.pullOptions(imageRef)
	if err != nil {
		return err
	}

	resp, err := client.ImagePull(ctx, imageRef, options)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	defer resp.Close()

	var thePullEvent *pullEvent
	decoder := json.NewDecoder(resp)
	for {
		if err := decoder.Decode(&thePullEvent); err != nil {
			if err == io.EOF {
				break
			}

			return fmt.Errorf("failed to pull image: %w", err)
		}
		if err := handlePullEvent(status, thePullEvent); err != nil {
			return err
		}
	}

	return nil
}

type emitter interface {
	onEvent(event *pullEvent)
}

func handlePullEvent(status emitter, event *pullEvent) error {
	if event.Error != "" {
		if strings.Contains(event.Error, "does not match the specified platform") {
			return &image.ErrPlatformMismatch{
				Err: errors.New(event.Error),
			}
		}
		return errors.New(event.Error)
	}

	// check for the last two events indicating the pull is complete
	if strings.HasPrefix(event.Status, "Digest:") || strings.HasPrefix(event.Status, "Status:") {
		return nil
	}

	status.onEvent(event)

	return nil
}

func (p *daemonImageProvider) pullOptions(imageRef string) (client.ImagePullOptions, error) {
	options := client.ImagePullOptions{}
	if p.platform != nil {
		options.Platforms = append(options.Platforms, ocispec.Platform{
			Architecture: p.platform.Architecture,
			OS:           p.platform.OS,
			Variant:      p.platform.Variant,
		})
	}

	// note: this will search the default config dir and allow for a DOCKER_CONFIG override
	cfg, err := config.Load("")
	if err != nil {
		return options, fmt.Errorf("failed to load docker config: %w", err)
	}
	log.Debugf("using docker config=%q", cfg.Filename)

	// get a URL that works with docker credential helpers
	url, err := authURL(imageRef, true)
	if err != nil {
		log.Warnf("failed to determine auth url from image=%q: %+v", imageRef, err)
		return options, nil
	}

	authConfig, err := cfg.GetAuthConfig(url)
	if err != nil {
		log.Warnf("failed to fetch registry auth (url=%s): %+v", url, err)
		return options, nil
	}

	empty := configTypes.AuthConfig{}
	if authConfig == empty {
		// we didn't find any entries in any auth sources. This might be because the workaround needed for the
		// docker credential helper was unnecessary (since the user isn't using a credential helper). For this reason
		// lets try this auth config lookup again, but this time for a url that doesn't consider the dockerhub
		// workaround for the credential helper.
		url, err = authURL(imageRef, false)
		if err != nil {
			log.Warnf("failed to determine auth url from image=%q: %+v", imageRef, err)
			return options, nil
		}

		authConfig, err = cfg.GetAuthConfig(url)
		if err != nil {
			log.Warnf("failed to fetch registry auth (url=%s): %+v", url, err)
			return options, nil
		}
	}

	log.Debugf("using docker credentials for %q", url)

	options.RegistryAuth, err = encodeCredentials(authConfig)
	if err != nil {
		log.Warnf("failed to encode registry auth (url=%s): %+v", url, err)
	}

	return options, nil
}

func authURL(imageRef string, dockerhubWorkaround bool) (string, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", err
	}

	url := ref.Context().RegistryStr()
	if dockerhubWorkaround && url == "index.docker.io" {
		// why do this? There is an upstream issue here: https://github.com/docker/docker-credential-helpers/blob/e595cd69465c6b0f7af2d49582b82fdeddecbf75/wincred/wincred_windows.go#L113-L127
		// where the hostname used for the auth config lookup requires this or else even pulling public images
		// will fail with auth related problems (bad username/password, bad personal access token, etc).
		// The above note only applies to the credential helper, not to auth entries directly written to the docker config.
		// For this reason callers need to try getting the authconfig for both v1 and non-v1 routes.
		url += "/v1/"
	}
	return url, nil
}

// Provide an image object that represents the cached docker image tar fetched from a docker daemon.
func (p *daemonImageProvider) Provide(ctx context.Context) (*image.Image, error) {
	startTime := time.Now()
	apiClient, err := p.newAPIClient()
	if err != nil {
		return nil, fmt.Errorf("%s not available: %w", p.name, err)
	}

	defer func() {
		if err := apiClient.Close(); err != nil {
			log.Errorf("unable to close %s client: %+v", p.name, err)
		}
	}()

	c2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pong, err := apiClient.Ping(c2, client.PingOptions{})
	if err != nil || pong.APIVersion == "" {
		return nil, fmt.Errorf("unable to get %s API response: %w", p.name, err)
	}

	log.WithFields("image", p.imageStr).Info("docker pulling image")
	imageRef, err := p.pullImageIfMissing(ctx, apiClient)
	if err != nil {
		return nil, err
	}

	log.WithFields("image", imageRef, "time", time.Since(startTime)).Info("docker pulled image")
	startTime = time.Now()

	// inspect the image that might have been pulled, using platform-aware inspect to resolve
	// the correct platform variant in Docker 29+ multi-platform image stores
	inspectResult, err := p.platformInspect(ctx, apiClient, imageRef)
	if err != nil {
		return nil, fmt.Errorf("unable to inspect existing image: %w", err)
	}

	// by this point the platform info should match based off of user input, so we should error out if this is not the case
	if err := p.validatePlatform(inspectResult); err != nil {
		return nil, err
	}

	log.WithFields("image", imageRef, "time", time.Since(startTime)).Trace("docker validated image")
	startTime = time.Now()

	tarFileName, err := p.saveImage(ctx, apiClient, imageRef)
	if err != nil {
		return nil, err
	}

	log.WithFields("image", imageRef, "time", time.Since(startTime), "path", tarFileName).Info("docker saved image")

	// use the existing tarball provider to process what was pulled from the docker daemon
	return NewArchiveProvider(p.tmpDirGen, tarFileName, withInspectMetadata(inspectResult)...).
		Provide(ctx)
}

func (p *daemonImageProvider) saveImage(ctx context.Context, apiClient client.APIClient, imageRef string) (string, error) {
	// save the image from the docker daemon to a tar file
	providerProgress, err := p.trackSaveProgress(ctx, apiClient, imageRef)
	if err != nil {
		return "", fmt.Errorf("unable to trace image save progress: %w", err)
	}
	defer func() {
		// NOTE: progress trackers should complete at the end of this function
		// whether the function errors or succeeds.
		providerProgress.SaveProgress.SetCompleted()
		providerProgress.CopyProgress.SetComplete()
	}()

	imageTempDir, err := p.tmpDirGen.NewDirectory(fmt.Sprintf("%s-daemon-image", p.name))
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

	providerProgress.Stage.Set(fmt.Sprintf("requesting image from %s", p.name))
	readCloser, err := p.platformSave(ctx, apiClient, imageRef)
	if err != nil {
		return "", fmt.Errorf("unable to save image tar: %w", err)
	}
	defer func() {
		err := readCloser.Close()
		if err != nil {
			log.Errorf("unable to close temp file (%s): %w", tempTarFile.Name(), err)
		}
	}()

	// NOTE: The image save progress is only a guess (a timer counting up to a particular time where
	// the overall progress would be considered at 50%). It's logical to adjust the first image save timer
	// to complete when the image save operation returns. The defer statement is a fallback in case the numbers
	// from the docker daemon don't line up (as we saw when metadata and actual size differ)
	// or there is a problem that causes us to return early with an error.
	providerProgress.SaveProgress.SetCompleted()

	// save the image contents to the temp file
	// note: this is the same image that will be used to querying image content during analysis
	providerProgress.Stage.Set("saving image to disk")
	nBytes, err := io.Copy(io.MultiWriter(tempTarFile, providerProgress.CopyProgress), readCloser)
	if err != nil {
		return "", fmt.Errorf("unable to save image to tar: %w", err)
	}
	if nBytes == 0 {
		return "", errors.New("cannot provide an empty image")
	}
	return tempTarFile.Name(), nil
}

func (p *daemonImageProvider) pullImageIfMissing(ctx context.Context, apiClient client.APIClient) (imageRef string, err error) {
	imageRef, originalImageRef, err := image.ParseReference(p.imageStr)
	if err != nil {
		return "", err
	}

	// check if the image exists locally (use platform-aware inspect so Docker 29+ resolves
	// the correct platform variant from a multi-platform manifest list)
	inspectResult, err := p.platformInspect(ctx, apiClient, imageRef)
	if err != nil {
		inspectResult, err = p.platformInspect(ctx, apiClient, originalImageRef)
		if err == nil {
			imageRef = strings.TrimSuffix(imageRef, ":latest")
		}
	}
	if err != nil {
		if errdefs.IsNotFound(err) {
			if err = p.pull(ctx, apiClient, imageRef); err != nil {
				return imageRef, err
			}
		} else {
			return imageRef, fmt.Errorf("unable to inspect existing image: %w", err)
		}
	} else {
		// looks like the image exists, but if the platform doesn't match what the user specified, we may need to
		// pull the image again with the correct platform specifier, which will override the local tag.
		if err = p.validatePlatform(inspectResult); err != nil {
			if err = p.pull(ctx, apiClient, imageRef); err != nil {
				return imageRef, err
			}
		}
	}
	return imageRef, nil
}

func (p *daemonImageProvider) validatePlatform(i client.ImageInspectResult) error {
	if p.platform == nil {
		// the user did not specify a platform
		return nil
	}

	if i.Os != p.platform.OS {
		return fmt.Errorf("image has unexpected OS %q, which differs from the user specified PS %q", i.Os, p.platform.OS)
	}

	if i.Architecture != p.platform.Architecture {
		return fmt.Errorf("image has unexpected architecture %q, which differs from the user specified architecture %q", i.Architecture, p.platform.Architecture)
	}

	// note: there is no architecture variant captured in inspect responses

	return nil
}

func withInspectMetadata(i client.ImageInspectResult) (metadata []image.AdditionalMetadata) {
	metadata = append(metadata,
		image.WithTags(i.RepoTags...),
		image.WithRepoDigests(i.RepoDigests...),
		image.WithArchitecture(i.Architecture, ""), // since we don't have variant info from the image directly, we don't report it
		image.WithOS(i.Os),
	)
	return metadata
}

func encodeCredentials(authConfig configTypes.AuthConfig) (string, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	// note: the contents may contain characters that should not be escaped (such as password contents)
	encoder.SetEscapeHTML(false)

	//nolint:gosec // G117: encoding auth config with password is required for Docker registry authentication
	if err := encoder.Encode(authConfig); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buffer.Bytes()), nil
}
