// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

func NewClusterSource(pkgOpts *types.ZarfPackageOptions) (types.PackageSource, error) {
	if !validate.IsLowercaseNumberHyphen(pkgOpts.PackageSource) {
		return nil, fmt.Errorf("invalid package name %q", pkgOpts.PackageSource)
	}
	cluster, err := cluster.NewClusterWithWait(cluster.DefaultTimeout, false)
	if err != nil {
		return nil, err
	}
	return &ClusterSource{pkgOpts, cluster}, nil
}

type ClusterSource struct {
	*types.ZarfPackageOptions
	*cluster.Cluster
}

func (s *ClusterSource) LoadPackage(_ types.PackagePathsMap) error {
	return fmt.Errorf("not implemented")
}

func (s *ClusterSource) Collect(_ string) error {
	return fmt.Errorf("not implemented")
}

func (s *ClusterSource) LoadPackageMetadata(dst types.PackagePathsMap, _ bool) (err error) {
	dpkg, err := s.GetDeployedPackage(s.PackageSource)
	if err != nil {
		return err
	}

	dst.SetDefaultRelative(types.ZarfYAML)

	return utils.WriteYaml(dst[types.ZarfYAML], dpkg.Data, 0755)
}
