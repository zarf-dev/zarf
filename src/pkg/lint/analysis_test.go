// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package lint_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

const (
	definition   = "composed definition"
	component    = ".components.[0]"
	actionPath   = component + ".actions.onDeploy.before.[0]"
	chartPath    = component + ".charts.[0]"
	chartBlocked = chartPath + ": chart schema validation requires resolved mappings and rendered values files"
)

// A fixture supplies authored source only. Loading owns resolution; Analyze owns
// overrides, rendering, schema validation, and staging; the caller owns Close.
type sourceFixture struct {
	spec   v1beta1.ComponentSpec
	values v1beta1.Values
	files  map[string]string
}

func (f sourceFixture) analyze(t *testing.T, opts lint.Options) *lint.LintError {
	t.Helper()
	dir := t.TempDir()
	pkg := v1beta1.Package{APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfPackageConfig,
		Values: f.values, Components: []v1beta1.Component{{Name: "app", ComponentSpec: f.spec}}}
	pkg.Metadata.Name = "analysis"
	contents, err := yaml.Marshal(pkg)
	require.NoError(t, err)
	for name, content := range f.files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), contents, 0o600))
	loaded, err := load.Package(t.Context(), dir, load.PackageOptions{
		DefinitionOptions: load.DefinitionOptions{SkipVersionCheck: true}, SkipValuesSchemaValidation: true,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, loaded.Close()) }()
	result := lint.Analyze(t.Context(), loaded, definition, false, opts)
	require.Equal(t, "analysis", result.PackageName)
	return result
}

// This complete local chart source is shared by mapping, rendering, exemption,
// and dependency cases; tests change authored inputs, never assembled output.
func chartSource() sourceFixture {
	return sourceFixture{
		spec:   v1beta1.ComponentSpec{Charts: []v1beta1.Chart{{Name: "web", Namespace: "default", Local: &v1beta1.LocalSource{Path: "chart"}}}},
		values: v1beta1.Values{Schema: "schema.json"},
		files:  map[string]string{"schema.json": `{"type":"object"}`, "chart/Chart.yaml": "apiVersion: v2\nname: web\nversion: 0.1.0\n", "chart/values.yaml": "{}\n"},
	}
}

type findingContract struct {
	rule, source, path string
	severity           lint.Severity
	message            string
}

func requireFindings(t *testing.T, result *lint.LintError, want []findingContract, blocked ...string) {
	t.Helper()
	require.Len(t, result.Findings, len(want))
	for _, expected := range want {
		var matches []lint.PackageFinding
		for _, actual := range result.Findings {
			if actual.RuleID == expected.rule && actual.Source == expected.source && actual.YqPath == expected.path {
				matches = append(matches, actual)
			}
		}
		require.Len(t, matches, 1, "%+v", expected)
		actual := matches[0]
		require.Equal(t, expected.severity, actual.Severity)
		require.Empty(t, actual.Item)
		require.Contains(t, actual.Description, expected.message)
	}
	require.NotContains(t, fmt.Sprintf("%+v", result.Findings), "secret-")
	require.ElementsMatch(t, blocked, result.NotEvaluated)
}

func requireFatal(t *testing.T, result *lint.LintError, messages ...string) {
	t.Helper()
	for _, message := range messages {
		require.ErrorContains(t, result.Err, message)
	}
	for _, level := range []lint.Severity{lint.SevInfo, lint.SevWarn, lint.SevErr} {
		require.True(t, result.FailsAt(level), "fatal prerequisite at %s", level)
	}
}

func TestAnalyzeEffectiveValues(t *testing.T) {
	t.Parallel()
	for _, repair := range []bool{false, true} {
		t.Run(fmt.Sprintf("repair=%t", repair), func(t *testing.T) {
			f := chartSource()
			f.values.Files = []string{"defaults.yaml", "later.yaml"}
			f.files["defaults.yaml"] = "app:\n  replicas: invalid\n  label: first\n  kept: baseline\n"
			f.files["later.yaml"] = "app:\n  label: second\n"
			f.files["schema.json"] = `{"type":"object","required":["notNeededYet"],"properties":{"app":{"type":"object","properties":{"replicas":{"type":"integer"}}}}}`
			f.files["chart/values.yaml"] = "app:\n  replicas: 0\n"
			f.files["chart/values.schema.json"] = `{"type":"object","properties":{"app":{"type":"object","required":["replicas","label","kept"],"properties":{"replicas":{"const":3},"label":{"const":"second"},"kept":{"const":"baseline"}}}}}`
			f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: ".app", TargetPath: ".app"}}
			f.spec.Actions.OnDeploy.Before = []v1beta1.ComponentAction{{Cmd: `{{ if or (ne .Values.app.replicas 3) (ne .Values.app.label "second") (ne .Values.app.kept "baseline") }}{{ fail "incorrect effective values" }}{{ end }}`, EnableTemplating: true}}
			opts := lint.Options{}
			if repair {
				opts.Values = value.Values{"app": map[string]any{"replicas": 3}}
			}
			result := f.analyze(t, opts)
			requireFindings(t, result, nil)
			if repair {
				require.NoError(t, result.Err)
			} else {
				requireFatal(t, result, "app.replicas", "integer")
			}
		})
	}
}

