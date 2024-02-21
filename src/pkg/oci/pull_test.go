// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with artifacts stored in OCI registries.
package oci

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"           // used for docker test registry
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

func TestPull(t *testing.T) {
	t.Run("generic pull", func(t *testing.T) {

		// This is the general flow that helm is doing
		// I want to figure out what tool they are using to actually start their server
		ctx := context.TODO()

		// Registry config
		config := &configuration.Configuration{}
		port, err := freeport.GetFreePort()
		if err != nil {
			t.Fatalf("error finding free port for test registry")
		}

		config.HTTP.Addr = fmt.Sprintf(":%d", port)
		config.HTTP.DrainTimeout = 10 * time.Second
		config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

		registryURL := fmt.Sprintf("oci://localhost:%d/package:1.0.1", port)

		ref, err := registry.NewRegistry(ctx, config)
		if err != nil {
			t.Fatal(err)
		}

		go ref.ListenAndServe()

		platform := PlatformForArch("arm64")
		remote, err := NewOrasRemote(registryURL, platform, WithPlainHTTP(true))
		if err != nil {
			t.Fatal(err)
		}

		annotations := map[string]string{
			ocispec.AnnotationTitle:       "name",
			ocispec.AnnotationDescription: "desc",
		}

		_, err = remote.PushManifestConfigFromMetadata(ctx, annotations, ocispec.MediaTypeImageConfig)
		if err != nil {
			t.Fatal(err)
		}

		require.True(t, true)
	})
}
