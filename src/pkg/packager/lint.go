// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"

	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// LintOptions are the optional parameters to Lint
type LintOptions struct {
	SetVariables       map[string]string
	Flavor             string
	SkipVariantFilters []load.VariantDimension
	CachePath          string
	types.RemoteOptions
}

// Lint lints the given Zarf package
func Lint(ctx context.Context, packagePath string, opts LintOptions) (err error) {
	if packagePath == "" {
		return errors.New("package path is required")
	}

	opts.CachePath, err = utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return err
	}

	loadOpts := load.PackageOptions{
		DefinitionOptions: load.DefinitionOptions{
			Flavor:             opts.Flavor,
			SetVariables:       opts.SetVariables,
			CachePath:          opts.CachePath,
			IsInteractive:      false,
			SkipVersionCheck:   true,
			RemoteOptions:      opts.RemoteOptions,
			SkipVariantFilters: opts.SkipVariantFilters,
		},
	}
	loaded, err := load.Package(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, loaded.Close())
	}()
	pkg := loaded.Definition.AsV1alpha1()
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
