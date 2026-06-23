// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/oci"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/split"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

// PackageDigestOptions are the options for PackageDigest.
type PackageDigestOptions struct {
	Architecture  string
	RemoteOptions types.RemoteOptions
}

// PackageDigest returns the SHA256 OCI manifest digest for the given package source.
// For OCI sources the digest is resolved directly from the registry without downloading
// the package. For local tarballs the manifest is computed deterministically from the
// package contents, producing the same digest that would result from publishing with
// PushPackage.
func PackageDigest(ctx context.Context, source string, opts PackageDigestOptions) (string, error) {
	srcType, err := identifySource(source)
	if err != nil {
		return "", err
	}

	switch srcType {
	case "oci":
		platform := oci.PlatformForArch(config.GetArch(opts.Architecture))
		remote, err := zoci.NewRemote(ctx, source, platform,
			oci.WithPlainHTTP(opts.RemoteOptions.PlainHTTP),
			oci.WithInsecureSkipVerify(opts.RemoteOptions.InsecureSkipTLSVerify))
		if err != nil {
			return "", fmt.Errorf("unable to connect to OCI registry: %w", err)
		}
		desc, err := remote.ResolveRoot(ctx)
		if err != nil {
			return "", fmt.Errorf("unable to resolve package digest: %w", err)
		}
		return desc.Digest.String(), nil

	case "split":
		tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return "", fmt.Errorf("unable to create temp directory: %w", err)
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				logger.From(ctx).Warn("failed to remove temp directory", "path", tmpDir, "error", err)
			}
		}()
		tmpPath := filepath.Join(tmpDir, "data.tar.zst")
		if err := split.ReassembleFile(source, tmpPath); err != nil {
			return "", fmt.Errorf("unable to reassemble split package: %w", err)
		}
		source = tmpPath
		fallthrough

	case "tarball":
		pkgLayout, err := layout.LoadFromTar(ctx, source, layout.PackageLayoutOptions{
			Filter: filters.Empty(),
		})
		if err != nil {
			return "", fmt.Errorf("unable to load package: %w", err)
		}
		defer func() {
			if err := pkgLayout.Cleanup(); err != nil {
				logger.From(ctx).Warn("failed to cleanup package layout", "error", err)
			}
		}()
		return zoci.DigestForLayout(ctx, pkgLayout)

	default:
		return "", fmt.Errorf("digest is not supported for source type %q", srcType)
	}
}
