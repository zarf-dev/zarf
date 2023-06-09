// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
)

func TestValidateReportSource(t *testing.T) {
	// Case 1: Valid VEX file path
	report := &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "../../../examples/component-reports/vex/test-component.vex-1.json",
			Type:   "vex",
		},
	}
	err := report.ValidateVexReport("")
	if err != nil {
		t.Errorf("Expected ValidateReportSource to return nil, but got error: %v", err)
	}

	// Case 2: Valid VEX directory path with valid files
	report = &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "../../../examples/component-reports/vex/",
			Type:   "vex",
		},
	}
	err = report.ValidateVexReport("")
	if err != nil {
		t.Errorf("Expected ValidateReportSource to return nil, but got error: %v", err)
	}

	// Case 3: invalid VEX directory path
	report = &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "/somewhere/over/the/rainbow/",
			Type:   "vex",
		},
	}
	err = report.ValidateVexReport("")
	if err == nil {
		t.Error("Expected ValidateReportSource to return an error, but got nil")
	}

	// // Case 4: Invalid file path
	report = &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "invalid/file.vex",
			Type:   "vex",
		},
	}
	err = report.ValidateVexReport("")
	if err == nil {
		t.Error("Expected ValidateReportSource to return an error, but got nil")
	}
}

func TestValidateType(t *testing.T) {
	report1 := &ZarfReport{
		types.ZarfComponentReport{
			Type: "should-ignore",
		},
	}

	if err := report1.ValidateType(); err != nil {
		t.Errorf("Expected TestValidateType to return nil, but it returned error %s", err)
	}

	report2 := &ZarfReport{
		types.ZarfComponentReport{
			Type: "vex",
		},
	}

	if err := report2.ValidateType(); err == nil {
		t.Errorf("Expected TestValidateType to return nil, but it returned error %s", err)
	}
}

func TestHasCorrectFields(t *testing.T) {
	report := &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "Test Source",
			Type:   "Test Type",
		},
	}

	if !report.HasCorrectFields() {
		t.Error("Expected HasCorrectFields to return true, but it returned false")
	}
}

func TestHasIncorrectFields(t *testing.T) {
	report := &ZarfReport{
		types.ZarfComponentReport{
			Name:   "",
			Source: "Test Source",
			Type:   "Test Type",
		},
	}

	if report.HasCorrectFields() {
		t.Error("Expected HasCorrectFields to return false, but it returned true")
	}

	report = &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "",
			Type:   "Test Type",
		},
	}

	if report.HasCorrectFields() {
		t.Error("Expected HasCorrectFields to return false, but it returned true")
	}

	report = &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "Test Source",
			Type:   "",
		},
	}

	if report.HasCorrectFields() {
		t.Error("Expected HasCorrectFields to return false, but it returned true")
	}
}

func TestValidateSource(t *testing.T) {

	report := &ZarfReport{
		types.ZarfComponentReport{
			Name:   "",
			Source: "not-url",
			Type:   "Test Type",
		},
	}

	if notURL := report.ValidateSource(); notURL {
		t.Error("Expected TestValidateSource to return false, but it returned true")
	}

	report = &ZarfReport{
		types.ZarfComponentReport{
			Name:   "Test Name",
			Source: "https://www.defenseunicorns.com",
			Type:   "Test Type",
		},
	}

	if url := report.ValidateSource(); !url {
		t.Error("Expected TestValidateSource to return true, but it returned false")
	}

}