func TestAnalyzeValuesSchemaMissing(t *testing.T) {
	t.Parallel()
	for _, consumer := range []string{"defaults", "inactive template", "mapping"} {
		t.Run(consumer, func(t *testing.T) {
			f := chartSource()
			f.values.Schema = ""
			switch consumer {
			case "defaults":
				f.values.Files = []string{"defaults.yaml"}
				f.files["defaults.yaml"] = "input: present\n"
				f.spec.Actions.OnDeploy.Before = []v1beta1.ComponentAction{{Cmd: "{{ .Values.input }}", EnableTemplating: true}}
			case "inactive template":
				f.spec.Actions.OnDeploy.Before = []v1beta1.ComponentAction{{Cmd: "{{ if false }}{{ .Values.input }}{{ end }}", EnableTemplating: true}}
			case "mapping":
				f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: ".input", TargetPath: ".input"}}
			}
			result := f.analyze(t, lint.Options{Values: value.Values{"input": "supplied"}})
			require.NoError(t, result.Err)
			requireFindings(t, result, []findingContract{{"values-schema-missing", definition, ".values", lint.SevWarn, "define a values schema"}})
			require.False(t, result.FailsAt(lint.SevErr))
		})
	}
}

func TestAnalyzeUnusedValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, command        string
		mapping, secondChart bool
		unused               []string
	}{
		{name: "no consumers", unused: []string{"app.drop", "app.keep", "enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "object read", command: "{{ .Values.app | toYaml }}", unused: []string{"enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "mapping excludes descendant", mapping: true, unused: []string{"app.drop", "enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "second chart consumes exclusion", mapping: true, secondChart: true, unused: []string{"enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "template consumes exclusion", mapping: true, command: "{{ .Values.app.drop }}", unused: []string{"enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "parent consumes exclusion", mapping: true, command: "{{ .Values.app | toYaml }}", unused: []string{"enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "root", command: "{{ .Values | toYaml }}"},
		{name: "root alias", command: "{{ $v := .Values }}{{ $v.app.keep }}"},
		{name: "dynamic subtree", command: "{{ index .Values.profiles .Values.selected }}", unused: []string{"app.drop", "app.keep", "enabled"}},
		{name: "dynamic root", command: "{{ index .Values .Values.selected }}"},
		{name: "inactive branch", command: "{{ if .Values.enabled }}{{ .Values.app.keep }}{{ end }}", unused: []string{"app.drop", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "literal index", command: `{{ index .Values "app" "keep" }}`, unused: []string{"app.drop", "enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
		{name: "rebound dot", command: "{{ with .Values.app }}{{ .keep }}{{ end }}", unused: []string{"enabled", "profiles.blue.name", "profiles.red.name", "selected"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := chartSource()
			f.values.Files = []string{"defaults.yaml"}
			f.files["defaults.yaml"] = "app:\n  keep: yes\n  drop: no\nenabled: false\nprofiles:\n  blue:\n    name: blue\n  red:\n    name: red\nselected: blue\n"
			f.spec.Actions.OnDeploy.Before = []v1beta1.ComponentAction{{Cmd: tc.command, EnableTemplating: true}}
			if tc.mapping {
				f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: ".app", TargetPath: ".config", ExcludePaths: []string{".app.drop"}}}
				if tc.secondChart {
					other := f.spec.Charts[0]
					other.Name, other.Values = "second", []v1beta1.ChartValue{{SourcePath: ".app.drop", TargetPath: ".separate"}}
					f.spec.Charts = append(f.spec.Charts, other)
				}
			}
			result := f.analyze(t, lint.Options{Values: value.Values{"suppliedOnly": "not a default"}})
			var want []findingContract
			for _, path := range tc.unused {
				want = append(want, findingContract{"values-unused", definition, ".values." + path, lint.SevWarn, "potentially unused default value"})
			}
			require.NoError(t, result.Err)
			requireFindings(t, result, want)
		})
	}
}

func TestAnalyzeUnusedDefaultsExcludedFromRootMapping(t *testing.T) {
	t.Parallel()
	f := chartSource()
	f.values.Files = []string{"defaults.yaml"}
	f.files["defaults.yaml"] = "common:\n  replicas: 2\n  images:\n    repository: example.com/app\n"
	f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: ".common", TargetPath: ".", ExcludePaths: []string{".common.images"}}}
	result := f.analyze(t, lint.Options{})
	require.NoError(t, result.Err)
	requireFindings(t, result, []findingContract{{lint.RuleValuesUnused, definition, ".values.common.images.repository", lint.SevWarn, "potentially unused default value"}})
}

func TestAnalyzeActionTemplates(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, content, rule, message, blocked string
		disabled                              bool
	}{
		{name: "syntax", content: "{{", rule: "template-syntax", message: "invalid Go template:", blocked: "template syntax is invalid"},
		{name: "evaluation", content: `{{ fail "secret-content" }}`, rule: "template-render", message: "template evaluation failed", blocked: "template evaluation failed"},
		{name: "function error is not missing input", content: `{{ fail "map has no entry for key \"TOKEN\"" }}`, rule: "template-render", message: "template evaluation failed", blocked: "template evaluation failed"},
		{name: "disabled syntax", content: "{{", disabled: true},
		{name: "disabled evaluation", content: `{{ fail "secret-content" }}`, disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := sourceFixture{spec: v1beta1.ComponentSpec{Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{
				Before: []v1beta1.ComponentAction{{Cmd: tc.content, EnableTemplating: !tc.disabled}},
			}}}}
			result := f.analyze(t, lint.Options{})
			require.NoError(t, result.Err)
			if tc.rule == "" {
				requireFindings(t, result, nil)
			} else {
				requireFindings(t, result, []findingContract{{tc.rule, definition, actionPath, lint.SevErr, tc.message}}, actionPath+": "+tc.blocked)
			}
		})
	}
}

