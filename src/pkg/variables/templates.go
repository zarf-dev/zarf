// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for interacting with variables
package variables

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// TextTemplate represents a value to be templated into a text file.
type TextTemplate struct {
	Sensitive  bool
	AutoIndent bool
	Type       v1alpha1.VariableType
	Value      string
}

// GetAllTemplates gets all of the current templates stored in the VariableConfig
func (vc *VariableConfig) GetAllTemplates() map[string]*TextTemplate {
	templateMap := vc.applicationTemplates

	for key, variable := range vc.setVariableMap {
		// Variable keys are always uppercase in the format ###ZARF_VAR_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###%s_VAR_%s###", vc.templatePrefix, key))] = &TextTemplate{
			Value:      variable.Value,
			Sensitive:  variable.Sensitive,
			AutoIndent: variable.AutoIndent,
			Type:       variable.Type,
		}
	}

	for _, constant := range vc.constants {
		// Constant keys are always uppercase in the format ###ZARF_CONST_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###%s_CONST_%s###", vc.templatePrefix, constant.Name))] = &TextTemplate{
			Value:      constant.Value,
			AutoIndent: constant.AutoIndent,
		}
	}

	return templateMap
}

// ReplaceTextTemplate loads a file from a given path, replaces text in it and writes it back in place.
func (vc *VariableConfig) ReplaceTextTemplate(path string) error {
	templateRegex := fmt.Sprintf("###%s_[A-Z0-9_]+###", strings.ToUpper(vc.templatePrefix))
	templateMap := vc.GetAllTemplates()

	textFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer textFile.Close()

	// This regex takes a line and parses the text before and after a discovered template: https://regex101.com/r/ilUxAz/1
	regexTemplateLine := regexp.MustCompile(fmt.Sprintf("(?P<preTemplate>.*?)(?P<template>%s)(?P<postTemplate>.*)", templateRegex))

	fileScanner := bufio.NewScanner(textFile)

	// Set the buffer to 1 MiB to handle long lines (i.e. base64 text in a secret)
	// 1 MiB is around the documented maximum size for secrets and configmaps
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	fileScanner.Buffer(buf, maxCapacity)

	// Set the scanner to split on new lines
	fileScanner.Split(bufio.ScanLines)

	text := ""

	for fileScanner.Scan() {
		line := fileScanner.Text()

		for {
			matches := regexTemplateLine.FindStringSubmatch(line)

			// No template left on this line so move on
			if len(matches) == 0 {
				text += fmt.Sprintln(line)
				break
			}

			preTemplate := matches[regexTemplateLine.SubexpIndex("preTemplate")]
			templateKey := matches[regexTemplateLine.SubexpIndex("template")]

			template := templateMap[templateKey]

			// Check if the template is nil (present), use the original templateKey if not (so that it is not replaced).
			value := templateKey
			if template != nil {
				value = template.Value

				// Check if the value is a file type and load the value contents from the file
				if template.Type == v1alpha1.FileVariableType && value != "" {
					if isText, err := helpers.IsTextFile(value); err != nil || !isText {
						nonTextWarning := fmt.Sprintf("Refusing to load a non-text file for templating %s", templateKey)
						vc.logger.Warn(nonTextWarning)
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					contents, err := os.ReadFile(value)
					if err != nil {
						unableToReadWarning := fmt.Sprintf("Unable to read file for templating - skipping: %s", err.Error())
						vc.logger.Warn(unableToReadWarning)
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					value = string(contents)
				}

				// Check if the value is autoIndented and add the correct spacing
				if template.AutoIndent {
					indent := fmt.Sprintf("\n%s", strings.Repeat(" ", len(preTemplate)))
					value = strings.ReplaceAll(value, "\n", indent)
				}
			}

			// Add the processed text and continue processing the line
			text += fmt.Sprintf("%s%s", preTemplate, value)
			line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
		}
	}

	return os.WriteFile(path, []byte(text), helpers.ReadWriteUser)
}
