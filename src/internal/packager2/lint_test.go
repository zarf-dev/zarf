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

	testCases := []struct {
		name     string
		path     string
		opts     LintOptions
		findings []lint.PackageFinding
	}{
		{
			name: "compose test ",
			path: filepath.Join("testdata", "lint-with-imports", "compose"),
			findings: []lint.PackageFinding{
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image not pinned with digest",
					Item:        "busybox:0.0.1",
					Severity:    lint.SevWarn,
				},
			},
		},
		{
			name: "variables test",
			path: filepath.Join("testdata", "lint-with-imports", "variables"),
			opts: LintOptions{
				SetVariables: map[string]string{
					"BUSYBOX_TAG": "1.0.0",
				},
			},
			findings: []lint.PackageFinding{
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image not pinned with digest",
					Item:        "busybox:1.0.0",
					Severity:    lint.SevWarn,
				},
			},
		},
		{
			name: "flavor test",
			path: filepath.Join("testdata", "lint-with-imports", "flavor"),
			opts: LintOptions{
				Flavor: "good-flavor",
			},
			findings: []lint.PackageFinding{
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image not pinned with digest",
					Item:        "image-in-good-flavor-component:unpinned",
					Severity:    lint.SevWarn,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			err := Lint(ctx, tc.path, tc.opts)
			var lintErr *lint.LintError
			require.ErrorAs(t, err, &lintErr)
			require.ElementsMatch(t, tc.findings, lintErr.Findings)
		})
	}
}
