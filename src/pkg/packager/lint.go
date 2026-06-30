// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/utils"
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

	var err error
	opts.CachePath, err = utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return err
	}

	loadOpts := load.DefinitionOptions{
		Flavor:           opts.Flavor,
		SetVariables:     opts.SetVariables,
		CachePath:        opts.CachePath,
		IsInteractive:    false,
		SkipVersionCheck: true,
		RemoteOptions:    opts.RemoteOptions,
	}
	defined, err := load.PackageDefinition(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}
	findings := []lint.PackageFinding{}
	declared := declaredVariableNames(defined.Pkg)
	for i, component := range defined.Pkg.Components {
		findings = append(findings, lint.CheckComponentValues(component, i)...)
		findings = append(findings, lint.CheckOnlyVariableReferences(component, i, declared)...)
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: defined.Pkg.Metadata.Name,
		Findings:    findings,
	}
}

func declaredVariableNames(pkg v1alpha1.ZarfPackage) map[string]struct{} {
	names := make(map[string]struct{}, len(pkg.Variables)+len(pkg.Constants))
	for _, v := range pkg.Variables {
		names[v.Name] = struct{}{}
	}
	for _, c := range pkg.Constants {
		names[c.Name] = struct{}{}
	}
	return names
}
