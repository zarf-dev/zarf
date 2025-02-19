package packager2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/oci"
	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry"
)

func TestPublishError(t *testing.T) {
	ctx := context.Background()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

	// TODO add freeport
	registryURL := testutil.SetupInMemoryRegistry(ctx, t, 5000)
	ref := registry.Reference{
		Registry:   registryURL,
		Repository: "my-namespace",
	}

	tt := []struct {
		name      string
		opts      PublishOpts
		expectErr error
	}{
		{
			name:      "Test empty publish opts",
			opts:      PublishOpts{},
			expectErr: errors.New("invalid registry"),
		},
		{
			name: "Test empty path",
			opts: PublishOpts{
				Path:     "",
				Registry: ref,
			},
			expectErr: errors.New("path must be specified"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// TODO Make parallel
			// t.Parallel()
			err := Publish(context.Background(), tc.opts)
			require.ErrorContains(t, err, tc.expectErr.Error())
		})
	}
}

func TestPublishSkeleton(t *testing.T) {
	ctx := context.Background()

	// TODO add freeport
	registryURL := testutil.SetupInMemoryRegistry(ctx, t, 5000)
	ref := registry.Reference{
		Registry:   registryURL,
		Repository: "my-namespace",
	}

	tt := []struct {
		name string
		opts PublishOpts
	}{
		{
			name: "Publish skeleton package",
			opts: PublishOpts{
				Path:          "testdata/skeleton",
				Registry:      ref,
				WithPlainHTTP: true,
			},
		},
		// {
		// 	name: "Publish package",
		// 	opts: PublishOpts{
		// 		Path:          "testdata/zarf-package-test-amd64-0.0.1.tar.zst",
		// 		Registry:      ref,
		// 		WithPlainHTTP: true,
		// 	},
		// },
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// TODO Make parallel
			// t.Parallel()

			// Publish test package
			err := Publish(context.Background(), tc.opts)
			require.NoError(t, err)

			// Read and unmarshall expected
			data, err := os.ReadFile(filepath.Join(tc.opts.Path, layout.ZarfYAML))
			require.NoError(t, err)
			var expectedPkg v1alpha1.ZarfPackage
			err = goyaml.Unmarshal(data, &expectedPkg)
			require.NoError(t, err)

			// Format url and instantiate remote
			ref, err := zoci.ReferenceFromMetadata(tc.opts.Registry.String(), &expectedPkg.Metadata, &expectedPkg.Build)
			require.NoError(t, err)
			rmt, err := zoci.NewRemote(ctx, ref, zoci.PlatformForSkeleton(), oci.WithPlainHTTP(true))
			require.NoError(t, err)

			// Fetch from remote and compare
			pkg, err := rmt.FetchZarfYAML(ctx)
			require.NoError(t, err)

			// HACK(mkcp): Match necessary fields
			pkg.Build = v1alpha1.ZarfBuildData{}
			pkg.Metadata.AggregateChecksum = ""
			expectedPkg.Metadata.Architecture = "skeleton"

			// NOTE(mkcp): In future schema version move ZarfPackage.Metadata.AggregateChecksum
			// to ZarfPackage.Build.AggregateChecksum. See ADR #26
			require.Equal(t, pkg, expectedPkg)
		})
	}
}

func TestPublishPackage(t *testing.T) {
	ctx := context.Background()

	// TODO add freeport
	registryURL := testutil.SetupInMemoryRegistry(ctx, t, 5000)
	ref := registry.Reference{
		Registry:   registryURL,
		Repository: "my-namespace",
	}

	tt := []struct {
		name string
		opts PublishOpts
	}{
		{
			name: "Publish package",
			opts: PublishOpts{
				Path:          "testdata/zarf-package-test-amd64-0.0.1.tar.zst",
				Registry:      ref,
				WithPlainHTTP: true,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// TODO Make parallel
			// t.Parallel()
			ctx := context.Background()

			// Publish test package
			err := Publish(ctx, tc.opts)
			require.NoError(t, err)

			// We want to pull the package and sure the content is the same as the local package

			pkgLayout, err := layout2.LoadFromTar(ctx, tc.opts.Path, layout2.PackageLayoutOptions{})
			require.NoError(t, err)
			// Format url and instantiate remote
			ref, err := zoci.ReferenceFromMetadata(tc.opts.Registry.String(), &pkgLayout.Pkg.Metadata, &pkgLayout.Pkg.Build)
			require.NoError(t, err)
			tmpdir := t.TempDir()
			_, err = pullOCI(context.Background(), ref, tmpdir, pkgLayout.Pkg.Metadata.AggregateChecksum, filters.Empty())
			require.NoError(t, err)
		})
	}
}
