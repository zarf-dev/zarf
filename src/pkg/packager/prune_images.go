// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains operations for managing packages.
package packager

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// FilterUnreferencedImages returns the candidate images that are not referenced by any
// active component in activeImages or in any other deployed package.
func FilterUnreferencedImages(candidateImages []string, allPackages []state.DeployedPackage, currentPkgName string, activeImages map[string]bool) []string {
	inUse := make(map[string]bool)
	for img := range activeImages {
		inUse[img] = true
	}

	for _, depPkg := range allPackages {
		if depPkg.Name == currentPkgName {
			continue
		}
		def, err := depPkg.Definition()
		if err != nil {
			continue
		}
		deployedComps := make(map[string]bool, len(depPkg.DeployedComponents))
		for _, c := range depPkg.DeployedComponents {
			deployedComps[c.Name] = true
		}
		for _, comp := range def.Components {
			if deployedComps[comp.Name] {
				for _, img := range comp.GetImages() {
					inUse[img] = true
				}
			}
		}
	}

	var unreferenced []string
	seen := make(map[string]bool)
	for _, img := range candidateImages {
		if !inUse[img] && !seen[img] {
			seen[img] = true
			unreferenced = append(unreferenced, img)
		}
	}
	return unreferenced
}

func isImageNotFoundError(err error) bool {
	var transportErr *transport.Error
	if errors.As(err, &transportErr) {
		if transportErr.StatusCode == http.StatusNotFound {
			return true
		}
		for _, diag := range transportErr.Errors {
			if diag.Code == transport.ManifestUnknownErrorCode {
				return true
			}
		}
	}
	return false
}

func deleteImageFromRegistryDefault(ctx context.Context, c *cluster.Cluster, s *state.State, ref string) error {
	options := []crane.Option{images.WithPushAuth(s.RegistryInfo)}
	if s.RegistryInfo.ShouldUseMTLS() && c != nil {
		certs, err := c.GetRegistryClientMTLSCert(ctx)
		if err != nil {
			return err
		}
		t, err := pki.TransportWithKey(certs)
		if err != nil {
			return err
		}
		options = append(options, crane.WithTransport(t))
	}

	err := crane.Delete(ref, options...)
	if err != nil && isImageNotFoundError(err) {
		return nil
	}
	return err
}

// pruneImagesFromRegistry deletes both the plain and CRC32-suffixed tags for each image from the internal registry.
func pruneImagesFromRegistry(ctx context.Context, c *cluster.Cluster, s *state.State, unreferenced []string, deleteFn func(ctx context.Context, c *cluster.Cluster, s *state.State, ref string) error) error {
	if len(unreferenced) == 0 || s == nil || s.RegistryInfo.Address == "" {
		return nil
	}
	if deleteFn == nil {
		deleteFn = deleteImageFromRegistryDefault
	}
	l := logger.From(ctx)

	endpoint := s.RegistryInfo.Address
	var tunnel *cluster.Tunnel
	if c != nil {
		var err error
		endpoint, tunnel, err = c.ConnectToZarfRegistryEndpoint(ctx, s.RegistryInfo)
		if err != nil {
			return fmt.Errorf("failed to connect to registry endpoint: %w", err)
		}
		if tunnel != nil {
			defer tunnel.Close()
		}
	}

	executePrune := func() error {
		for _, img := range unreferenced {
			plainRef, err := transform.ImageTransformHostWithoutChecksum(endpoint, img)
			if err != nil {
				return fmt.Errorf("unable to transform image reference %s: %w", img, err)
			}
			crcRef, err := transform.ImageTransformHost(endpoint, img)
			if err != nil {
				return fmt.Errorf("unable to transform CRC image reference %s: %w", img, err)
			}

			l.Info("pruning orphaned image", "image", img)
			if err := deleteFn(ctx, c, s, plainRef); err != nil {
				return fmt.Errorf("unable to delete plain image tag %s: %w", plainRef, err)
			}
			if err := deleteFn(ctx, c, s, crcRef); err != nil {
				return fmt.Errorf("unable to delete CRC image tag %s: %w", crcRef, err)
			}
		}
		return nil
	}

	if tunnel != nil {
		return tunnel.Wrap(executePrune)
	}
	return executePrune()
}
