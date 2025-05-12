// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager2

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestLintPackageWithImports(t *testing.T) {
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	setVariables := map[string]string{
		"BUSYBOX_TAG": "1.0.0",
	}
	ctx := context.Background()
	findings := []lint.PackageFinding{
		// Test local import lints properly
		{
			YqPath:      ".components.[0].images.[0]",
			Description: "Image not pinned with digest",
			Item:        "busybox:1.0.0",
			Severity:    lint.SevWarn,
		},
		// Test imported skeleton package lints properly
		{
			YqPath:      ".components.[2].images.[0]",
			Description: "Image not pinned with digest",
			Item:        "ghcr.io/zarf-dev/doom-game:0.0.1",
			Severity:    lint.SevWarn,
		},
		// Test flavors
		{
			YqPath:      ".components.[3].images.[0]",
			Description: "Image not pinned with digest",
			Item:        "image-in-good-flavor-component:unpinned",
			Severity:    lint.SevWarn,
		},
	}
	err := Lint(ctx, filepath.Join("testdata", "lint-with-imports"), "good-flavor", setVariables)
	var lintErr *lint.LintError
	require.ErrorAs(t, err, &lintErr)
	require.ElementsMatch(t, findings, lintErr.Findings)
}
