// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package network

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestNetworkSuite struct {
	suite.Suite
	*require.Assertions
}

func (suite *TestNetworkSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
}
func (suite *TestNetworkSuite) Test_0_DoHostnamesMatch() {

	b, err := DoHostnamesMatch("https://zarf.dev", "https://zarf.dev")
	suite.NoError(err)
	suite.True(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "https://docs.zarf.dev")
	suite.NoError(err)
	suite.False(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "https://zarf.dev/docs")
	suite.NoError(err)
	suite.True(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "https://user:pass@zarf.dev")
	suite.NoError(err)
	suite.True(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "")
	suite.NoError(err)
	suite.False(b)
	b, err = DoHostnamesMatch("", "https://zarf.dev")
	suite.NoError(err)
	suite.False(b)
}

func TestNetwork(t *testing.T) {
	suite.Run(t, new(TestNetworkSuite))
}
