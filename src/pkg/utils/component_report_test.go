// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ZarfTestReport tests src/pkg/utils/component_report.go
type ZarfTestReport struct {
	success bool       // whether the test should succeed or fail
	report  ZarfReport // component report to test
}

// TestReportSuite tests src/pkg/utils/component_report.go
type TestReportSuite struct {
	suite.Suite
	*require.Assertions
	// test paths
	validPaths   []ZarfTestReport
	invalidPaths []ZarfTestReport
	// test fields
	validFields   []ZarfTestReport
	invalidFields []ZarfTestReport
	// test types
	validTypes   []ZarfTestReport
	invalidTypes []ZarfTestReport
	// test sources
	validSources   []ZarfTestReport
	invalidSources []ZarfTestReport
}

func (suite *TestReportSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())

	suite.validPaths = []ZarfTestReport{
		{
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "valid file",
					Source: "../../test/packages/53-component-reports/vex/test-component.vex-1.json",
					Type:   "vex",
				},
			},
		},
		{
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "valid directory",
					Source: "../../test/packages/53-component-reports/vex/",
					Type:   "vex",
				},
			},
		},
	}

	suite.invalidPaths = []ZarfTestReport{
		{
			success: false,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "invalid file",
					Source: "/somewhere/over/the/rainbow/file.vex",
					Type:   "vex",
				},
			},
		},
		{
			success: false,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "invalid directory",
					Source: "/somewhere/over/the/rainbow/",
					Type:   "vex",
				},
			},
		},
	}

	suite.validTypes = []ZarfTestReport{
		{
			// we expect ValidateType to throw an error b/c the vexReport is invalid
			// but it SHOULD validate the type
			success: false,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name: "should-not-be-ignoreed",
					Type: "vex",
				},
			},
		},
	}

	suite.invalidTypes = []ZarfTestReport{
		{
			// we expect no error b/c the vexReport is invalid and therefore ignored
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name: "should-be-ignoreed",
					Type: "should-ignore",
				},
			},
		},
		{
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name: "should-be-ignored",
					Type: "yaml",
				},
			},
		},
	}

	suite.validFields = []ZarfTestReport{
		{
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "valid fields",
					Source: "../../test/packages/53-component-reports/vex/test-component.vex-1.json",
					Type:   "vex",
				},
			},
		},
		{
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "valid fields 2",
					Source: "just make sure fields are there",
					Type:   "vex",
				},
			},
		},
	}

	suite.invalidFields = []ZarfTestReport{
		{
			success: false,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name: "missing source and type",
				},
			},
		},
		{
			success: false,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "invalid fields",
					Source: "Missing type",
				},
			},
		},
	}

	suite.validSources = []ZarfTestReport{
		{
			// this is just a wrapper function with a message statement around isURL
			// expect true if isURL
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "valid source",
					Source: "https://zarf.dev",
					Type:   "vex",
				},
			},
		},
		{
			success: true,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "valid source 2",
					Source: "https://defenseunicorns.com",
					Type:   "vex",
				},
			},
		},
	}

	suite.invalidSources = []ZarfTestReport{
		{
			success: false,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "invalid source",
					Source: "not-url",
					Type:   "vex",
				},
			},
		},
		{
			success: false,
			report: ZarfReport{
				types.ZarfComponentReport{
					Name:   "invalid source 2",
					Source: "@defenseunicorns",
					Type:   "vex",
				},
			},
		},
	}

}

func (suite *TestReportSuite) Test_0_ValidateReportPath() {

	for _, test := range suite.validPaths {
		if test.success {
			suite.NoError(test.report.ValidateVexReport(""))
		} else {
			suite.NoError(test.report.ValidateVexReport(""))
		}
	}

	for _, test := range suite.invalidPaths {
		if test.success {
			suite.Error(test.report.ValidateVexReport(""))
		} else {
			suite.Error(test.report.ValidateVexReport(""))
		}
	}
}

func (suite *TestReportSuite) Test_1_ValidateType() {
	// on valid types, we want errors b/c the vex report is invalid
	for _, test := range suite.validTypes {
		if test.success {
			suite.Error(test.report.ValidateType())
		} else {
			suite.Error(test.report.ValidateType())
		}
	}
	// invalid types, no errors b/c the vex report is ignored
	for _, test := range suite.invalidTypes {
		if test.success {
			suite.NoError(test.report.ValidateType())
		} else {
			suite.NoError(test.report.ValidateType())
		}
	}
}

func (suite *TestReportSuite) Test_2_HasCorrectFields() {
	for _, test := range suite.validFields {
		if test.success {
			suite.True(test.report.HasCorrectFields())
		} else {
			suite.True(test.report.HasCorrectFields())
		}
	}

	for _, test := range suite.invalidFields {
		if test.success {
			suite.False(test.report.HasCorrectFields())
		} else {
			suite.False(test.report.HasCorrectFields())
		}
	}
}

func (suite *TestReportSuite) Test_3_ValidateSource() {
	// this is just a wrapper function with a message statement around isURL
	// expect true if isURL
	for _, test := range suite.validSources {
		if test.success {
			suite.True(test.report.ValidateSource())
		} else {
			suite.True(test.report.ValidateSource())
		}
	}

	for _, test := range suite.invalidSources {
		if test.success {
			suite.False(test.report.ValidateSource())
		} else {
			suite.False(test.report.ValidateSource())
		}
	}
}

func TestReport(t *testing.T) {
	message.SetLogLevel(message.DebugLevel)
	suite.Run(t, new(TestReportSuite))
}
