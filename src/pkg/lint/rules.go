// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package lint

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

// Stable semantic rule IDs.
const (
	RuleRepositoryUnpinned    = "repository-unpinned"
	RuleImageReferenceInvalid = "image-reference-invalid"
	RuleImageUnpinned         = "image-unpinned"
	RuleImageRegistryMissing  = "image-registry-missing"
	RuleFileChecksumMissing   = "file-checksum-missing"
	RuleImageArchiveDomain    = "image-archive-domain"
	RuleValuesSchemaMissing   = "values-schema-missing"
	RuleValuesUnused          = "values-unused"
	RuleTemplateSyntax        = "template-syntax"
	RuleTemplateInputMissing  = "template-input-missing"
	RuleTemplateRender        = "template-render"
	RuleValuesMapping         = "values-mapping"
	RuleChartValuesSchema     = "chart-values-schema"
	RuleChartSchemaSkipped    = "chart-schema-skipped"
)

var skippableRules = map[string]struct{}{
	RuleRepositoryUnpinned:    {},
	RuleImageReferenceInvalid: {},
	RuleImageUnpinned:         {},
	RuleImageRegistryMissing:  {},
	RuleFileChecksumMissing:   {},
	RuleImageArchiveDomain:    {},
	RuleValuesSchemaMissing:   {},
	RuleValuesUnused:          {},
	RuleTemplateSyntax:        {},
	RuleTemplateInputMissing:  {},
	RuleTemplateRender:        {},
	RuleValuesMapping:         {},
	RuleChartValuesSchema:     {},
	RuleChartSchemaSkipped:    {},
}

// ValidateSkipRules rejects unknown rule IDs.
func ValidateSkipRules(rules []string) error {
	for _, rule := range rules {
		if _, ok := skippableRules[rule]; !ok {
			return fmt.Errorf("unknown lint rule %q", rule)
		}
	}
	return nil
}

// CheckPrimitives checks recommended practices on a resolved package's repositories,
// images, files, and image archives. componentConfig selects singular component paths.
func CheckPrimitives(pkg api.Package, componentConfig bool) []PackageFinding {
	var findings []PackageFinding
	switch pkg.GetAPIVersion() {
	case v1alpha1.APIVersion:
		for i, component := range convert.PackageToV1alpha1(pkg).Components {
			findings = append(findings, checkComponentPrimitivesV1Alpha1(component, i)...)
		}
	case v1beta1.APIVersion:
		for i, component := range convert.PackageToV1beta1(pkg).Components {
			path := fmt.Sprintf(".components.[%d]", i)
			if componentConfig {
				path = ".component"
			}
			findings = append(findings, checkComponentPrimitivesV1Beta1(component.ComponentSpec, path)...)
		}
	}
	return findings
}
