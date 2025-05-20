// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/deprecated"
	"github.com/zarf-dev/zarf/src/types"
)

// Validate errors if a package violates the schema or any runtime validations
// This must be run while in the parent directory of the zarf.yaml being validated
func Validate(pkg v1alpha1.ZarfPackage, baseDir string, setVariables map[string]string) error {
	if err := lint.ValidatePackage(pkg); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}
	findings, err := lint.ValidatePackageSchema(setVariables)
	if err != nil {
		return fmt.Errorf("unable to check schema: %w", err)
	}
	if len(findings) == 0 {
		return nil
	}
	return &lint.LintError{
		BaseDir:     baseDir,
		PackageName: pkg.Metadata.Name,
		Findings:    findings,
	}
}

// recordPackageMetadata records various package metadata during package create.
func recordPackageMetadata(pkg *v1alpha1.ZarfPackage, createOpts types.ZarfCreateOptions) error {
	now := time.Now()
	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		pkg.Build.User = os.Getenv("USERNAME")
	} else {
		pkg.Build.User = os.Getenv("USER")
	}

	// Record the hostname of the package creation terminal.
	// The error here is ignored because the hostname is not critical to the package creation.
	hostname, _ := os.Hostname()
	pkg.Build.Terminal = hostname

	if pkg.IsInitConfig() && pkg.Metadata.Version == "" {
		pkg.Metadata.Version = config.CLIVersion
	}

	pkg.Build.Architecture = pkg.Metadata.Architecture

	// Record the Zarf Version the CLI was built with.
	pkg.Build.Version = config.CLIVersion

	// Record the time of package creation.
	pkg.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the migrations that will be ran on the package.
	pkg.Build.Migrations = []string{
		deprecated.ScriptsToActionsMigrated,
		deprecated.PluralizeSetVariable,
	}

	// Record the flavor of Zarf used to build this package (if any).
	pkg.Build.Flavor = createOpts.Flavor

	pkg.Build.RegistryOverrides = createOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	pkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return nil
}
