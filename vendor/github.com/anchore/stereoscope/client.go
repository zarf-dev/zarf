package stereoscope

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/wagoodman/go-partybus"

	"github.com/anchore/go-collections"
	"github.com/anchore/go-logger"
	"github.com/anchore/stereoscope/internal/bus"
	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

var rootTempDirGenerator = file.NewTempDirGenerator("stereoscope")

func WithRegistryOptions(options image.RegistryOptions) Option {
	return func(c *config) error {
		c.Registry = options
		return nil
	}
}

func WithInsecureSkipTLSVerify() Option {
	return func(c *config) error {
		c.Registry.InsecureSkipTLSVerify = true
		return nil
	}
}

func WithInsecureAllowHTTP() Option {
	return func(c *config) error {
		c.Registry.InsecureUseHTTP = true
		return nil
	}
}

func WithCredentials(credentials ...image.RegistryCredentials) Option {
	return func(c *config) error {
		c.Registry.Credentials = append(c.Registry.Credentials, credentials...)
		return nil
	}
}

func WithAdditionalMetadata(metadata ...image.AdditionalMetadata) Option {
	return func(c *config) error {
		c.AdditionalMetadata = append(c.AdditionalMetadata, metadata...)
		return nil
	}
}

func WithPlatform(platform string) Option {
	return func(c *config) error {
		p, err := image.NewPlatform(platform)
		if err != nil {
			return err
		}
		c.Platform = p
		return nil
	}
}

// GetImage parses the user provided image string and provides an image object;
// note: the source where the image should be referenced from is automatically inferred.
func GetImage(ctx context.Context, imgStr string, options ...Option) (*image.Image, error) {
	// look for a known source scheme like docker:
	source, imgStr := ExtractSchemeSource(imgStr, allProviderTags()...)
	return getImageFromSource(ctx, imgStr, source, options...)
}

// GetImageFromSource returns an image from the explicitly provided source.
func GetImageFromSource(ctx context.Context, imgStr string, source image.Source, options ...Option) (*image.Image, error) {
	if source == "" {
		return nil, fmt.Errorf("source not provided, please specify a valid source tag")
	}
	return getImageFromSource(ctx, imgStr, source, options...)
}

func getImageFromSource(ctx context.Context, imgStr string, source image.Source, options ...Option) (*image.Image, error) {
	log.Debugf("image: source=%+v location=%+v", source, imgStr)

	// apply ImageProviderConfig config
	cfg := config{}
	if err := applyOptions(&cfg, options...); err != nil {
		return nil, err
	}

	// select image provider
	providers := collections.TaggedValueSet[image.Provider]{}.Join(
		ImageProviders(ImageProviderConfig{
			UserInput: imgStr,
			Platform:  cfg.Platform,
			Registry:  cfg.Registry,
		})...,
	)
	if source != "" {
		source = strings.ToLower(strings.TrimSpace(source))
		providers = providers.Select(source)
		if len(providers) == 0 {
			return nil, fmt.Errorf("unable to find image providers matching: '%s'", source)
		}
	}

	var errs []error
	for _, provider := range providers.Values() {
		img, err := provider.Provide(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		if img != nil {
			err = applyAdditionalMetadata(img, cfg.AdditionalMetadata...)
			return img, err
		}
	}
	return nil, fmt.Errorf("unable to detect input for '%s', errs: %w", imgStr, errors.Join(errs...))
}

func SetLogger(logger logger.Logger) {
	log.Log = logger
}

func SetBus(b *partybus.Bus) {
	bus.SetPublisher(b)
}

// Cleanup deletes all directories created by stereoscope calls.
//
// Deprecated: please use image.Image.Cleanup() over this.
func Cleanup() {
	if err := rootTempDirGenerator.Cleanup(); err != nil {
		log.Errorf("failed to cleanup tempdir root: %w", err)
	}
}
