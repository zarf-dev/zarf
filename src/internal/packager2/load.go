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
	"github.com/mholt/archiver/v3"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
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
	Inspect                 bool
}

// LoadPackage optionally fetches and loads the package from the given source.
func LoadPackage(ctx context.Context, opt LoadOptions) (*layout.PackageLayout, error) {
	srcType, err := identifySource(opt.Source)
	if err != nil {
		return nil, err
	}
	architecture := config.GetArch(opt.Architecture)

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpDir)
	tarPath := filepath.Join(tmpDir, "data.tar.zst")

	isPartial := false
	switch srcType {
	case "oci":
		// this is a special case during inspect. do not pull the full package as it may be very large
		if opt.Inspect {
			path, err := pullOCIMetadata(ctx, opt.Source, tmpDir, opt.Shasum, architecture)
			if err != nil {
				return nil, err
			}
			layoutOpt := layout.PackageLayoutOptions{
				PublicKeyPath:           opt.PublicKeyPath,
				SkipSignatureValidation: opt.SkipSignatureValidation,
				IsPartial:               isPartial,
				Inspect:                 true,
			}
			pkgLayout, err := layout.LoadFromDir(ctx, path, layoutOpt)
			if err != nil {
				return nil, err
			}
			return pkgLayout, nil
		}
		isPartial, tarPath, err = pullOCI(ctx, opt.Source, tmpDir, opt.Shasum, architecture, opt.Filter)
		if err != nil {
			return nil, err
		}
	case "http", "https":
		tarPath, err = pullHTTP(ctx, opt.Source, tmpDir, opt.Shasum)
		if err != nil {
			return nil, err
		}
	case "split":
		err = assembleSplitTar(opt.Source, tarPath)
		if err != nil {
			return nil, err
		}
	case "tarball":
		tarPath = opt.Source
	default:
		return nil, fmt.Errorf("unknown source type: %s", opt.Source)
	}
	if srcType != "oci" && opt.Shasum != "" {
		err := helpers.SHAsMatch(tarPath, opt.Shasum)
		if err != nil {
			return nil, err
		}
	}

	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           opt.PublicKeyPath,
		SkipSignatureValidation: opt.SkipSignatureValidation,
		IsPartial:               isPartial,
	}
	pkgLayout, err := layout.LoadFromTar(ctx, tarPath, layoutOpt)
	if err != nil {
		return nil, err
	}
	return pkgLayout, nil
}

// identifySource returns the source type for the given source.
func identifySource(src string) (string, error) {
	parsed, err := url.Parse(src)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Scheme, nil
	}
	if strings.HasSuffix(src, ".tar.zst") || strings.HasSuffix(src, ".tar") {
		return "tarball", nil
	}
	if strings.Contains(src, ".part000") {
		return "split", nil
	}
	return "", fmt.Errorf("unknown source %s", src)
}

func assembleSplitTar(src, tarPath string) error {
	pattern := strings.Replace(src, ".part000", ".part*", 1)
	splitFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find split tarball files: %w", err)
	}
	// Ensure the files are in order so they are appended in the correct order
	slices.Sort(splitFiles)

	tarFile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()
	for i, splitFile := range splitFiles {
		if i == 0 {
			b, err := os.ReadFile(splitFile)
			if err != nil {
				return err
			}
			var pkgData types.ZarfSplitPackageData
			err = json.Unmarshal(b, &pkgData)
			if err != nil {
				return err
			}
			expectedCount := len(splitFiles) - 1
			if expectedCount != pkgData.Count {
				return fmt.Errorf("split file count to not match, expected %d but have %d", pkgData.Count, expectedCount)
			}
			continue
		}
		f, err := os.Open(splitFile)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tarFile, f)
		if err != nil {
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func GetPackageFromSourceOrCluster(ctx context.Context, cluster *cluster.Cluster, src string, skipSignatureValidation bool, publicKeyPath string, inspect bool) (v1alpha1.ZarfPackage, error) {
	_, err := identifySource(src)
	if err != nil {
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
		Inspect:                 inspect,
	}
	p, err := LoadPackage(ctx, loadOpt)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	//nolint: errcheck // ignore
	defer p.Cleanup()
	return p.Pkg, nil
}

// GetSBOMFromLocalOrRemote fetches the SBOM from the given source and extracts it to the destination directory.
// This function will handle both local and remote sources, including OCI registries.
// Returns the path to the extracted SBOM files or an error if the operation fails.
func GetSBOMFromLocalOrRemote(ctx context.Context, src string, dst string, skipSignatureValidation bool, publicKeyPath string) (string, error) {
	srcType, err := identifySource(src)
	if err != nil {
		return "", err
	}

	// we need a temporary directory to store the sbom tarball
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpDir)

	// If the source is OCI - we want to prevent pulling the full package
	// Instead we will fetch the SBOM directly from the OCI registry
	if srcType == "oci" {
		pkgName, err := FetchSBOM(ctx, tmpDir, FetchOptions{
			Source:                  src,
			Architecture:            config.GetArch(),
			PublicKeyPath:           publicKeyPath,
			SkipSignatureValidation: skipSignatureValidation,
		})
		if err != nil {
			return "", err
		}
		path := filepath.Join(dst, pkgName)
		err = archiver.Extract(filepath.Join(tmpDir, "sboms.tar"), "", path)
		if err != nil {
			return "", err
		}
		return path, nil
	}
	loadOpt := LoadOptions{
		Source:                  src,
		SkipSignatureValidation: skipSignatureValidation,
		Architecture:            config.GetArch(),
		Filter:                  filters.Empty(),
		PublicKeyPath:           publicKeyPath,
	}
	layout, err := LoadPackage(ctx, loadOpt)
	if err != nil {
		return "", err
	}
	return layout.GetSBOM(dst)
}
