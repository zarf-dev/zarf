// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mholt/archives"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/gabriel-vasile/mimetype"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

// PullOptions declares optional configuration for a Pull operation.
type PullOptions struct {
	// SHASum uniquely identifies a package based on its contents.
	SHASum string
	// SkipSignatureValidation flags whether Pull should skip validating the signature.
	SkipSignatureValidation bool
	// Architecture is the package architecture.
	Architecture string
	// Filters describes a Filter strategy to include or exclude certain components from the package.
	Filters filters.ComponentFilterStrategy
	// PublicKeyPath validates the create-time signage of a package.
	PublicKeyPath string
}

// Pull takes a source URL and destination directory and fetches the Zarf package from the given sources.
func Pull(ctx context.Context, source, destination string, opts PullOptions) error {
	l := logger.From(ctx)
	start := time.Now()

	// ensure filters are set
	f := opts.Filters
	if f == nil {
		f = filters.Empty()
	}
	// ensure architecture is set
	arch := config.GetArch(opts.Architecture)

	u, err := url.Parse(source)
	if err != nil {
		return err
	}
	if destination == "" {
		return fmt.Errorf("no output directory specified")
	}
	if u.Scheme == "" {
		return errors.New("scheme must be either oci:// or http(s)://")
	}
	if u.Host == "" {
		return errors.New("host cannot be empty")
	}

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer func() {
		if rErr := os.Remove(tmpDir); rErr != nil {
			err = fmt.Errorf("cleanup failed: %w", rErr)
		}
	}()
	tmpPath := ""

	isPartial := false
	switch u.Scheme {
	case "oci":
		ociOpts := PullOCIOptions{
			Source:                  source,
			Directory:               tmpDir,
			Shasum:                  opts.SHASum,
			Architecture:            arch,
			PublicKeyPath:           opts.PublicKeyPath,
			LayersSelector:          zoci.AllLayers,
			SkipSignatureValidation: opts.SkipSignatureValidation,
			Filter:                  f,
			Modifiers:               []oci.Modifier{},
		}
		l.Info("starting pull from oci source", "source", source)
		isPartial, tmpPath, err = pullOCI(ctx, ociOpts)
		if err != nil {
			return err
		}
	case "http", "https":
		l.Info("starting pull from http(s) source", "src", source, "digest", opts.SHASum)
		tmpPath, err = pullHTTP(ctx, source, tmpDir, opts.SHASum)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown scheme %s", u.Scheme)
	}

	// This loadFromTar is done so that validatePackageIntegrtiy and validatePackageSignature are called
	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  f,
	}
	_, err = layout.LoadFromTar(ctx, tmpPath, layoutOpt)
	if err != nil {
		return err
	}

	name, err := nameFromMetadata(ctx, tmpPath)
	if err != nil {
		return err
	}
	tarPath := filepath.Join(destination, name)
	err = os.Remove(tarPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dstFile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer func() {
		if dstErr := dstFile.Close(); dstErr != nil {
			err = fmt.Errorf("unable to cleanup: %w", dstErr)
		}
	}()
	srcFile, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	// TODO(mkcp): add to error chain
	defer srcFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	l.Debug("done packager2.Pull", "source", source, "destination", destination, "duration", time.Since(start))
	return nil
}

// PullOptions are the options for PullPackage.
type PullOCIOptions struct {
	Source                  string
	Directory               string
	Shasum                  string
	Architecture            string
	PublicKeyPath           string
	LayersSelector          zoci.LayersSelector
	SkipSignatureValidation bool
	Filter                  filters.ComponentFilterStrategy
	Modifiers               []oci.Modifier
}

