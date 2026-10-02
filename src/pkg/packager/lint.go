// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
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

// Lint lints a Zarf package or component config.
func Lint(ctx context.Context, packagePath string, opts LintOptions) (err error) {
	if packagePath == "" {
		return errors.New("package path is required")
	}

	opts.CachePath, err = utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return err
	}
	path, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(path.ManifestFile)
	if err != nil {
		return err
	}
	header, err := load.ParseDefinitionHeader(contents)
	if err != nil {
		return err
	}
	if header.Kind == string(v1beta1.ZarfComponentConfig) {
		component, err := load.Component(ctx, path.ManifestFile, load.ComponentOptions{
			CachePath:     opts.CachePath,
			RemoteOptions: opts.RemoteOptions,
		})
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, component.Close())
		}()
		if validationErrs := internalv1beta1.ValidateComponent(v1beta1.Component{
			Name:          component.Definition.Metadata.Name,
			ComponentSpec: component.Definition.Component,
		}); len(validationErrs) > 0 {
			return fmt.Errorf("component validation failed:\n%w", validationErrs)
		}
		findings := lint.CheckComponentValuesV1Beta1(component.Definition.Component, ".component")
		if len(findings) == 0 {
			return nil
		}
		return &lint.LintError{PackageName: component.Definition.Metadata.Name, Findings: findings}
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
	findings := []lint.PackageFinding{}
	switch loaded.Definition.GetAPIVersion() {
	case v1alpha1.APIVersion:
		pkg := convert.PackageToV1alpha1(loaded.Definition)
		for i, component := range pkg.Components {
			findings = append(findings, lint.CheckComponentValues(component, i)...)
		}
	case v1beta1.APIVersion:
		pkg := convert.PackageToV1beta1(loaded.Definition)
		for i, component := range pkg.Components {
			findings = append(findings, lint.CheckComponentValuesV1Beta1(component.ComponentSpec, fmt.Sprintf(".components.[%d]", i))...)
		}
	default:
		return fmt.Errorf("linting packages with apiVersion %q is not supported", loaded.Definition.GetAPIVersion())
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: loaded.Definition.Metadata.Name,
		Findings:    findings,
	}
}
