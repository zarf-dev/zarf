// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/composer"
)

// Validate lints the given Zarf package
func Validate(ctx context.Context, pkg v1alpha1.ZarfPackage, baseDir, flavor string, setVariables map[string]string) error {
	findings := []lint.PackageFinding{}
	compFindings, err := lintComponents(ctx, pkg, flavor, setVariables)
	if err != nil {
		return err
	}
	findings = append(findings, compFindings...)
	schemaFindings, err := lint.ValidatePackageSchema(setVariables)
	if err != nil {
		return err
	}
	findings = append(findings, schemaFindings...)
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		BaseDir:     baseDir,
		PackageName: pkg.Metadata.Name,
		Findings:    findings,
	}
}

func lintComponents(ctx context.Context, pkg v1alpha1.ZarfPackage, flavor string, setVariables map[string]string) ([]lint.PackageFinding, error) {
	findings := []lint.PackageFinding{}
	for i, component := range pkg.Components {
		arch := config.GetArch(pkg.Metadata.Architecture)
		if !composer.CompatibleComponent(component, arch, flavor) {
			continue
		}
		findings = append(findings, lint.CheckComponentValues(component, i)...)
	}
	return findings, nil
}
