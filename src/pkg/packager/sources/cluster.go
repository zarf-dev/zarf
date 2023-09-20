// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// veryify that ClusterSource implements PackageSource
	_ PackageSource = (*ClusterSource)(nil)
)

// NewClusterSource creates a new cluster source.
func NewClusterSource(pkgOpts *types.ZarfPackageOptions) (PackageSource, error) {
	if !validate.IsLowercaseNumberHyphen(pkgOpts.PackageSource) {
		return nil, fmt.Errorf("invalid package name %q", pkgOpts.PackageSource)
	}
	cluster, err := cluster.NewClusterWithWait(cluster.DefaultTimeout, false)
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
func (s *ClusterSource) LoadPackage(_ *layout.PackagePaths) error {
	return fmt.Errorf("not implemented")
}

// Collect collects a package from a cluster.
//
// This is not implemented.
func (s *ClusterSource) Collect(_ string) error {
	return fmt.Errorf("not implemented")
}

// LoadPackageMetadata loads package metadata from a cluster.
func (s *ClusterSource) LoadPackageMetadata(dst *layout.PackagePaths, _ bool, _ bool) (err error) {
	dpkg, err := s.GetDeployedPackage(s.PackageSource)
	if err != nil {
		return err
	}

	dst.ZarfYAML = filepath.Join(dst.Base, layout.ZarfYAML)

	return utils.WriteYaml(dst.ZarfYAML, dpkg.Data, 0755)
}
