// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

// LoadOptions are the options for LoadPackage.
type LoadOptions struct {
	Source                  string
	Shasum                  string
	Architecture            string
	PublicKeyPath           string
	SkipSignatureValidation bool
	Filter                  filters.ComponentFilterStrategy
	LayersSelector          zoci.LayersSelector
	Output                  string
}

// LoadPackage fetches, verifies, and loads a Zarf package from the specified source.
func LoadPackage(ctx context.Context, opt LoadOptions) (*layout.PackageLayout, error) {
	if opt.Filter == nil {
		opt.Filter = filters.Empty()
	}

	if opt.LayersSelector == "" {
		opt.LayersSelector = zoci.AllLayers
	}

	srcType, err := identifySource(opt.Source)
	if err != nil {
		return nil, err
	}

	// Prepare a temp workspace
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpDir)

	isPartial := false
	tmpPath := filepath.Join(tmpDir, "data.tar.zst")
	switch srcType {
	case "oci":
		ociOpts := PullOCIOptions{
			Source:         opt.Source,
			Directory:      tmpDir,
			Shasum:         opt.Shasum,
			Architecture:   config.GetArch(opt.Architecture),
			Filter:         opt.Filter,
			LayersSelector: opt.LayersSelector,
		}

		isPartial, tmpPath, err = pullOCI(ctx, ociOpts)
		if err != nil {
			return nil, err
		}
	case "http", "https":
		tmpPath, err = pullHTTP(ctx, opt.Source, tmpDir, opt.Shasum)
		if err != nil {
			return nil, err
		}
	case "split":
		// If there is not already a target output, then output to the same directory so the split file can become a single tar
		if opt.Output == "" {
			opt.Output = filepath.Dir(opt.Source)
		}
		err := assembleSplitTar(opt.Source, tmpPath)
		if err != nil {
			return nil, err
		}
	case "tarball":
		tmpPath = opt.Source
	default:
		err := fmt.Errorf("cannot fetch or locate tarball for unsupported source type %s", srcType)
		return nil, err
	}

	// Verify checksum if provided
	if srcType != "oci" && opt.Shasum != "" {
		if err := helpers.SHAsMatch(tmpPath, opt.Shasum); err != nil {
			return nil, fmt.Errorf("SHA256 mismatch for %s: %w", tmpPath, err)
		}
	}

	// Load package layout
	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           opt.PublicKeyPath,
		SkipSignatureValidation: opt.SkipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  opt.Filter,
	}
	pkgLayout, err := layout.LoadFromTar(ctx, tmpPath, layoutOpt)
	if err != nil {
		return nil, err
	}

	if opt.Output != "" {
		name, err := nameFromMetadata(ctx, tmpPath)
		if err != nil {
			return nil, err
		}
		tarPath := filepath.Join(opt.Output, name)
		err = os.Remove(tarPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		dstFile, err := os.Create(tarPath)
		if err != nil {
			return nil, err
		}
		defer func() {
			if dstErr := dstFile.Close(); dstErr != nil {
				err = fmt.Errorf("unable to cleanup: %w", dstErr)
			}
		}()
		srcFile, err := os.Open(tmpPath)
		if err != nil {
			return nil, err
		}
		// TODO(mkcp): add to error chain
		defer srcFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return nil, err
		}
	}

	return pkgLayout, nil
}

// identifySource returns the source type for the given source string.
func identifySource(src string) (string, error) {
	if parsed, err := url.Parse(src); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Scheme, nil
	}
	if strings.HasSuffix(src, ".tar.zst") || strings.HasSuffix(src, ".tar") {
		return "tarball", nil
	}
	if strings.Contains(src, ".part000") {
		return "split", nil
	}
	// match deployed package names: lowercase, digits, hyphens
	if lint.IsLowercaseNumberHyphenNoStartHyphen(src) {
		return "cluster", nil
	}
	return "", fmt.Errorf("unknown source %s", src)
}

// assembleSplitTar reconstructs a split tarball into a single archive.
func assembleSplitTar(src, dest string) error {
	pattern := strings.Replace(src, ".part000", ".part*", 1)
	splitFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find split tarball files: %w", err)
	}
	if len(splitFiles) == 0 {
		return fmt.Errorf("no split files with pattern %s found", pattern)
	}
	slices.Sort(splitFiles)

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	for i, part := range splitFiles {
		if i == 0 {
			// validate metadata
			data, err := os.ReadFile(part)
			if err != nil {
				return err
			}
			var meta types.ZarfSplitPackageData
			err = json.Unmarshal(data, &meta)
			if err != nil {
				return err
			}
			expected := len(splitFiles) - 1
			if meta.Count != expected {
				return fmt.Errorf("split parts mismatch: expected %d, got %d", expected, meta.Count)
			}
			continue
		}

		f, err := os.Open(part)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	for _, file := range splitFiles {
		err := os.Remove(file)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetPackageFromSourceOrCluster retrieves a Zarf package from a source or cluster.
func GetPackageFromSourceOrCluster(ctx context.Context, cluster *cluster.Cluster, src string, skipSignatureValidation bool, publicKeyPath string, layerSelector zoci.LayersSelector) (v1alpha1.ZarfPackage, error) {
	srcType, err := identifySource(src)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	if srcType == "cluster" {
		if cluster == nil {
			return v1alpha1.ZarfPackage{}, fmt.Errorf("cannot get Zarf package from Kubernetes without configuration")
		}
		depPkg, err := cluster.GetDeployedPackage(ctx, src)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
		return depPkg.Data, nil
	}

	loadOpt := LoadOptions{
		Source:                  src,
		SkipSignatureValidation: skipSignatureValidation,
		Architecture:            config.GetArch(),
		Filter:                  filters.Empty(),
		PublicKeyPath:           publicKeyPath,
		LayersSelector:          layerSelector,
	}
	p, err := LoadPackage(ctx, loadOpt)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	//nolint: errcheck // ignore
	defer p.Cleanup()
	return p.Pkg, nil
}
