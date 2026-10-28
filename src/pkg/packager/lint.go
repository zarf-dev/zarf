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
func Lint(ctx context.Context, packagePath string, opts LintOptions) error {
	if packagePath == "" {
		return errors.New("package path is required")
	}

	cachePath, err := utils.ResolveCachePath(opts.CachePath)
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
		component, err := load.ComponentDefinition(ctx, path.ManifestFile, load.ComponentOptions{
			CachePath:     cachePath,
			RemoteOptions: opts.RemoteOptions,
		})
		if err != nil {
			return err
		}
		findings := lint.CheckComponentValuesV1Beta1(component.Component, ".component")
		if len(findings) == 0 {
			return nil
		}
		return &lint.LintError{PackageName: component.Metadata.Name, Findings: findings}
	}

	loadOpts := load.DefinitionOptions{
		Flavor:           opts.Flavor,
		SetVariables:     opts.SetVariables,
		CachePath:        cachePath,
		IsInteractive:    false,
		SkipVersionCheck: true,
		RemoteOptions:    opts.RemoteOptions,
	}
	definition, err := load.PackageDefinition(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}
	findings := []lint.PackageFinding{}
	switch definition.GetAPIVersion() {
	case v1alpha1.APIVersion:
		pkg := convert.PackageToV1alpha1(definition)
		for i, component := range pkg.Components {
			findings = append(findings, lint.CheckComponentValues(component, i)...)
		}
	case v1beta1.APIVersion:
		pkg := convert.PackageToV1beta1(definition)
		for i, component := range pkg.Components {
			findings = append(findings, lint.CheckComponentValuesV1Beta1(component.ComponentSpec, fmt.Sprintf(".components.[%d]", i))...)
		}
	default:
		return fmt.Errorf("linting packages with apiVersion %q is not supported", definition.GetAPIVersion())
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		PackageName: definition.Metadata.Name,
		Findings:    findings,
	}
}
