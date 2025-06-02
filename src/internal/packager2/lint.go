// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"

	"github.com/zarf-dev/zarf/src/internal/packager2/create"
	"github.com/zarf-dev/zarf/src/pkg/lint"
)

// LintOptions are the optional parameters to Lint
type LintOptions struct {
	SetVariables map[string]string
	Flavor       string
}

// Lint lints the given Zarf package
func Lint(ctx context.Context, packagePath string, opts LintOptions) error {
	if packagePath == "" {
		return errors.New("package path is required")
	}
	pkg, err := create.LoadPackageDefinition(ctx, packagePath, opts.Flavor, opts.SetVariables)
	if err != nil {
		return err
	}
	findings := []lint.PackageFinding{}
	for i, component := range pkg.Components {
		findings = append(findings, lint.CheckComponentValues(component, i)...)
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: pkg.Metadata.Name,
		Findings:    findings,
	}
}
