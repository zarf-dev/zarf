// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"context"
	"fmt"
	"os"

	goyaml "github.com/goccy/go-yaml"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/composer"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// LintError represents an error containing lint findings.
//
//nolint:revive // ignore name
type LintError struct {
	BaseDir     string
	PackageName string
	Findings    []PackageFinding
}

func (e *LintError) Error() string {
	return fmt.Sprintf("linting error found %d instance(s)", len(e.Findings))
}

// OnlyWarnings returns true if all findings have severity warning.
func (e *LintError) OnlyWarnings() bool {
	for _, f := range e.Findings {
		if f.Severity == SevErr {
			return false
		}
	}
	return true
}

// Validate lints the given Zarf package
func Validate(ctx context.Context, baseDir, flavor string, setVariables map[string]string) error {
	err := os.Chdir(baseDir)
	if err != nil {
		return fmt.Errorf("unable to access directory %q: %w", baseDir, err)
	}
	b, err := os.ReadFile(layout.ZarfYAML)
	if err != nil {
		return err
	}
	var pkg v1alpha1.ZarfPackage
	err = goyaml.Unmarshal(b, &pkg)
	if err != nil {
		return err
	}

	findings := []PackageFinding{}
	compFindings, err := lintComponents(ctx, pkg, flavor, setVariables)
	if err != nil {
		return err
	}
	findings = append(findings, compFindings...)
	schemaFindings, err := ValidatePackageSchema(setVariables)
	if err != nil {
		return err
	}
	findings = append(findings, schemaFindings...)
	if len(findings) == 0 {
		return nil
	}
	return &LintError{
		BaseDir:     baseDir,
		PackageName: pkg.Metadata.Name,
		Findings:    findings,
	}
}

func lintComponents(ctx context.Context, pkg v1alpha1.ZarfPackage, flavor string, setVariables map[string]string) ([]PackageFinding, error) {
	findings := []PackageFinding{}
	for i, component := range pkg.Components {
		arch := config.GetArch(pkg.Metadata.Architecture)
		if !composer.CompatibleComponent(component, arch, flavor) {
			continue
		}
		chain, err := composer.NewImportChain(ctx, component, i, pkg.Metadata.Name, arch, flavor)
		if err != nil {
			return nil, err
		}
		node := chain.Head()
		for node != nil {
			component := node.ZarfComponent
			compFindings, err := templateZarfObj(&component, setVariables)
			if err != nil {
				return nil, err
			}
			compFindings = append(compFindings, CheckComponentValues(component, node.Index())...)
			for i := range compFindings {
				compFindings[i].PackagePathOverride = node.ImportLocation()
				compFindings[i].PackageNameOverride = node.OriginalPackageName()
			}
			findings = append(findings, compFindings...)
			node = node.Next()
		}
	}
	return findings, nil
}

func templateZarfObj(zarfObj any, setVariables map[string]string) ([]PackageFinding, error) {
	var findings []PackageFinding
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) error {
		yamlTemplates, err := utils.FindYamlTemplates(zarfObj, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				findings = append(findings, PackageFinding{
					Description: fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					Severity:    SevWarn,
				})
			}
			if _, present := setVariables[key]; !present {
				findings = append(findings, PackageFinding{
					Description: fmt.Sprintf("package template %s is not set and won't be evaluated during lint", key),
					Severity:    SevWarn,
				})
			}
		}
		for key, value := range setVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
		return nil
	}

	if err := setVarsAndWarn(v1alpha1.ZarfPackageTemplatePrefix, false); err != nil {
		return nil, err
	}

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := setVarsAndWarn(v1alpha1.ZarfPackageVariablePrefix, true); err != nil {
		return nil, err
	}

	if err := utils.ReloadYamlTemplate(zarfObj, templateMap); err != nil {
		return nil, err
	}
	return findings, nil
}