func TestAnalyzeMissingInputs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, command, key, remedy string
		opts                       lint.Options
	}{
		{name: "value", command: "{{ .Values.ingress.hostname }}", key: "ingress.hostname", remedy: "--set-values ingress.hostname=<value>"},
		{name: "variable", command: "{{ .Variables.TOKEN }}", key: ".Variables.TOKEN", remedy: "--deploy-set-variables TOKEN=<value>"},
		{name: "constant", command: "{{ .Constants.TOKEN }}", key: ".Constants.TOKEN", remedy: "define package constant TOKEN"},
		{name: "alias", command: "{{ $v := .Variables }}{{ $v.TOKEN }}", key: "$v.TOKEN", remedy: "check aliases to identify the owning input"},
		{name: "supplied variable", command: "{{ .Variables.TOKEN }}", opts: lint.Options{DeploySetVariables: map[string]string{"TOKEN": "secret-supplied"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := chartSource()
			f.spec.Charts = nil
			f.spec.Actions.OnDeploy.Before = []v1beta1.ComponentAction{{Cmd: tc.command, EnableTemplating: true}}
			result := f.analyze(t, tc.opts)
			require.NoError(t, result.Err)
			if tc.key == "" {
				requireFindings(t, result, nil)
			} else {
				requireFindings(t, result, []findingContract{{"template-input-missing", definition, actionPath, lint.SevWarn, tc.remedy}}, actionPath+": missing input "+tc.key)
				require.Equal(t, 1, result.Findings[0].Line)
				require.Positive(t, result.Findings[0].Column)
				require.False(t, result.FailsAt(lint.SevErr))
				if tc.name != "value" {
					require.NotContains(t, result.Findings[0].Description, "--set-values")
				}
			}
		})
	}
}

