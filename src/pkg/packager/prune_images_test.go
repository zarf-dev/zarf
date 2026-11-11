// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestFilterUnreferencedImages(t *testing.T) {
	t.Parallel()

	pkgA := state.DeployedPackage{
		Name: "pkg-a",
		DeployedComponents: []state.DeployedComponent{
			{Name: "comp-shared"},
			{Name: "comp-inactive"},
		},
		Data: v1alpha1.ZarfPackage{
			Components: []v1alpha1.ZarfComponent{
				{
					Name:   "comp-shared",
					Images: []string{"shared-image:v1"},
				},
				{
					Name:   "comp-inactive",
					Images: []string{"unreferenced-in-inactive:v1"},
				},
			},
		},
	}
	// For pkg-a, only comp-shared is considered deployed, comp-inactive is present in DeployedComponents
	// but let's test a case where comp-other is NOT deployed in DeployedComponents
	pkgB := state.DeployedPackage{
		Name: "pkg-b",
		DeployedComponents: []state.DeployedComponent{
			{Name: "comp-active"},
		},
		Data: v1alpha1.ZarfPackage{
			Components: []v1alpha1.ZarfComponent{
				{
					Name:   "comp-active",
					Images: []string{"shared-b:v1"},
				},
				{
					Name:   "comp-not-deployed",
					Images: []string{"not-deployed-image:v1"},
				},
			},
		},
	}

	allPkgs := []state.DeployedPackage{pkgA, pkgB}

	t.Run("single dropped image not used elsewhere is unreferenced", func(t *testing.T) {
		candidates := []string{"unique-to-cur:v1"}
		unref := FilterUnreferencedImages(candidates, allPkgs, "current-pkg", nil)
		require.Equal(t, []string{"unique-to-cur:v1"}, unref)
	})

	t.Run("image used by other package is kept", func(t *testing.T) {
		candidates := []string{"shared-image:v1", "unique-to-cur:v1"}
		unref := FilterUnreferencedImages(candidates, allPkgs, "current-pkg", nil)
		require.Equal(t, []string{"unique-to-cur:v1"}, unref)
	})

	t.Run("image only in undeployed component of other package is unreferenced", func(t *testing.T) {
		candidates := []string{"not-deployed-image:v1"}
		unref := FilterUnreferencedImages(candidates, allPkgs, "current-pkg", nil)
		require.Equal(t, []string{"not-deployed-image:v1"}, unref)
	})

	t.Run("image still active in current package is kept", func(t *testing.T) {
		candidates := []string{"active-in-cur:v1", "dropped:v1"}
		active := map[string]bool{"active-in-cur:v1": true}
		unref := FilterUnreferencedImages(candidates, allPkgs, "current-pkg", active)
		require.Equal(t, []string{"dropped:v1"}, unref)
	})

	t.Run("duplicate candidates are deduplicated", func(t *testing.T) {
		candidates := []string{"dropped:v1", "dropped:v1"}
		unref := FilterUnreferencedImages(candidates, allPkgs, "current-pkg", nil)
		require.Equal(t, []string{"dropped:v1"}, unref)
	})
}

func TestPruneImagesFromRegistry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty unreferenced returns nil without actions", func(t *testing.T) {
		called := false
		err := pruneImagesFromRegistry(ctx, nil, &state.State{RegistryInfo: state.RegistryInfo{Address: "127.0.0.1:5000"}}, nil, func(ctx context.Context, c *cluster.Cluster, s *state.State, ref string) error {
			called = true
			return nil
		})
		require.NoError(t, err)
		require.False(t, called)
	})

	t.Run("empty registry address returns nil without actions", func(t *testing.T) {
		called := false
		err := pruneImagesFromRegistry(ctx, nil, &state.State{RegistryInfo: state.RegistryInfo{Address: ""}}, []string{"img:v1"}, func(ctx context.Context, c *cluster.Cluster, s *state.State, ref string) error {
			called = true
			return nil
		})
		require.NoError(t, err)
		require.False(t, called)
	})

	t.Run("deletes both plain and crc tags", func(t *testing.T) {
		var deleted []string
		mockDelete := func(ctx context.Context, c *cluster.Cluster, s *state.State, ref string) error {
			deleted = append(deleted, ref)
			return nil
		}

		s := &state.State{RegistryInfo: state.RegistryInfo{Address: "127.0.0.1:5000"}}
		err := pruneImagesFromRegistry(ctx, nil, s, []string{"repo/image:v1.0.0"}, mockDelete)
		require.NoError(t, err)
		require.Len(t, deleted, 2)
		require.Contains(t, deleted[0], "127.0.0.1:5000/repo/image:v1.0.0")
		require.Contains(t, deleted[1], "127.0.0.1:5000/repo/image:")
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		mockDelete := func(ctx context.Context, c *cluster.Cluster, s *state.State, ref string) error {
			return errors.New("registry connection error")
		}

		s := &state.State{RegistryInfo: state.RegistryInfo{Address: "127.0.0.1:5000"}}
		err := pruneImagesFromRegistry(ctx, nil, s, []string{"repo/image:v1.0.0"}, mockDelete)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to delete plain image tag")
	})
}

func TestIsImageNotFoundError(t *testing.T) {
	t.Parallel()

	require.False(t, isImageNotFoundError(errors.New("generic error")))
	require.True(t, isImageNotFoundError(&transport.Error{StatusCode: http.StatusNotFound}))
	require.True(t, isImageNotFoundError(&transport.Error{
		Errors: []transport.Diagnostic{
			{Code: transport.ManifestUnknownErrorCode},
		},
	}))
}
