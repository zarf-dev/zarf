// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"

	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/types"
)

// LintOptions are the optional parameters to Lint
type LintOptions struct {
	SetVariables map[string]string
	Flavor       string
	CachePath    string
	types.RemoteOptions
}

// Lint lints the given Zarf package
func Lint(ctx context.Context, packagePath string, opts LintOptions) error {
	if packagePath == "" {
		return errors.New("package path is required")
	}
	loadOpts := load.DefinitionOptions{
		Flavor:           opts.Flavor,
		SetVariables:     opts.SetVariables,
		CachePath:        opts.CachePath,
		IsInteractive:    false,
		SkipVersionCheck: true,
		RemoteOptions:    opts.RemoteOptions,
	}
	result, err := load.PackageDefinition(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}
	pkg := result.Package
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
