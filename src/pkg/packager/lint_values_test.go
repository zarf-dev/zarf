// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

// These fixtures enter through Lint: setup supplies source inputs, never rendered
// files, coalesced chart values, or action-generated output owned by production.
func lintSource(t *testing.T, spec v1beta1.ComponentSpec, values v1beta1.Values, files map[string]string) string {
	t.Helper()
	pkg := v1beta1.Package{APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfPackageConfig,
		Values: values, Components: []v1beta1.Component{{Name: "app", ComponentSpec: spec}}}
	pkg.Metadata.Name = "lint-source"
	return testutil.WritePackage(t, pkg, files)
}

func lintSourceSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[rel] = string(content)
		return nil
	}))
	return files
}

func TestLintNeverExecutesActions(t *testing.T) {
	t.Parallel()
	generated := filepath.Join(t.TempDir(), "generated.txt")
	sentinel := filepath.Join(t.TempDir(), "action-ran")
	action := v1beta1.ComponentAction{Cmd: fmt.Sprintf("touch %q", sentinel), EnableTemplating: true}
	dir := lintSource(t, v1beta1.ComponentSpec{
		Images: []v1beta1.Image{{Name: "example.com/app:1.0"}},
		Files:  []v1beta1.File{{Source: generated, Destination: "/tmp/generated", EnableTemplating: true}},
		Actions: v1beta1.ComponentActions{
			OnCreate: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{{Cmd: fmt.Sprintf("printf '{{ .Constants.TOKEN }}' > %q", generated)}}},
			OnDeploy: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{{Cmd: "{{", EnableTemplating: true}}, OnSuccess: []v1beta1.ComponentAction{action}},
			OnRemove: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{action}},
		},
	}, v1beta1.Values{}, nil)
	before := lintSourceSnapshot(t, dir)
	result := Lint(t.Context(), dir, LintOptions{})
	require.ErrorContains(t, result.Err, "generated.txt")
	require.ErrorContains(t, result.Err, ".components.[0].files.[0]")
	require.True(t, result.FailsAt(lint.SevErr))
	require.Len(t, result.Findings, 2)
	require.Contains(t, result.Findings, lint.PackageFinding{RuleID: "image-unpinned", Source: "composed definition", YqPath: ".components.[0].images.[0]", Description: "Image not pinned with digest", Item: "example.com/app:1.0", Severity: lint.SevInfo})
	var rules []string
	for _, finding := range result.Findings {
		rules = append(rules, finding.RuleID)
		if finding.RuleID == "template-syntax" {
			require.Equal(t, "composed definition", finding.Source)
			require.Equal(t, ".components.[0].actions.onDeploy.before.[0]", finding.YqPath)
			require.Equal(t, lint.Severity(lint.SevErr), finding.Severity)
		}
	}
	require.ElementsMatch(t, []string{"image-unpinned", "template-syntax"}, rules)
	require.ElementsMatch(t, []string{".components.[0].files.[0]: resource unavailable", ".components.[0].actions.onDeploy.before.[0]: template syntax is invalid"}, result.NotEvaluated)
	require.NoFileExists(t, generated)
	require.NoFileExists(t, sentinel)
	require.Equal(t, before, lintSourceSnapshot(t, dir))
}

func TestLintLocalTemplatedResources(t *testing.T) {
	t.Parallel()
	dir := lintSource(t, v1beta1.ComponentSpec{
		Charts: []v1beta1.Chart{{Name: "web", Namespace: "default", Local: &v1beta1.LocalSource{Path: "chart"},
			ValuesFiles: []v1beta1.ValuesFile{{Path: "web-values.yaml", EnableTemplating: true}, {Path: "literal-values.yaml"}}}},
		Files:     []v1beta1.File{{Source: "settings.txt", Destination: "/tmp/settings", EnableTemplating: true}, {Source: "literal.txt", Destination: "/tmp/literal"}},
		Manifests: []v1beta1.Manifest{{Name: "app", Files: []string{"manifest.yaml"}, EnableTemplating: true}, {Name: "literal", Files: []string{"literal.yaml"}}},
	}, v1beta1.Values{Files: []string{"defaults.yaml"}, Schema: "schema.json"}, map[string]string{
		"defaults.yaml": "replicas: invalid\nfileValue: file\nmanifestValue: manifest\n",
		"schema.json":   `{"type":"object","properties":{"replicas":{"type":"integer"}}}`,
		"settings.txt":  "{{ .Values.fileValue }}\n", "literal.txt": "{{ invalid-template\n",
		"manifest.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: '{{ .Values.manifestValue }}'\n", "literal.yaml": "{{ invalid-template\n",
		"web-values.yaml": "replicas: {{ .Values.replicas }}\n", "literal-values.yaml": "literal: '{{ invalid-template'\n",
		"chart/Chart.yaml": "apiVersion: v2\nname: web\nversion: 0.1.0\n", "chart/values.yaml": "replicas: invalid\n",
		"chart/values.schema.json": `{"type":"object","required":["replicas"],"properties":{"replicas":{"type":"integer","const":2}}}`,
		// Helm templates belong to Helm, not Zarf's template parser or renderer.
		"chart/templates/helm.yaml": "{{ deliberately-not-a-zarf-function\n",
	})
	before := lintSourceSnapshot(t, dir)
	result := Lint(t.Context(), dir, LintOptions{Values: value.Values{"replicas": 2}})
	require.NoError(t, result.Err)
	require.Empty(t, result.Findings)
	require.Empty(t, result.NotEvaluated)
	require.Equal(t, before, lintSourceSnapshot(t, dir))
}