func TestAnalyzeDefaultStateAccess(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, command, missing string
		access                 []v1beta1.StateAccessKey
	}{
		{name: "non-sensitive defaults", command: `{{ .State.Distro }}{{ .State.StorageClass }}{{ .State.IPFamily }}
{{ .State.Registry.Address }}{{ .State.Registry.Port }}{{ .State.Registry.PushUsername }}{{ .State.Registry.PullUsername }}
{{ .State.Registry.Mode }}{{ .State.Registry.MTLSEnabled }}{{ .State.Registry.SeedAddress }}
{{ .State.Git.Address }}{{ .State.Git.PushUsername }}{{ .State.Git.PullUsername }}{{ .State.Git.IsInternal }}
{{ .State.Injector.Image }}{{ .State.Injector.Port }}{{ .State.Injector.PayloadConfigMaps }}{{ .State.Injector.PayloadShaSum }}
{{ if or (empty .State.Registry.Address) (empty .State.Registry.PushUsername) (empty .State.Git.Address) (empty .State.Git.PullUsername) }}{{ fail "default state is not populated" }}{{ end }}`},
		{name: "registry gated", command: "{{ .State.Registry.PushPassword }}", missing: ".State.Registry.PushPassword"},
		{name: "git gated", command: "{{ .State.Git.PullPassword }}", missing: ".State.Git.PullPassword"},
		{name: "agent gated", command: "{{ .State.Agent.Key }}", missing: ".State.Agent.Key"},
		{name: "registry declared", command: "{{ .State.Registry.PushPassword }}{{ .State.Registry.PullPassword }}{{ .State.Registry.Secret }}{{ .State.Registry.Htpasswd }}", access: []v1beta1.StateAccessKey{v1beta1.StateAccessRegistryCredentials}},
		{name: "git declared", command: "{{ .State.Git.PushPassword }}{{ .State.Git.PullPassword }}", access: []v1beta1.StateAccessKey{v1beta1.StateAccessGitCredentials}},
		{name: "agent declared", command: "{{ .State.Agent.CA }}{{ .State.Agent.Cert }}{{ .State.Agent.Key }}", access: []v1beta1.StateAccessKey{v1beta1.StateAccessAgentCerts}},
		{name: "unrelated declaration stays gated", command: "{{ .State.Git.PushPassword }}", missing: ".State.Git.PushPassword", access: []v1beta1.StateAccessKey{v1beta1.StateAccessRegistryCredentials}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := sourceFixture{spec: v1beta1.ComponentSpec{StateAccess: tc.access, Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{
				Before: []v1beta1.ComponentAction{{Cmd: tc.command, EnableTemplating: true}},
			}}}}
			result := f.analyze(t, lint.Options{})
			require.NoError(t, result.Err)
			if tc.missing == "" {
				requireFindings(t, result, nil)
			} else {
				requireFindings(t, result, []findingContract{{"template-input-missing", definition, actionPath, lint.SevWarn, "component's declared stateAccess"}}, actionPath+": missing input "+tc.missing)
			}
		})
	}
}

func TestAnalyzeRemovalActionsAndWaits(t *testing.T) {
	t.Parallel()
	f := sourceFixture{spec: v1beta1.ComponentSpec{Actions: v1beta1.ComponentActions{OnRemove: v1beta1.ComponentActionSet{
		Before:    []v1beta1.ComponentAction{{Cmd: "{{", EnableTemplating: true}},
		OnSuccess: []v1beta1.ComponentAction{{EnableTemplating: true, Wait: &v1beta1.ComponentActionWait{Network: &v1beta1.ComponentActionWaitNetwork{Protocol: "tcp", Address: "{{"}}}},
		OnFailure: []v1beta1.ComponentAction{{EnableTemplating: true, Wait: &v1beta1.ComponentActionWait{Cluster: &v1beta1.ComponentActionWaitCluster{Kind: "Pod", Name: "{{"}}}},
	}}}}
	var want []findingContract
	var blocked []string
	for _, list := range []string{"before", "onSuccess", "onFailure"} {
		path := component + ".actions.onRemove." + list + ".[0]"
		want = append(want, findingContract{"template-syntax", definition, path, lint.SevErr, "invalid Go template:"})
		blocked = append(blocked, path+": template syntax is invalid")
	}
	result := f.analyze(t, lint.Options{})
	require.NoError(t, result.Err)
	requireFindings(t, result, want, blocked...)
}

