// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestIsJSONPathWaitTypeSuite struct {
	suite.Suite
	*require.Assertions
	waitTypes testWaitTypes
}

type testWaitTypes struct {
	jsonPathType  []string
	conditionType []string
}

func (suite *TestIsJSONPathWaitTypeSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())

	suite.waitTypes.jsonPathType = []string{
		"{.status.availableReplicas}=1",
		"{.status.containerStatuses[0].ready}=true",
		"{.spec.containers[0].ports[0].containerPort}=80",
		"{.spec.nodeName}=knode0",
	}
	suite.waitTypes.conditionType = []string{
		"Ready",
		"delete",
		"",
	}
}

func (suite *TestIsJSONPathWaitTypeSuite) Test_0_IsJSONPathWaitType() {
	for _, waitType := range suite.waitTypes.conditionType {
		suite.False(isJSONPathWaitType(waitType), "Expected %s not to be a JSONPath wait type", waitType)
	}
	for _, waitType := range suite.waitTypes.jsonPathType {
		suite.True(isJSONPathWaitType(waitType), "Expected %s to be a JSONPath wait type", waitType)
	}
}

func TestIsJSONPathWaitType(t *testing.T) {
	suite.Run(t, new(TestIsJSONPathWaitTypeSuite))
}
