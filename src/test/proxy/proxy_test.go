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
)

type RegistryProxyTestSuite struct {
	suite.Suite
	*require.Assertions
}

func (suite *RegistryProxyTestSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
}

func (suite *RegistryProxyTestSuite) Test_0_RegistryProxyInit() {
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "init", "--features=registry-proxy=true", "--registry-mode=proxy", "--components=git-server", "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *RegistryProxyTestSuite) Test_1_Flux() {
	tmpdir := suite.T().TempDir()
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "create", "examples/podinfo-flux", "-o", tmpdir)
	suite.NoError(err, stdOut, stdErr)

	deployPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-podinfo-flux-%s.tar.zst", runtime.GOARCH))
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "deploy", deployPath, "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func TestRegistryProxy(t *testing.T) {
	suite.Run(t, new(RegistryProxyTestSuite))
}