func TestAnalyzeResourceTemplateFailures(t *testing.T) {
	t.Parallel()
	f := chartSource()
	f.spec.Files = []v1beta1.File{{Source: "settings.txt", Destination: "/tmp/settings", EnableTemplating: true}}
	f.spec.Manifests = []v1beta1.Manifest{{Name: "app", Files: []string{"manifest.yaml"}, EnableTemplating: true}}
	f.spec.Charts[0].ValuesFiles = []v1beta1.ValuesFile{{Path: "web-values.yaml", EnableTemplating: true}}
	f.files["settings.txt"], f.files["manifest.yaml"] = `{{ fail "secret-file" }}`, "{{"
	f.files["web-values.yaml"] = "hostname: {{ .Values.ingress.hostname }}\n"
	f.files["chart/values.schema.json"] = `{"type":"object","required":["hostname"],"properties":{"hostname":{"type":"string"}}}`
	result := f.analyze(t, lint.Options{})
	require.NoError(t, result.Err)
	requireFindings(t, result, []findingContract{
		{"template-render", "settings.txt", component + ".files.[0]", lint.SevErr, "template evaluation failed"},
		{"template-syntax", "manifest.yaml", component + ".manifests.[0]", lint.SevErr, "invalid Go template:"},
		{"template-input-missing", "web-values.yaml", chartPath + ".valuesFiles.[0]", lint.SevWarn, "--set-values ingress.hostname=<value>"},
	}, component+".files.[0]: template evaluation failed", component+".manifests.[0]: template syntax is invalid", chartPath+".valuesFiles.[0]: missing input ingress.hostname", chartBlocked)
}

func TestAnalyzeChartSchemaExemptions(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"exempt only", "checked sibling", "warning suppressed"} {
		t.Run(mode, func(t *testing.T) {
			f := chartSource()
			f.spec.Charts[0].SkipSchemaValidation = true
			f.files["chart/values.yaml"] = "replicas: invalid\n"
			f.files["chart/values.schema.json"] = `{"type":"object","properties":{"replicas":{"type":"integer"}}}`
			opts := lint.Options{}
			want := []findingContract{{"chart-schema-skipped", definition, chartPath + ".skipSchemaValidation", lint.SevWarn, "chart web disables schema validation"}}
			blocked := []string{chartPath + ": chart schema validation disabled by skipSchemaValidation"}
			if mode != "exempt only" {
				checked := f.spec.Charts[0]
				checked.Name, checked.SkipSchemaValidation = "checked", false
				f.spec.Charts = append(f.spec.Charts, checked)
				f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: ".missing", TargetPath: ".input"}}
				f.spec.Charts[0].ValuesFiles = []v1beta1.ValuesFile{{Path: "exempt-values.yaml", EnableTemplating: true}}
				f.files["exempt-values.yaml"] = `replicas: {{ fail "secret-exempt" }}`
				if mode == "warning suppressed" {
					opts.SkipRules = []string{"chart-schema-skipped"}
					want = nil
				}
				want = append(want,
					findingContract{"template-input-missing", definition, chartPath + ".values.[0]", lint.SevWarn, "--set-values missing=<value>"},
					findingContract{"template-render", "exempt-values.yaml", chartPath + ".valuesFiles.[0]", lint.SevErr, "template evaluation failed"},
					findingContract{lint.RuleChartValuesSchema, definition, component + ".charts.[1]", lint.SevErr, "replicas"})
				blocked = append(blocked, chartPath+".values.[0]: missing input missing", chartPath+".valuesFiles.[0]: template evaluation failed")
			}
			result := f.analyze(t, opts)
			require.NoError(t, result.Err)
			requireFindings(t, result, want, blocked...)
			require.Equal(t, opts.SkipRules, result.SkippedRules)
			require.Equal(t, mode != "exempt only", result.FailsAt(lint.SevErr))
		})
	}
}

// Source lint must validate the effective dependency values: defaults can satisfy
// a required field, while a parent override can make that same field invalid.
func TestAnalyzeChartSchemaIncludesDependencyDefaults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		invalid bool
	}{{"dependency defaults satisfy required field", false}, {"parent override violates dependency schema", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := chartSource()
			f.spec.Charts[0].ValuesFiles = []v1beta1.ValuesFile{{Path: "parent-values.yaml"}}
			f.files["parent-values.yaml"] = "{}\n"
			if tc.invalid {
				f.files["parent-values.yaml"] = "worker:\n  replicas: wrong\n"
			}
			f.files["chart/Chart.yaml"] = "apiVersion: v2\nname: parent\nversion: 0.1.0\ndependencies:\n  - name: worker\n    version: 0.1.0\n    repository: file://charts/worker\n"
			f.files["chart/charts/worker/Chart.yaml"] = "apiVersion: v2\nname: worker\nversion: 0.1.0\n"
			f.files["chart/charts/worker/values.yaml"] = "replicas: 2\n"
			f.files["chart/charts/worker/values.schema.json"] = `{"type":"object","required":["replicas"],"properties":{"replicas":{"type":"integer"}}}`
			result := f.analyze(t, lint.Options{})
			require.NoError(t, result.Err)
			if tc.invalid {
				requireFindings(t, result, []findingContract{{lint.RuleChartValuesSchema, definition, chartPath, lint.SevErr, "replicas"}})
				require.Contains(t, result.Findings[0].Description, "worker")
			} else {
				requireFindings(t, result, nil)
			}
			require.Equal(t, tc.invalid, result.FailsAt(lint.SevErr))
		})
	}
}

