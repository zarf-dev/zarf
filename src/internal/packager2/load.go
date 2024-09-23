// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"archive/tar"
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
	"github.com/mholt/archiver/v3"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// LoadOptions are the options for LoadPackage.
type LoadOptions struct {
	Source                  string
	Shasum                  string
	PublicKeyPath           string
	SkipSignatureValidation bool
	Filter                  filters.ComponentFilterStrategy
}

// LoadPackage optionally fetches and loads the package from the given source.
func LoadPackage(ctx context.Context, opt LoadOptions) (*layout.PackagePaths, error) {
	srcType, err := identifySource(opt.Source)
	if err != nil {
		return nil, err
	}

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpDir)
	tarPath := filepath.Join(tmpDir, "data.tar.zst")

	isPartial := false
	switch srcType {
	case "oci":
		isPartial, err = pullOCI(ctx, opt.Source, tarPath, opt.Shasum, opt.Filter)
		if err != nil {
			return nil, err
		}
	case "http", "https":
		err = pullHTTP(ctx, opt.Source, tarPath, opt.Shasum)
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

	// Extract the package
	packageDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	pathsExtracted := []string{}
	err = archiver.Walk(tarPath, func(f archiver.File) error {
		if f.IsDir() {
			return nil
		}
		header, ok := f.Header.(*tar.Header)
		if !ok {
			return fmt.Errorf("expected header to be *tar.Header but was %T", f.Header)
		}
		// If path has nested directories we want to create them.
		dir := filepath.Dir(header.Name)
		if dir != "." {
			err := os.MkdirAll(filepath.Join(packageDir, dir), helpers.ReadExecuteAllWriteUser)
			if err != nil {
				return err
			}
		}
		dst, err := os.Create(filepath.Join(packageDir, header.Name))
		if err != nil {
			return err
		}
		defer dst.Close()
		_, err = io.Copy(dst, f)
		if err != nil {
			return err
		}
		pathsExtracted = append(pathsExtracted, header.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Load the package paths
	pkgPaths := layout.New(packageDir)
	pkgPaths.SetFromPaths(pathsExtracted)
	pkg, _, err := pkgPaths.ReadZarfYAML()
	if err != nil {
		return nil, err
	}
	// TODO: Filter is not persistently applied.
	pkg.Components, err = opt.Filter.Apply(pkg)
	if err != nil {
		return nil, err
	}
	if err := pkgPaths.MigrateLegacy(); err != nil {
		return nil, err
	}
	if !pkgPaths.IsLegacyLayout() {
		if err := sources.ValidatePackageIntegrity(pkgPaths, pkg.Metadata.AggregateChecksum, isPartial); err != nil {
			return nil, err
		}
		if opt.SkipSignatureValidation {
			if err := sources.ValidatePackageSignature(ctx, pkgPaths, opt.PublicKeyPath); err != nil {
				return nil, err
			}
		}
	}
	for _, component := range pkg.Components {
		if err := pkgPaths.Components.Unarchive(component); err != nil {
			if errors.Is(err, layout.ErrNotLoaded) {
				_, err := pkgPaths.Components.Create(component)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}
	if pkgPaths.SBOMs.Path != "" {
		if err := pkgPaths.SBOMs.Unarchive(); err != nil {
			return nil, err
		}
	}
	return pkgPaths, nil
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

func packageFromSourceOrCluster(ctx context.Context, cluster *cluster.Cluster, src string, skipSignatureValidation bool) (v1alpha1.ZarfPackage, error) {
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
		Filter:                  filters.Empty(),
	}
	pkgPaths, err := LoadPackage(ctx, loadOpt)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	defer os.RemoveAll(pkgPaths.Base)
	pkg, _, err := pkgPaths.ReadZarfYAML()
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return pkg, nil
}