func pullOCI(ctx context.Context, opts PullOCIOptions) (bool, string, error) {
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return false, "", err
	}
	defer os.Remove(tmpDir)
	if opts.Shasum != "" {
		opts.Source = fmt.Sprintf("%s@sha256:%s", opts.Source, opts.Shasum)
	}
	platform := oci.PlatformForArch(opts.Architecture)
	remote, err := zoci.NewRemote(ctx, opts.Source, platform, opts.Modifiers...)
	if err != nil {
		return false, "", err
	}
	desc, err := remote.ResolveRoot(ctx)
	if err != nil {
		return false, "", fmt.Errorf("could not find package %s with architecture %s: %w", opts.Source, platform.Architecture, err)
	}
	isPartial := false
	tarPath := filepath.Join(opts.Directory, "data.tar")
	pkg, err := remote.FetchZarfYAML(ctx)
	if err != nil {
		return false, "", err
	}
	if !pkg.Metadata.Uncompressed {
		tarPath = fmt.Sprintf("%s.zst", tarPath)
	}
	if supportsFiltering(desc.Platform) {
		pkg.Components, err = opts.Filter.Apply(pkg)
		if err != nil {
			return false, "", err
		}
	}

	// zarf creates layers around the contents of component primarily
	// this assembles the layers for the components - whether filtered above or not
	layersToPull, err := remote.AssembleLayers(ctx, pkg.Components, isSkeleton(desc.Platform), opts.LayersSelector)
	if err != nil {
		return false, "", err
	}

	root, err := remote.FetchRoot(ctx)
	if err != nil {
		return false, "", err
	}
	if len(root.Layers) != len(layersToPull) {
		isPartial = true
	}

	_, err = remote.PullPackage(ctx, tmpDir, config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return false, "", err
	}
	allTheLayers, err := filepath.Glob(filepath.Join(tmpDir, "*"))
	if err != nil {
		return false, "", err
	}
	err = archive.Compress(ctx, allTheLayers, tarPath, archive.CompressOpts{})
	if err != nil {
		return false, "", err
	}
	return isPartial, tarPath, nil
}

func pullHTTP(ctx context.Context, src, tarDir, shasum string) (string, error) {
	if shasum == "" {
		return "", errors.New("shasum cannot be empty")
	}
	tarPath := filepath.Join(tarDir, "data")

	err := pullHTTPFile(ctx, src, tarPath)
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

func pullHTTPFile(ctx context.Context, src, tarPath string) error {
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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

// nameFromMetadata reads the zarf.yaml inside the archive at "path"
// (which may be plain, .tar, .tar.zst, .zip, etc) and builds its package name.
func nameFromMetadata(ctx context.Context, path string) (string, error) {
	// 1) quick invalid‐path check
	if helpers.InvalidPath(path) {
		return "", &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// 2) mount the archive as a virtual file system
	fsys, err := archives.FileSystem(ctx, path, nil)
	if err != nil {
		return "", fmt.Errorf("unable to open archive %q: %w", path, err)
	}

	// 3) open just the zarf.yaml entry
	f, err := fsys.Open(layout.ZarfYAML)
	if err != nil {
		return "", fmt.Errorf("%s does not contain a %s", path, layout.ZarfYAML)
	}
	defer f.Close()

	// 4) read & unmarshal into our package struct
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	var pkg v1alpha1.ZarfPackage
	if err := goyaml.Unmarshal(data, &pkg); err != nil {
		return "", err
	}
	if pkg.Metadata.Name == "" {
		return "", fmt.Errorf("%s does not contain a zarf.yaml", path)
	}

	// 5) build the output name exactly as before
	arch := config.GetArch(pkg.Metadata.Architecture, pkg.Build.Architecture)
	if pkg.Build.Architecture == zoci.SkeletonArch {
		arch = zoci.SkeletonArch
	}

	var name string
	switch pkg.Kind {
	case v1alpha1.ZarfInitConfig:
		name = fmt.Sprintf("zarf-init-%s", arch)
	case v1alpha1.ZarfPackageConfig:
		name = fmt.Sprintf("zarf-package-%s-%s", pkg.Metadata.Name, arch)
	default:
		name = fmt.Sprintf("zarf-%s-%s", strings.ToLower(string(pkg.Kind)), arch)
	}
	if pkg.Build.Differential {
		name = fmt.Sprintf("%s-%s-differential-%s",
			name, pkg.Build.DifferentialPackageVersion, pkg.Metadata.Version)
	} else if pkg.Metadata.Version != "" {
		name = fmt.Sprintf("%s-%s", name, pkg.Metadata.Version)
	}

	// 6) choose tar vs tar.zst
	if pkg.Metadata.Uncompressed {
		return name + ".tar", nil
	}
	return name + ".tar.zst", nil
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