func TestAnalyzeBlockedMappings(t *testing.T) {
	t.Parallel()
	for _, mapping := range []bool{false, true} {
		t.Run(fmt.Sprintf("mapping=%t", mapping), func(t *testing.T) {
			f := chartSource()
			consumer := chartPath + ".valuesFiles.[0]"
			if mapping {
				f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: ".hostname", TargetPath: ".hostname"}}
				consumer = chartPath + ".values.[0]"
			} else {
				f.spec.Charts[0].ValuesFiles = []v1beta1.ValuesFile{{Path: "web-values.yaml", EnableTemplating: true}}
				f.files["web-values.yaml"] = "hostname: {{ .Values.hostname }}\n"
			}
			f.files["chart/values.schema.json"] = `{"type":"object","required":["hostname"],"properties":{"hostname":{"type":"string"}}}`
			opts := lint.Options{SkipRules: []string{"template-input-missing", "template-render"}}
			result := f.analyze(t, opts)
			require.NoError(t, result.Err)
			requireFindings(t, result, nil, consumer+": missing input hostname", chartBlocked)
			require.Equal(t, opts.SkipRules, result.SkippedRules)
			require.False(t, result.FailsAt(lint.SevErr))
		})
	}
}

func TestAnalyzeInvalidMapping(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source, target, message string
		skips                         []string
	}{
		{name: "target", source: ".input", target: "invalid", message: `targetPath "invalid" must start with a dot`},
		{name: "source with mapping rule skipped", source: "input", target: ".target", message: "invalid path format: input", skips: []string{lint.RuleValuesMapping}},
		{name: "target with mapping rule skipped", source: ".input", target: "invalid", message: `targetPath "invalid" must start with a dot`, skips: []string{lint.RuleValuesMapping}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := chartSource()
			f.values.Files = []string{"defaults.yaml"}
			f.files["defaults.yaml"] = "input: valid\n"
			f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: tc.source, TargetPath: tc.target}}
			result := f.analyze(t, lint.Options{SkipRules: tc.skips})
			rule, path := lint.RuleValuesMapping, chartPath+".values"
			if len(tc.skips) > 0 {
				rule = lint.RuleChartValuesSchema
			}
			if tc.source == "input" {
				path += ".[0]"
			}
			require.NoError(t, result.Err)
			requireFindings(t, result, []findingContract{{rule, definition, path, lint.SevErr, tc.message}}, chartBlocked)
			require.Equal(t, tc.skips, result.SkippedRules)
			require.True(t, result.FailsAt(lint.SevErr), "skipping mapping diagnostics must not waive an enabled chart check's prerequisites")
		})
	}
}

func TestAnalyzeChartMappingsDoNotChangeLaterInputs(t *testing.T) {
	t.Parallel()
	f := chartSource()
	f.values.Files = []string{"defaults.yaml"}
	f.files["defaults.yaml"] = "first:\n  keep: 1\nsecond:\n  other: 2\n"
	f.spec.Charts[0].Values = []v1beta1.ChartValue{{SourcePath: ".first", TargetPath: ".joined"}, {SourcePath: ".second", TargetPath: ".joined"}}
	f.spec.Charts = append(f.spec.Charts, v1beta1.Chart{Name: "original", Namespace: "default", Local: &v1beta1.LocalSource{Path: "other-chart"}, ValuesFiles: []v1beta1.ValuesFile{{Path: "original-values.yaml", EnableTemplating: true}}})
	f.files["original-values.yaml"] = "first: {{ .Values.first | toJson }}\n"
	f.files["other-chart/Chart.yaml"], f.files["other-chart/values.yaml"] = "apiVersion: v2\nname: original\nversion: 0.1.0\n", "{}\n"
	f.files["other-chart/values.schema.json"] = `{"type":"object","properties":{"first":{"type":"object","additionalProperties":false,"properties":{"keep":{"const":1}}}}}`
	result := f.analyze(t, lint.Options{})
	require.NoError(t, result.Err)
	requireFindings(t, result, nil)
}
