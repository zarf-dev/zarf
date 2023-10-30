// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CompositionSuite struct {
	suite.Suite
	*require.Assertions
}

func (suite *CompositionSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
}

// func (suite *CompositionSuite) TearDownSuite() {

// }

func (suite *SkeletonSuite) Test_0_ComposabilityExample() {

}

func TestCompositionSuite(t *testing.T) {
	e2e.SetupWithCluster(t)

	suite.Run(t, new(CompositionSuite))
}
