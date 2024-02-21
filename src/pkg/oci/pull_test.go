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
	"github.com/stretchr/testify/suite"
	orasRegistry "oras.land/oras-go/v2/registry"
)

type OCISuite struct {
	suite.Suite
	*require.Assertions
	Reference   orasRegistry.Reference
	registryURL string
}

func (suite *OCISuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	suite.StartRegistry()
}

func (suite *OCISuite) StartRegistry() {
	// Registry config
	ctx := context.TODO()
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	suite.NoError(err)

	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	ref, err := registry.NewRegistry(ctx, config)
	suite.NoError(err)

	go ref.ListenAndServe()

	suite.registryURL = fmt.Sprintf("oci://localhost:%d/package:1.0.1", port)
}

func (suite *OCISuite) Test_0_Publish() {
	suite.T().Log("")
	// This is the general flow that helm is doing
	// I want to figure out what tool they are using to actually start their server

	ctx := context.TODO()
	platform := PlatformForArch("fake-package-so-does-not-matter")
	remote, err := NewOrasRemote(suite.registryURL, platform, WithPlainHTTP(true))
	suite.NoError(err)

	annotations := map[string]string{
		ocispec.AnnotationTitle:       "name",
		ocispec.AnnotationDescription: "desc",
	}

	_, err = remote.PushManifestConfigFromMetadata(ctx, annotations, ocispec.MediaTypeImageConfig)
	suite.NoError(err)

}

func TestPublishDeploySuite(t *testing.T) {
	suite.Run(t, new(OCISuite))
}
