// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/composer"
)

type LintOptions struct {
	SetVariables map[string]string
	Flavor       string
}

// Lint lints the given Zarf package
func Lint(ctx context.Context, packagePath string, opts LintOptions) error {
	if packagePath == "" {
		return errors.New("package path is required")
	}
	pkg, err := layout2.LoadPackageDefinition(ctx, packagePath, opts.Flavor, opts.SetVariables)
	if err != nil {
		return err
	}
	findings, err := lintComponents(pkg, opts.Flavor)
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: pkg.Metadata.Name,
		Findings:    findings,
	}
}

func lintComponents(pkg v1alpha1.ZarfPackage, flavor string) ([]lint.PackageFinding, error) {
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
