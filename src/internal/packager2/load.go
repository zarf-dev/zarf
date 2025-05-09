// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"encoding/json"
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
	Cluster                 *cluster.Cluster
	Source                  string
	Shasum                  string
	Architecture            string
	PublicKeyPath           string
	SkipSignatureValidation bool
	Filter                  filters.ComponentFilterStrategy
	LayersSelector          zoci.LayersSelector
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

	// Fetch or assemble the package tar
	isPartial, tarPath, err := fetchPackage(ctx, srcType, opt.Source, opt.Shasum, opt.Architecture, opt.LayersSelector, tmpDir, opt.Filter)
	if err != nil {
		return nil, err
	}

	// Verify checksum if provided
	if srcType != "oci" && opt.Shasum != "" {
		if err := helpers.SHAsMatch(tarPath, opt.Shasum); err != nil {
			return nil, fmt.Errorf("SHA256 mismatch for %s: %w", tarPath, err)
		}
	}

	// Load package layout
	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           opt.PublicKeyPath,
		SkipSignatureValidation: opt.SkipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  opt.Filter,
	}
	return layout.LoadFromTar(ctx, tarPath, layoutOpt)
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

// fetchPackage fetches or assembles the package tar for different source types.
func fetchPackage(ctx context.Context, srcType string, source string, shasum string, architecture string, layersSelector zoci.LayersSelector, workDir string, filter filters.ComponentFilterStrategy) (bool, string, error) {
	tarPath := filepath.Join(workDir, "data.tar.zst")
	switch srcType {
	case "oci":
		ociOpts := PullOCIOptions{
			Source:         source,
			Directory:      workDir,
			Shasum:         shasum,
			Architecture:   config.GetArch(architecture),
			Filter:         filter,
			LayersSelector: layersSelector,
		}

		return pullOCI(ctx, ociOpts)

	case "http", "https":
		path, err := pullHTTP(ctx, source, workDir, shasum)
		return false, path, err

	case "split":
		err := assembleSplitTar(source, tarPath)
		return false, tarPath, err

	case "tarball":
		return false, source, nil

	default:
		err := fmt.Errorf("cannot fetch or locate tarball for unsupported source type %s", srcType)
		return false, "", err
	}
}

// assembleSplitTar reconstructs a split tarball into a single archive.
func assembleSplitTar(src, dest string) error {
	pattern := strings.Replace(src, ".part000", ".part*", 1)
	splitFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find split tarball files: %w", err)
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
	return nil
}

func GetPackageFromSourceOrCluster(ctx context.Context, cluster *cluster.Cluster, src string, skipSignatureValidation bool, publicKeyPath string) (v1alpha1.ZarfPackage, error) {
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
	}
	p, err := LoadPackage(ctx, loadOpt)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	//nolint: errcheck // ignore
	defer p.Cleanup()
	return p.Pkg, nil
}
