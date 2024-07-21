// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// verify that ClusterSource implements PackageSource
	_ PackageSource = (*ClusterSource)(nil)
)

// NewClusterSource creates a new cluster source.
func NewClusterSource(pkgOpts *types.ZarfPackageOptions) (PackageSource, error) {
	if !types.IsLowercaseNumberHyphenNoStartHyphen(pkgOpts.PackageSource) {
		return nil, fmt.Errorf("invalid package name %q", pkgOpts.PackageSource)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cluster.DefaultTimeout)
	defer cancel()

	cluster, err := cluster.NewClusterWithWait(ctx)
	if err != nil {
		return nil, err
	}
	return &ClusterSource{pkgOpts, cluster}, nil
}

// ClusterSource is a package source for clusters.
type ClusterSource struct {
	*types.ZarfPackageOptions
	*cluster.Cluster
}

// LoadPackage loads a package from a cluster.
//
// This is not implemented.
func (s *ClusterSource) LoadPackage(_ context.Context, _ *layout.PackagePaths, _ filters.ComponentFilterStrategy, _ bool) (types.ZarfPackage, []string, error) {
	return types.ZarfPackage{}, nil, fmt.Errorf("not implemented")
}

// Collect collects a package from a cluster.
//
// This is not implemented.
func (s *ClusterSource) Collect(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// LoadPackageMetadata loads package metadata from a cluster.
func (s *ClusterSource) LoadPackageMetadata(ctx context.Context, dst *layout.PackagePaths, _ bool, _ bool) (types.ZarfPackage, []string, error) {
	dpkg, err := s.GetDeployedPackage(ctx, s.PackageSource)
	if err != nil {
		return types.ZarfPackage{}, nil, err
	}

	if err := utils.WriteYaml(dst.ZarfYAML, dpkg.Data, helpers.ReadUser); err != nil {
		return types.ZarfPackage{}, nil, err
	}

	return dpkg.Data, nil, nil
}
