// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"os"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// LintOptions configures source linting.
type LintOptions = lint.Options

// Lint resolves source resources, runs lint analysis, and closes the resources.
func Lint(ctx context.Context, packagePath string, opts LintOptions) *lint.LintError {
	if err := lint.ValidateSkipRules(opts.SkipRules); err != nil {
		return &lint.LintError{Err: err}
	}
	if packagePath == "" {
		return &lint.LintError{Err: errors.New("package path is required")}
	}
	cachePath, err := utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return &lint.LintError{Err: err}
	}
	opts.CachePath = cachePath
	path, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return &lint.LintError{Err: err}
	}
	contents, err := os.ReadFile(path.ManifestFile)
	if err != nil {
		return &lint.LintError{Err: err}
	}
	header, err := load.ParseDefinitionHeader(contents)
	if err != nil {
		return &lint.LintError{Err: err}
	}
	// The analyzer validates the effective values after overrides are merged.
	// Loading still parses and validates every schema document.
	loaded, err := loadDefinition(ctx, packagePath, load.PackageOptions{
		DefinitionOptions: load.DefinitionOptions{
			Flavor: opts.Flavor, SetVariables: opts.CreateSetVariables, CachePath: opts.CachePath,
			IsInteractive: false, SkipVersionCheck: true, RemoteOptions: opts.RemoteOptions,
		},
		SkipValuesSchemaValidation: true,
	})
	if err != nil {
		return &lint.LintError{Err: err}
	}
	result := lint.Analyze(ctx, loaded, "composed definition", header.Kind == string(v1beta1.ZarfComponentConfig), opts)
	result.Err = errors.Join(result.Err, loaded.Close())
	return result
}
