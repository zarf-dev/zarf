// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/openvex/go-vex/pkg/vex"
)

// VexType is the type of report that is a VEX document
const VexType = "vex"

// Extend src/types.ZarfComponentReport to add a HasCorrectFields() method
type ZarfReport struct {
	ZarfComponentReport types.ZarfComponentReport
}

func (report *ZarfReport) ValidateReportSource(file_path string) error {
	// overload method to allow for file_path to be passed in or be the report source
	if file_path == "" {
		file_path = report.ZarfComponentReport.Source
	}
	path, err := os.Stat(file_path)
	if err != nil {
		return fmt.Errorf(lang.PkgValidateErrPath, err)
	}
	if !path.IsDir() {
		// check valid vex document
		vexDoc, err := vex.Load(file_path)
		if err != nil {
			return err
		}
		for _, s := range vexDoc.Statements {
			err := vex.Statement.Validate(s)
			if err != nil {
				return err
			}
		}
	} else {
		message.Debugf("VEX path is a directory!")
		file, err := os.Open(file_path)

		if err != nil {
			return fmt.Errorf(lang.PkgValidateErrPath, err)
		}

		defer file.Close()

		files, err := file.Readdirnames(0)
		message.Debugf("Files found are: %s", files)

		if err != nil {
			return fmt.Errorf(lang.PkgValidateErrPath, err)
		}

		for _, f := range files {
			filePath := fmt.Sprintf("%s/%s", report.ZarfComponentReport.Source, f)
			message.Debugf("Attempting to validate %s", filePath)
			if err := report.ValidateReportSource(filePath); err != nil {
				return fmt.Errorf(lang.PkgValidateErrVexInvalid1, err)
			}
		}
	}

	return nil
}
func (report *ZarfReport) ValidateSource() bool {
	// TODO - add more validation for URL Source
	if IsURL(report.ZarfComponentReport.Source) {
		message.Debug("skipping validation due to remote location - validation will occur during create")
		return true
	}

	return false
}

// TODO ValidateVexType
func (report *ZarfReport) ValidateType() error {
	// TODO - add more validation for Type
	var err error

	if report.ZarfComponentReport.Type == strings.ToLower(VEX_TYPE) {
		if err = report.ValidateReportSource(report.ZarfComponentReport.Source); err != nil {
			message.Debugf("Error validating VEX report: %s", err)
			return err
		}
	}

	return err
}

// TODO HasValidVexFields
func (report *ZarfReport) HasCorrectFields() bool {
	if report.ZarfComponentReport.Name == "" {
		return false
	}
	if report.ZarfComponentReport.Source == "" {
		return false
	}
	if report.ZarfComponentReport.Type == "" {
		return false
	}

	return true
}
