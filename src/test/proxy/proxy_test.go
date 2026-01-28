// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package proxy provides tests for Zarf registry proxy mode.
package proxy

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RegistryProxyTestSuite struct {
	suite.Suite
	*require.Assertions
	cluster *cluster.Cluster
}

func (suite *RegistryProxyTestSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	var err error
	suite.cluster, err = cluster.New(suite.T().Context())
	suite.NoError(err)
}

func (suite *RegistryProxyTestSuite) Test_0_RegistryProxyInit() {
	ctx := suite.T().Context()

	stdOut, stdErr, err := e2e.Zarf(suite.T(), "init", "--features=registry-proxy=true", "--registry-mode=proxy", "--components=git-server", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Verify the registry proxy TLS secrets were created
	_, err = suite.cluster.Clientset.CoreV1().Secrets("zarf").Get(ctx, cluster.RegistryServerTLSSecret, metav1.GetOptions{})
	suite.NoError(err, "zarf-registry-server-tls secret should exist")

	_, err = suite.cluster.Clientset.CoreV1().Secrets("zarf").Get(ctx, cluster.RegistryClientTLSSecret, metav1.GetOptions{})
	suite.NoError(err, "zarf-registry-client-tls secret should exist")
}

func (suite *RegistryProxyTestSuite) Test_1_Flux() {
	tmpdir := suite.T().TempDir()
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "create", "examples/podinfo-flux", "-o", tmpdir)
	suite.NoError(err, stdOut, stdErr)

	deployPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-podinfo-flux-%s.tar.zst", runtime.GOARCH))
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "deploy", deployPath, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "remove", deployPath, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "tools", "registry", "prune", "--confirm")
	suite.NoError(err, stdOut, stdErr)
	// verify that an image name is in the prune output
	suite.Contains(stdOut, "stefanprodan/podinfo")
}

func (suite *RegistryProxyTestSuite) Test_2_UpdateCredsUpdatesMTLSSecrets() {
	ctx := suite.T().Context()

	// Get the original client TLS secret from the zarf namespace
	originalPKI, err := suite.cluster.GetRegistryClientMTLSCert(ctx)
	suite.NoError(err)
	suite.NotEmpty(originalPKI.Cert)

	stdOut, stdErr, err := e2e.Zarf(suite.T(), "tools", "update-creds", "registry", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Get the updated client TLS secret from the zarf namespace
	updatedPKI, err := suite.cluster.GetRegistryClientMTLSCert(ctx)
	suite.NoError(err)
	suite.NotEmpty(updatedPKI.Cert)

	// Verify that the certificate was regenerated
	suite.NotEqual(originalPKI.Cert, updatedPKI.Cert)

	// Get the updated client TLS secret from the podinfo-oci namespace
	updatedNamespaceSecret, err := suite.cluster.Clientset.CoreV1().Secrets("podinfo-oci").Get(ctx, cluster.RegistryClientTLSSecret, metav1.GetOptions{})
	suite.NoError(err)

	// Verify the secret in podinfo-oci namespace matches the zarf namespace
	suite.Equal(updatedPKI.Cert, updatedNamespaceSecret.Data[cluster.RegistrySecretCertPath])
}

func TestRegistryProxy(t *testing.T) {
	suite.Run(t, new(RegistryProxyTestSuite))
}
