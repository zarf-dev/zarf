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
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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

func (suite *RegistryProxyTestSuite) Test_1_DeployRegularPackage() {
	tmpdir := suite.T().TempDir()
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "create", "examples/dos-games", "-o", tmpdir)
	suite.NoError(err, stdOut, stdErr)

	deployPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-dos-games-%s-1.3.0.tar.zst", runtime.GOARCH))
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "deploy", deployPath, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "remove", deployPath, "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *RegistryProxyTestSuite) Test_2_UpdateCredsUpdatesMTLSSecrets() {
	ctx := suite.T().Context()

	originalPKI, err := suite.cluster.GetRegistryClientMTLSCert(ctx)
	suite.NoError(err)
	suite.NotEmpty(originalPKI.Cert)

	stdOut, stdErr, err := e2e.Zarf(suite.T(), "tools", "update-creds", "registry", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	updatedPKI, err := suite.cluster.GetRegistryClientMTLSCert(ctx)
	suite.NoError(err)
	suite.NotEmpty(updatedPKI.Cert)

	// Verify that the original certificate matches what was regenerated
	suite.NotEqual(originalPKI.Cert, updatedPKI.Cert)

	// Get the updated client TLS secret from the dos-games namespace
	updatedNamespaceSecret, err := suite.cluster.Clientset.CoreV1().Secrets("dos-games").Get(ctx, cluster.RegistryClientTLSSecret, metav1.GetOptions{})
	suite.NoError(err)

	suite.Equal(updatedPKI.Cert, updatedNamespaceSecret.Data[cluster.RegistrySecretCertPath])
}

func (suite *RegistryProxyTestSuite) Test_3_OCIOpsPackage() {
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
	suite.Contains(stdErr, "stefanprodan/podinfo")
}

func (suite *RegistryProxyTestSuite) Test_4_SwitchBetweenNodePort() {
	ctx := suite.T().Context()
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "init", "--registry-mode=nodeport", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Verify that the svc zarf-docker-registry has the default nodeport
	svc, err := suite.cluster.Clientset.CoreV1().Services("zarf").Get(ctx, "zarf-docker-registry", metav1.GetOptions{})
	suite.NoError(err)
	suite.Equal(corev1.ServiceTypeNodePort, svc.Spec.Type)
	suite.Equal(int32(state.ZarfInClusterContainerRegistryNodePort), svc.Spec.Ports[0].NodePort)
	// Verify that the daemonset zarf-registry-proxy does not exist
	_, err = suite.cluster.Clientset.AppsV1().DaemonSets("zarf").Get(ctx, "zarf-registry-proxy", metav1.GetOptions{})
	suite.True(kerrors.IsNotFound(err))

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "init", "--features=registry-proxy=true", "--registry-mode=proxy", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Verify that the svc zarf-docker-registry is not a nodeport svc
	svc, err = suite.cluster.Clientset.CoreV1().Services("zarf").Get(ctx, "zarf-docker-registry", metav1.GetOptions{})
	suite.NoError(err)
	suite.NotEqual(corev1.ServiceTypeNodePort, svc.Spec.Type)
	// Verify that the daemonset zarf-registry-proxy exists
	_, err = suite.cluster.Clientset.AppsV1().DaemonSets("zarf").Get(ctx, "zarf-registry-proxy", metav1.GetOptions{})
	suite.NoError(err, "zarf-registry-proxy daemonset should exist in proxy mode")
}

func TestRegistryProxy(t *testing.T) {
	suite.Run(t, new(RegistryProxyTestSuite))
}
