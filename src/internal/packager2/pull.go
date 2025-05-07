// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/gabriel-vasile/mimetype"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

func PullOCI(ctx context.Context, src, tarDir, shasum string, architecture string, filter filters.ComponentFilterStrategy, mods ...oci.Modifier) (bool, string, error) {
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return false, "", err
	}
	defer os.Remove(tmpDir)
	if shasum != "" {
		src = fmt.Sprintf("%s@sha256:%s", src, shasum)
	}
	platform := oci.PlatformForArch(architecture)
	remote, err := zoci.NewRemote(ctx, src, oci.PlatformForArch(architecture), mods...)
	if err != nil {
		return false, "", err
	}
	desc, err := remote.ResolveRoot(ctx)
	if err != nil {
		return false, "", fmt.Errorf("could not find package %s with architecture %s: %w", src, platform.Architecture, err)
	}
	layersToPull := []ocispec.Descriptor{}
	isPartial := false
	tarPath := filepath.Join(tarDir, "data.tar")
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
		pkg.Components, err = filter.Apply(pkg)
		if err != nil {
			return false, "", err
		}
		layersToPull, err = remote.LayersFromRequestedComponents(ctx, pkg.Components)
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
	// TODO(mkcp): See https://github.com/zarf-dev/zarf/issues/3051
	err = archiver.Archive(allTheLayers, tarPath)
	if err != nil {
		return false, "", err
	}
	return isPartial, tarPath, nil
}

func PullHTTP(ctx context.Context, src, tarDir, shasum string) (string, error) {
	if shasum == "" {
		return "", errors.New("shasum cannot be empty")
	}
	tarPath := filepath.Join(tarDir, "data")

	err := PullHTTPFile(ctx, src, tarPath)
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

func PullHTTPFile(ctx context.Context, src, tarPath string) error {
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
