// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/gabriel-vasile/mimetype"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

// PullOptions declares optional configuration for a Pull operation.
type PullOptions struct {
	// SHASum uniquely identifies a package based on its contents.
	SHASum string
	// Verify validates the package signature
	Verify bool
	// Architecture is the package architecture.
	Architecture string
	// PublicKeyPath validates the create-time signage of a package.
	PublicKeyPath string
	// OCIConcurrency is the number of layers pulled in parallel
	OCIConcurrency int
	// CachePath is used to cache layers from OCI package pulls
	CachePath string
	RemoteOptions
}

// Pull takes a source URL and destination directory, fetches the Zarf package from the given sources, and returns the path to the fetched package.
func Pull(ctx context.Context, source, destination string, opts PullOptions) (_ string, err error) {
	l := logger.From(ctx)
	start := time.Now()

	// ensure architecture is set
	arch := config.GetArch(opts.Architecture)

	u, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	if destination == "" {
		return "", fmt.Errorf("no output directory specified")
	}
	if u.Scheme == "" {
		return "", errors.New("scheme must be either oci:// or http(s)://")
	}
	if u.Host == "" {
		return "", errors.New("host cannot be empty")
	}

	pkgLayout, err := LoadPackage(ctx, source, LoadOptions{
		Shasum:         opts.SHASum,
		Architecture:   arch,
		PublicKeyPath:  opts.PublicKeyPath,
		Verify:         opts.Verify,
		Output:         destination,
		OCIConcurrency: opts.OCIConcurrency,
		RemoteOptions:  opts.RemoteOptions,
		CachePath:      opts.CachePath,
	})
	if err != nil {
		return "", err
	}
	if err := pkgLayout.Cleanup(); err != nil {
		return "", err
	}
	filename, err := pkgLayout.FileName()
	if err != nil {
		return "", err
	}
	filepath := filepath.Join(destination, filename)
	l.Debug("done packager.Pull", "source", source, "destination", destination, "duration", time.Since(start))
	return filepath, nil
}

type pullOCIOptions struct {
	Source         string
	Shasum         string
	Architecture   string
	LayersSelector zoci.LayersSelector
	Filter         filters.ComponentFilterStrategy
	OCIConcurrency int
	CachePath      string
	PublicKeyPath  string
	RemoteOptions
	Verify bool
}

func pullOCI(ctx context.Context, opts pullOCIOptions) (*layout.PackageLayout, error) {
	if opts.Shasum != "" {
		opts.Source = fmt.Sprintf("%s@sha256:%s", opts.Source, opts.Shasum)
	}
	cacheMod, err := zoci.GetOCICacheModifier(ctx, opts.CachePath)
	if err != nil {
		return nil, err
	}
	platform := oci.PlatformForArch(opts.Architecture)
	remote, err := zoci.NewRemote(ctx, opts.Source, platform, oci.WithPlainHTTP(opts.PlainHTTP), oci.WithInsecureSkipVerify(opts.InsecureSkipTLSVerify), cacheMod)
	if err != nil {
		return nil, err
	}
	desc, err := remote.ResolveRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not find package %s with architecture %s: %w", opts.Source, platform.Architecture, err)
	}
	isPartial := false
	pkg, err := remote.FetchZarfYAML(ctx)
	if err != nil {
		return nil, err
	}
	if supportsFiltering(desc.Platform) {
		pkg.Components, err = opts.Filter.Apply(pkg)
		if err != nil {
			return nil, err
		}
	}

	// zarf creates layers around the contents of component primarily
	// this assembles the layers for the components - whether filtered above or not
	layersToPull, err := remote.AssembleLayers(ctx, pkg.Components, isSkeleton(desc.Platform), opts.LayersSelector)
	if err != nil {
		return nil, err
	}

	root, err := remote.FetchRoot(ctx)
	if err != nil {
		return nil, err
	}
	if len(root.Layers) != len(layersToPull) {
		isPartial = true
	}
	dirPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	_, err = remote.PullPackage(ctx, dirPath, opts.OCIConcurrency, layersToPull...)
	if err != nil {
		return nil, err
	}
	layoutOpts := layout.PackageLayoutOptions{
		PublicKeyPath: opts.PublicKeyPath,
		Verify:        opts.Verify,
		IsPartial:     isPartial,
		Filter:        opts.Filter,
	}
	pkgLayout, err := layout.LoadFromDir(ctx, dirPath, layoutOpts)
	if err != nil {
		return nil, err
	}
	return pkgLayout, nil
}

func pullHTTP(ctx context.Context, src, tarDir, shasum string, insecureTLSSkipVerify bool) (string, error) {
	if shasum == "" {
		return "", errors.New("shasum cannot be empty")
	}
	tarPath := filepath.Join(tarDir, "data")

	err := pullHTTPFile(ctx, src, tarPath, insecureTLSSkipVerify)
	if err != nil {
		return "", err
	}

	received, err := helpers.GetSHA256OfFile(tarPath)
	if err != nil {
		return "", err
	}
	if received != shasum {
		return "", fmt.Errorf("shasum mismatch for file %s, expected %s but got %s", tarPath, shasum, received)
	}

	mtype, err := mimetype.DetectFile(tarPath)
	if err != nil {
		return "", err
	}

	newPath := filepath.Join(tarDir, "data.tar")

	if mtype.Is("application/x-tar") {
		err = os.Rename(tarPath, newPath)
		if err != nil {
			return "", err
		}
		return newPath, nil
	} else if mtype.Is("application/zstd") {
		newPath = fmt.Sprintf("%s.zst", newPath)
		err = os.Rename(tarPath, newPath)
		if err != nil {
			return "", err
		}
		return newPath, nil
	}
	return "", fmt.Errorf("unsupported file type: %s", mtype.Extension())
}

func pullHTTPFile(ctx context.Context, src, tarPath string, insecureTLSSkipVerify bool) (err error) {
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return errors.New("could not get default transport")
	}
	transport = transport.Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecureTLSSkipVerify}
	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	if resp.StatusCode != http.StatusOK {
		_, err := io.Copy(io.Discard, resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected http response status code %s for source %s", resp.Status, src)
	}
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

// supportsFiltering checks if the package supports filtering.
// This is true if the package is not a skeleton package and the platform is not nil.
func supportsFiltering(platform *ocispec.Platform) bool {
	if platform == nil {
		return false
	}
	if isSkeleton(platform) {
		return false
	}
	return true
}

// isSkeleton checks if the package is explicitly a skeleton package.
func isSkeleton(platform *ocispec.Platform) bool {
	if platform == nil {
		return false
	}
	skeletonPlatform := zoci.PlatformForSkeleton()
	if platform.Architecture == skeletonPlatform.Architecture && platform.OS == skeletonPlatform.OS {
		return true
	}
	return false
}
