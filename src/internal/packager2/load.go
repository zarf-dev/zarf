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
	"github.com/zarf-dev/zarf/src/internal/packager2/filters"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

// LoadOptions are the options for LoadPackage.
type LoadOptions struct {
	Shasum                  string
	Architecture            string
	PublicKeyPath           string
	SkipSignatureValidation bool
	Filter                  filters.ComponentFilterStrategy
	Output                  string
	// number of layers to pull in parallel
	OCIConcurrency int
	// Layers to pull during OCI pull
	LayersSelector zoci.LayersSelector
	// Only applicable to OCI + HTTP
	RemoteOptions
}

// LoadPackage fetches, verifies, and loads a Zarf package from the specified source.
func LoadPackage(ctx context.Context, source string, opts LoadOptions) (_ *layout.PackageLayout, err error) {
	if source == "" {
		return nil, fmt.Errorf("must provide a package source")
	}
	if opts.Filter == nil {
		opts.Filter = filters.Empty()
	}

	if opts.LayersSelector == "" {
		opts.LayersSelector = zoci.AllLayers
	}

	srcType, err := identifySource(source)
	if err != nil {
		return nil, err
	}

	// Prepare a temp workspace
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	isPartial := false
	tmpPath := filepath.Join(tmpDir, "data.tar.zst")
	switch srcType {
	case "oci":
		ociOpts := pullOCIOptions{
			Source:         source,
			Directory:      tmpDir,
			Shasum:         opts.Shasum,
			Architecture:   config.GetArch(opts.Architecture),
			Filter:         opts.Filter,
			LayersSelector: opts.LayersSelector,
			OCIConcurrency: opts.OCIConcurrency,
			RemoteOptions:  opts.RemoteOptions,
		}

		isPartial, tmpPath, err = pullOCI(ctx, ociOpts)
		if err != nil {
			return nil, err
		}
	case "http", "https":
		tmpPath, err = pullHTTP(ctx, source, tmpDir, opts.Shasum, opts.InsecureSkipTLSVerify)
		if err != nil {
			return nil, err
		}
	case "split":
		// If there is not already a target output, then output to the same directory so the split file can become a single tar
		if opts.Output == "" {
			opts.Output = filepath.Dir(source)
		}
		err := assembleSplitTar(source, tmpPath)
		if err != nil {
			return nil, err
		}
	case "tarball":
		tmpPath = source
	default:
		err := fmt.Errorf("cannot fetch or locate tarball for unsupported source type %s", srcType)
		return nil, err
	}

	// Verify checksum if provided
	if srcType != "oci" && opts.Shasum != "" {
		if err := helpers.SHAsMatch(tmpPath, opts.Shasum); err != nil {
			return nil, fmt.Errorf("SHA256 mismatch for %s: %w", tmpPath, err)
		}
	}

	// Load package layout
	layoutOpts := layout.PackageLayoutOptions{
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  opts.Filter,
	}
	pkgLayout, err := layout.LoadFromTar(ctx, tmpPath, layoutOpts)
	if err != nil {
		return nil, err
	}

	if opts.Output != "" {
		filename, err := pkgLayout.FileName()
		if err != nil {
			return nil, err
		}
		tarPath := filepath.Join(opts.Output, filename)
		err = os.Remove(tarPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		dstFile, err := os.Create(tarPath)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = errors.Join(err, dstFile.Close())
		}()
		srcFile, err := os.Open(tmpPath)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = errors.Join(err, srcFile.Close())
		}()
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
func assembleSplitTar(src, dest string) (err error) {
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
	defer func() {
		err = errors.Join(err, out.Close())
	}()

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

		// Create a new scope for the file so the defer close happens during each loop rather than once the function completes
		err := func() (err error) {
			f, err := os.Open(part)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, f.Close())
			}()

			_, err = io.Copy(out, f)
			return err
		}()
		if err != nil {
			return err
		}
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
func GetPackageFromSourceOrCluster(ctx context.Context, cluster *cluster.Cluster, src string, opts LoadOptions) (_ v1alpha1.ZarfPackage, err error) {
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
	// This function only returns the ZarfPackageConfig so we only need the metadata
	opts.LayersSelector = zoci.MetadataLayers
	pkgLayout, err := LoadPackage(ctx, src, opts)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()
	return pkgLayout.Pkg, nil
}