func TestLintRejectsInvalidSkipsBeforeLoading(t *testing.T) {
	t.Parallel()
	result := Lint(t.Context(), filepath.Join(t.TempDir(), "nonexistent.yaml"), LintOptions{SkipRules: []string{"unknown-rule"}})
	require.ErrorContains(t, result.Err, "unknown-rule")
	require.NotContains(t, result.Err.Error(), "nonexistent.yaml")
}

func TestLintLegacyVariableTemplating(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	contents := map[string]string{
		"zarf.yaml": `apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: legacy-source
variables:
  - name: NAME
    default: rendered
components:
  - name: app
    files:
      - source: settings.txt
        target: /tmp/settings
        template: true
    manifests:
      - name: app
        files: [manifest.yaml]
        template: true
`,
		"settings.txt":  `{{ if ne "###ZARF_VAR_NAME###" "rendered" }}{{ fail "legacy replacement did not run" }}{{ end }}`,
		"manifest.yaml": `{{ if ne "###ZARF_VAR_NAME###" "rendered" }}{{ fail "legacy replacement did not run" }}{{ end }}`,
	}
	for name, content := range contents {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	before := lintSourceSnapshot(t, dir)
	result := Lint(t.Context(), dir, LintOptions{})
	require.NoError(t, result.Err)
	require.Empty(t, result.Findings)
	require.Empty(t, result.NotEvaluated)
	require.Equal(t, before, lintSourceSnapshot(t, dir))
}

func TestLintStagingCleanup(t *testing.T) {
	// Isolate TMPDIR in a child process: do not mutate shared config or the test
	// process environment while the other lint cases run in parallel.
	if source := os.Getenv("ZARF_TEST_LINT_CLEANUP_SOURCE"); source != "" {
		opts := LintOptions{}
		if os.Getenv("ZARF_TEST_LINT_CLEANUP_RESULT") == "schema-skipped" {
			opts.SkipRules = []string{lint.RuleChartValuesSchema}
		}
		result := Lint(t.Context(), source, opts)
		switch os.Getenv("ZARF_TEST_LINT_CLEANUP_RESULT") {
		case "success":
			require.NoError(t, result.Err)
			require.Empty(t, result.Findings)
		case "template":
			require.NoError(t, result.Err)
			require.Len(t, result.Findings, 1)
			require.Equal(t, "template-syntax", result.Findings[0].RuleID)
		case "schema", "schema-skipped":
			require.NoError(t, result.Err)
			require.Empty(t, result.NotEvaluated)
			require.Equal(t, opts.SkipRules, result.SkippedRules)
			if len(opts.SkipRules) > 0 {
				require.Empty(t, result.Findings)
				require.False(t, result.FailsAt(lint.SevErr))
			} else {
				require.Len(t, result.Findings, 1)
				finding := result.Findings[0]
				require.Equal(t, lint.PackageFinding{RuleID: lint.RuleChartValuesSchema, Source: "composed definition", YqPath: ".components.[0].charts.[0]", Severity: lint.SevErr, Description: finding.Description}, finding)
				require.Contains(t, finding.Description, "replicas")
				require.True(t, result.FailsAt(lint.SevErr))
			}
		case "resource":
			require.ErrorContains(t, result.Err, "missing.txt")
		default:
			t.Fatal("invalid cleanup test result")
		}
		return
	}
	t.Parallel()
	for _, outcome := range []string{"success", "template", "resource", "schema", "schema-skipped"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			content, source := "literal content\n", "settings.txt"
			if outcome == "template" {
				content = "{{"
			}
			if outcome == "resource" {
				source = "missing.txt"
			}
			spec := v1beta1.ComponentSpec{Files: []v1beta1.File{{Source: source, Destination: "/tmp/settings", EnableTemplating: true}}}
			files := map[string]string{"settings.txt": content}
			if outcome == "schema" || outcome == "schema-skipped" {
				spec.Charts = []v1beta1.Chart{{Name: "web", Namespace: "default", Local: &v1beta1.LocalSource{Path: "chart"}}}
				files["chart/Chart.yaml"] = "apiVersion: v2\nname: web\nversion: 0.1.0\n"
				files["chart/values.yaml"] = "replicas: invalid\n"
				files["chart/values.schema.json"] = `{"type":"object","properties":{"replicas":{"type":"integer"}}}`
			}
			dir := lintSource(t, spec, v1beta1.Values{}, files)
			before := lintSourceSnapshot(t, dir)
			staging := t.TempDir()
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLintStagingCleanup$", "-test.count=1")
			cmd.Env = append(os.Environ(), "TMPDIR="+staging, "ZARF_TEST_LINT_CLEANUP_SOURCE="+dir, "ZARF_TEST_LINT_CLEANUP_RESULT="+outcome)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
			remaining, err := os.ReadDir(staging)
			require.NoError(t, err)
			require.Empty(t, remaining)
			require.Equal(t, before, lintSourceSnapshot(t, dir))
		})
	}
}
