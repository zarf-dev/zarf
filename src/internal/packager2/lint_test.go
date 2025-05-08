// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestLintPackageWithImports(t *testing.T) {
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	setVariables := map[string]string{
		"BUSYBOX_IMAGE": "latest",
		"UNSET":         "actually-is-set",
	}
	ctx := context.Background()
	findings := []lint.PackageFinding{
		// unset exists in both the root and imported package
		{
			YqPath:              "",
			Description:         "package template UNSET is not set and won't be evaluated during lint",
			Item:                "",
			PackageNameOverride: "linted-import",
			PackagePathOverride: "linted-import",
			Severity:            lint.SevWarn,
		},
		{
			YqPath:              "",
			Description:         "package template UNSET is not set and won't be evaluated during lint",
			Item:                "",
			PackageNameOverride: "lint",
			PackagePathOverride: ".",
			Severity:            lint.SevWarn,
		},
		// Test imported skeleton package lints properly
		{
			YqPath:              ".components.[0].images.[0]",
			Description:         "Image not pinned with digest",
			Item:                "ghcr.io/zarf-dev/doom-game:0.0.1",
			PackageNameOverride: "dos-games",
			PackagePathOverride: "oci://ghcr.io/zarf-dev/packages/dos-games:1.2.0",
			Severity:            lint.SevWarn,
		},
		// Test local import lints properly
		{
			YqPath:              ".components.[1].images.[0]",
			Description:         "Image not pinned with digest",
			Item:                "busybox:latest",
			PackageNameOverride: "linted-import",
			PackagePathOverride: "linted-import",
			Severity:            lint.SevWarn,
		},
		// Test flavors
		{
			YqPath:              ".components.[4].images.[0]",
			Description:         "Image not pinned with digest",
			Item:                "image-in-good-flavor-component:unpinned",
			PackageNameOverride: "lint",
			PackagePathOverride: ".",
			Severity:            lint.SevWarn,
		},
	}
	pkg, err := layout2.LoadPackageDefinition(ctx, "testdata/lint-with-imports", "good-flavor", setVariables)
	require.NoError(t, err)
	err = Validate(ctx, pkg, "testdata/lint-with-imports", "good-flavor", setVariables)
	var lintErr *lint.LintError
	require.ErrorAs(t, err, &lintErr)
	require.ElementsMatch(t, findings, lintErr.Findings)
}
