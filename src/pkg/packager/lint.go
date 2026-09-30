// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"

	"github.com/zarf-dev/zarf/src/api/convert"
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
			Flavor:           opts.Flavor,
			SetVariables:     opts.SetVariables,
			CachePath:        opts.CachePath,
			IsInteractive:    false,
			SkipVersionCheck: true,
			RemoteOptions:    opts.RemoteOptions,
		},
	}
	loaded, err := load.Package(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, loaded.Close())
	}()
	if loaded.Definition.GetAPIVersion() != v1alpha1.APIVersion {
		return fmt.Errorf("linting packages with apiVersion %q is not yet supported; only %s is supported", loaded.Definition.GetAPIVersion(), v1alpha1.APIVersion)
	}
	pkg := convert.PackageToV1alpha1(loaded.Definition)
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
