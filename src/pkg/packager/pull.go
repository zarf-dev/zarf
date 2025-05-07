// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

// PullOptions declares optional configuration for a Pull operation.
type PullOptions struct {
	// OutputDirectory specifies where on disk to write the pulled package.
	OutputDirectory string
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

// Pull fetches the Zarf package from the given sources.
func Pull(ctx context.Context, src string, opts PullOptions) error {
	l := logger.From(ctx)
	start := time.Now()

	// ensure filters are set
	f := opts.Filters
	if f == nil {
		f = filters.Empty()
	}
	// ensure architecture is set
	arch := config.GetArch(opts.Architecture)

	u, err := url.Parse(src)
	if err != nil {
		return err
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
		l.Info("starting pull from oci source", "src", src)
		isPartial, tmpPath, err = packager2.PullOCI(ctx, src, tmpDir, opts.SHASum, arch, f)
		if err != nil {
			return err
		}
	case "http", "https":
		l.Info("starting pull from http(s) source", "src", src, "digest", opts.SHASum)
		tmpPath, err = packager2.PullHTTP(ctx, src, tmpDir, opts.SHASum)
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

	name, err := nameFromMetadata(tmpPath)
	if err != nil {
		return err
	}
	tarPath := filepath.Join(opts.OutputDirectory, name)
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

	l.Debug("done packager2.Pull", "src", src, "dir", opts.OutputDirectory, "duration", time.Since(start))
	return nil
}

func nameFromMetadata(path string) (string, error) {
	var pkg v1alpha1.ZarfPackage
	// TODO(mkcp): See https://github.com/zarf-dev/zarf/issues/3051
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
