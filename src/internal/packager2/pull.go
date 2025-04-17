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

	"github.com/zarf-dev/zarf/src/pkg/logger"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/gabriel-vasile/mimetype"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

// TODO: Add options struct
// Pull fetches the Zarf package from the given sources.
func Pull(ctx context.Context, src, dir, shasum, architecture string, filter filters.ComponentFilterStrategy, publicKeyPath string, skipSignatureValidation bool) error {
	if filter == nil {
		filter = filters.Empty()
	}
	l := logger.From(ctx)
	start := time.Now()
	u, err := url.Parse(src)
	if err != nil {
		return err
	}
	if u.Scheme == "" {
		return errors.New("scheme cannot be empty")
	}
	if u.Host == "" {
		return errors.New("host cannot be empty")
	}
	// ensure architecture is set
	architecture = config.GetArch(architecture)

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer os.Remove(tmpDir)
	tmpPath := ""

	isPartial := false
	switch u.Scheme {
	case "oci":
		ociOpts := PullOCIOptions{
			Source:                  src,
			Directory:               tmpDir,
			Shasum:                  shasum,
			Architecture:            architecture,
			PublicKeyPath:           publicKeyPath,
			InspectTarget:           "",
			SkipSignatureValidation: skipSignatureValidation,
			Filter:                  filter,
			Modifiers:               []oci.Modifier{},
		}
		l.Info("starting pull from oci source", "src", src, "digest", shasum)
		isPartial, tmpPath, err = pullOCI(ctx, ociOpts)
		if err != nil {
			return err
		}
	case "http", "https":
		l.Info("starting pull from http(s) source", "src", src, "digest", shasum)
		tmpPath, err = pullHTTP(ctx, src, tmpDir, shasum)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown scheme %s", u.Scheme)
	}

	// This loadFromTar is done so that validatePackageIntegrtiy and validatePackageSignature are called
	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           publicKeyPath,
		SkipSignatureValidation: skipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  filter,
	}
	_, err = layout.LoadFromTar(ctx, tmpPath, layoutOpt)
	if err != nil {
		return err
	}

	name, err := nameFromMetadata(tmpPath)
	if err != nil {
		return err
	}
	tarPath := filepath.Join(dir, name)
	err = os.Remove(tarPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dstFile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	srcFile, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	l.Debug("done packager2.Pull", "src", src, "dir", dir, "duration", time.Since(start))
	return nil
}

// PullOptions are the options for PullPackage.
type PullOCIOptions struct {
	Source                  string
	Directory               string
	Shasum                  string
	Architecture            string
	PublicKeyPath           string
	InspectTarget           string
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
	layersToPull := []ocispec.Descriptor{}
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
		root, err := remote.FetchRoot(ctx)
		if err != nil {
			return false, "", err
		}
		if len(root.Layers) != len(layersToPull) {
			isPartial = true
		}
		pkg.Components, err = opts.Filter.Apply(pkg)
		if err != nil {
			return false, "", err
		}
		layersToPull, err = remote.LayersFromRequestedComponents(ctx, pkg.Components, opts.InspectTarget)
		if err != nil {
			return false, "", err
		}
	}
	_, err = remote.PullPackage(ctx, tmpDir, config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return false, "", err
	}
	allTheLayers, err := filepath.Glob(filepath.Join(tmpDir, "*"))
	if err != nil {
		return false, "", err
	}
	err = archiver.Archive(allTheLayers, tarPath)
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

func nameFromMetadata(path string) (string, error) {
	var pkg v1alpha1.ZarfPackage
	err := archiver.Walk(path, func(f archiver.File) error {
		if f.Name() == layout.ZarfYAML {
			b, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			if err := goyaml.Unmarshal(b, &pkg); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if pkg.Metadata.Name == "" {
		return "", fmt.Errorf("%s does not contain a zarf.yaml", path)
	}

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
		name = fmt.Sprintf("%s-%s-differential-%s", name, pkg.Build.DifferentialPackageVersion, pkg.Metadata.Version)
	} else if pkg.Metadata.Version != "" {
		name = fmt.Sprintf("%s-%s", name, pkg.Metadata.Version)
	}
	if pkg.Metadata.Uncompressed {
		return fmt.Sprintf("%s.tar", name), nil
	}
	return fmt.Sprintf("%s.tar.zst", name), nil
}

func supportsFiltering(platform *ocispec.Platform) bool {
	if platform == nil {
		return false
	}
	skeletonPlatform := zoci.PlatformForSkeleton()
	if platform.Architecture == skeletonPlatform.Architecture && platform.OS == skeletonPlatform.OS {
		return false
	}
	return true
}
