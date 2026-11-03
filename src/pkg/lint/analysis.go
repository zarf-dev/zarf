// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package lint

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	packagertemplate "github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"
	chartutil "helm.sh/helm/v4/pkg/chart/common/util"
	chartv2util "helm.sh/helm/v4/pkg/chart/v2/util"
)

// analysis owns one source-only analysis. All writes are confined to staging;
// the resolved definition, defaults, and source resources remain unchanged.
type analysis struct {
	ctx       context.Context
	loaded    *load.ResolvedPackage
	vals      value.Values
	vc        *variables.VariableConfig
	objects   template.Objects
	opts      Options
	result    LintError
	state     *state.State
	source    string
	staging   string
	consumers [][]string
	mappings  []api.ChartValue
	exposes   bool
}

// Options configures source linting, including definition loading and analysis.
type Options struct {
	CreateSetVariables map[string]string
	Flavor             string
	CachePath          string
	Values             value.Values
	DeploySetVariables map[string]string
	SkipRules          []string
	types.RemoteOptions
}

// Analyze checks a resolved source definition without executing actions. The caller
// retains ownership of loaded and closes it after analysis. Results include fatal
// input/resource errors as well as findings from independent checks.
func Analyze(ctx context.Context, loaded *load.ResolvedPackage, source string, componentConfig bool, opts Options) *LintError {
	a := &analysis{
		ctx: ctx, loaded: loaded, opts: opts, source: source,
		result:  LintError{PackageName: loaded.Definition.Metadata.Name, SkippedRules: slices.Clone(opts.SkipRules)},
		exposes: len(loaded.Values) > 0,
	}
	for _, finding := range CheckPrimitives(loaded.Definition, componentConfig) {
		a.add(finding)
	}
	a.vals = loaded.Values.DeepCopy()
	a.vals.DeepMerge(opts.Values)
	if loaded.ValuesSchema != nil {
		if err := a.vals.ValidateAgainstSchema(ctx, loaded.ValuesSchema, "resolved values schema", value.ValidateOptions{SkipRequired: true}); err != nil {
			a.result.Err = err
			return &a.result
		}
	}
	if !a.enabled(RuleValuesSchemaMissing) && !a.enabled(RuleValuesUnused) &&
		!a.enabled(RuleTemplateSyntax) && !a.enabled(RuleTemplateRender) &&
		!a.enabled(RuleTemplateInputMissing) && !a.enabled(RuleValuesMapping) &&
		!a.enabled(RuleChartValuesSchema) && !a.enabled(RuleChartSchemaSkipped) {
		return &a.result
	}
	vc := packagertemplate.GetZarfVariableConfig(ctx, false)
	vc.SetConstants(loaded.Definition.Constants)
	if err := vc.PopulateVariables(loaded.Definition.Variables, opts.DeploySetVariables); err != nil {
		a.result.Err = err
		return &a.result
	}
	a.vc = vc
	var err error
	a.state, err = state.Default()
	if err != nil {
		a.result.Err = err
		return &a.result
	}
	a.staging, err = utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		a.result.Err = err
		return &a.result
	}
	for i, component := range loaded.Definition.Components {
		prefix := fmt.Sprintf(".components.[%d]", i)
		if componentConfig {
			prefix = ".component"
		}
		a.component(component, prefix, filepath.Join(a.staging, fmt.Sprint(i)))
	}
	if a.exposes && loaded.ValuesSchema == nil {
		a.add(PackageFinding{RuleID: RuleValuesSchemaMissing, Severity: SevWarn, YqPath: ".values", Description: "define a values schema to catch invalid package configuration before deployment"})
	}
	if a.enabled(RuleValuesUnused) {
		a.unused(map[string]any(loaded.Values), nil)
	}
	a.result.Err = errors.Join(a.result.Err, os.RemoveAll(a.staging))
	return &a.result
}

func (a *analysis) enabled(rule string) bool {
	return !slices.Contains(a.opts.SkipRules, rule)
}

func (a *analysis) add(finding PackageFinding) *PackageFinding {
	if a.enabled(finding.RuleID) {
		if finding.Source == "" {
			finding.Source = a.source
		}
		a.result.Findings = append(a.result.Findings, finding)
		return &a.result.Findings[len(a.result.Findings)-1]
	}
	return nil
}

func (a *analysis) blocked(location, cause string) {
	entry := location + ": " + cause
	if !slices.Contains(a.result.NotEvaluated, entry) {
		a.result.NotEvaluated = append(a.result.NotEvaluated, entry)
	}
}

func (a *analysis) component(component api.Component, prefix, dir string) {
	applicationTemplates, err := packagertemplate.GetZarfTemplates(a.ctx, a.loaded.Definition.GetAPIVersion(), component.Service.Name, a.state)
	if err != nil {
		a.resourceError(prefix, err)
		return
	}
	a.vc.SetApplicationTemplates(applicationTemplates)
	a.objects, err = template.NewObjects(a.vals).WithPackage(a.loaded.Definition).
		WithVariables(a.vc.GetSetVariableMap()).WithConstants(a.vc.GetConstants()).
		WithState(template.StateAccess{State: a.state, AccessKeys: component.StateAccess})
	if err != nil {
		a.resourceError(prefix, err)
		return
	}
	for _, phase := range []struct {
		name string
		set  api.ActionSet
	}{{"onDeploy", component.Actions.OnDeploy}, {"onRemove", component.Actions.OnRemove}} {
		for _, list := range []struct {
			name    string
			actions []api.Action
		}{{"before", phase.set.Before}, {"after", phase.set.After}, {"onSuccess", phase.set.OnSuccess}, {"onFailure", phase.set.OnFailure}} {
			for i, action := range list.actions {
				if !action.EnableTemplating || !a.needsTemplates() {
					continue
				}
				path := fmt.Sprintf("%s.actions.%s.%s.[%d]", prefix, phase.name, list.name, i)
				for _, content := range action.TemplateStrings() {
					a.checkTemplate(content, a.source, path, false)
				}
			}
		}
	}
	for i, manifest := range component.Manifests {
		if !manifest.EnableTemplating || !a.needsTemplates() {
			continue
		}
		path := fmt.Sprintf("%s.manifests.[%d]", prefix, i)
		manifestDir := filepath.Join(dir, "manifests", fmt.Sprint(i))
		if err := assemble.PackageManifest(a.ctx, manifest, manifestDir, a.loaded.Resources); err != nil {
			a.resourceError(path, err)
			continue
		}
		for fileIdx, source := range manifest.Files {
			a.walkTemplates(filepath.Join(manifestDir, string(layout.ManifestsComponentDir), layout.ManifestFileName(manifest.Name, fileIdx)), path, source)
		}
		for kustomizeIdx, source := range manifest.Kustomize.Files {
			a.walkTemplates(filepath.Join(manifestDir, string(layout.ManifestsComponentDir), layout.KustomizationFileName(manifest.Name, kustomizeIdx)), path, source)
		}
	}
	for i, file := range component.Files {
		if !file.EnableTemplating || !a.needsTemplates() {
			continue
		}
		path := fmt.Sprintf("%s.files.[%d]", prefix, i)
		dst := filepath.Join(dir, "files", fmt.Sprint(i), "content")
		if err := assemble.PackageFile(a.ctx, file, dst, a.loaded.Resources); err != nil {
			a.resourceError(path, err)
			continue
		}
		a.walkTemplates(dst, path, file.Source)
	}
	for i, chart := range component.Charts {
		a.chart(chart, fmt.Sprintf("%s.charts.[%d]", prefix, i), filepath.Join(dir, "charts", fmt.Sprint(i)))
	}
}

func (a *analysis) needsTemplates() bool {
	return a.enabled(RuleTemplateSyntax) || a.enabled(RuleTemplateRender) ||
		a.enabled(RuleTemplateInputMissing) || a.enabled(RuleValuesUnused) || a.enabled(RuleValuesSchemaMissing)
}

func (a *analysis) resourceError(path string, err error) {
	a.result.Err = errors.Join(a.result.Err, fmt.Errorf("%s: %w", path, err))
	a.blocked(path, "resource unavailable")
}

func (a *analysis) walkTemplates(root, path, source string) {
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if a.loaded.Definition.GetAPIVersion() == v1alpha1.APIVersion {
			// Acquisition materializes this file in staging; never rewrite authored sources.
			if err := a.vc.ReplaceTextTemplate(file); err != nil {
				return err
			}
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		location := source
		if rel != "." {
			location = filepath.Join(source, rel)
		}
		a.checkTemplate(string(content), location, path, false)
		return nil
	})
	if err != nil {
		a.resourceError(path, err)
	}
}

// checkTemplate parses every branch for consumption, but executes only to check
// rendering. No commands are executed and rendered action text is discarded.
func (a *analysis) checkTemplate(content, source, path string, required bool) (string, bool) {
	parsed, err := template.Parse(content)
	if err != nil {
		// Unknown consumers must not turn a syntax problem into unused-key noise.
		a.consumers = append(a.consumers, nil)
		rule := RuleTemplateSyntax
		if !a.enabled(rule) && (a.enabled(RuleTemplateRender) || required) {
			rule = RuleTemplateRender
			if !a.enabled(rule) {
				rule = RuleChartValuesSchema
			}
		}
		a.add(PackageFinding{RuleID: rule, Severity: SevErr, Source: source, YqPath: path, Description: "invalid Go template: " + err.Error()})
		a.blocked(path, "template syntax is invalid")
		return "", false
	}
	if a.enabled(RuleValuesUnused) || a.enabled(RuleValuesSchemaMissing) {
		consumers := parsed.ValueConsumers()
		a.consumers = append(a.consumers, consumers...)
		a.exposes = a.exposes || len(consumers) > 0
	}
	if !required && !a.enabled(RuleTemplateRender) && !a.enabled(RuleTemplateInputMissing) {
		return content, true
	}
	rendered, err := parsed.Apply(a.ctx, a.objects)
	if err == nil {
		return rendered, true
	}
	var diagnostic *template.Error
	if errors.As(err, &diagnostic) && diagnostic.MissingInput != "" {
		if finding := a.missingInput(source, path, diagnostic.MissingInput); finding != nil {
			finding.Line, finding.Column = diagnostic.Line, diagnostic.Column
		}
		return "", false
	}
	rule := RuleTemplateRender
	if !a.enabled(rule) && required {
		rule = RuleChartValuesSchema
	}
	// Evaluation errors can contain the value passed to Sprig's fail function.
	// Report the source location, not potentially sensitive evaluated arguments.
	a.add(PackageFinding{RuleID: rule, Severity: SevErr, Source: source, YqPath: path, Description: "template evaluation failed; check function arguments and value types at this location"})
	a.blocked(path, "template evaluation failed")
	return "", false
}

func (a *analysis) missingInput(source, path, key string) *PackageFinding {
	var remedy string
	switch {
	case strings.HasPrefix(key, ".Variables."):
		remedy = "supply --deploy-set-variables " + strings.TrimPrefix(key, ".Variables.") + "=<value>"
	case strings.HasPrefix(key, ".Constants."):
		remedy = "define package constant " + strings.TrimPrefix(key, ".Constants.")
	case strings.HasPrefix(key, ".State"):
		remedy = "check this field against Zarf's default state and the component's declared stateAccess"
	case strings.HasPrefix(key, ".Values."):
		key = strings.TrimPrefix(key, ".Values.")
		remedy = "define it in package defaults, pass --values <file> containing it, or pass --set-values " + key + "=<value>"
	default:
		remedy = "provide this key in the template object used at this location; check aliases to identify the owning input"
	}
	a.blocked(path, "missing input "+key)
	return a.add(PackageFinding{RuleID: RuleTemplateInputMissing, Severity: SevWarn, Source: source, YqPath: path, Description: "missing input " + key + "; " + remedy})
}

func (a *analysis) chart(chart api.Chart, path, dir string) {
	a.exposes = a.exposes || len(chart.Values) > 0
	a.mappings = append(a.mappings, chart.Values...)
	validate := a.enabled(RuleChartValuesSchema) && !chart.SkipSchemaValidation
	if chart.SkipSchemaValidation {
		a.add(PackageFinding{RuleID: RuleChartSchemaSkipped, Severity: SevWarn, YqPath: path + ".skipSchemaValidation", Description: "chart " + chart.Name + " disables schema validation"})
		a.blocked(path, "chart schema validation disabled by skipSchemaValidation")
	}
	checkMapping := a.enabled(RuleValuesMapping) || validate
	mappingOK := true
	for i, mapping := range chart.Values {
		if !checkMapping {
			break
		}
		mappingPath := fmt.Sprintf("%s.values.[%d]", path, i)
		if err := value.Path(mapping.SourcePath).Validate(); err != nil {
			rule := RuleValuesMapping
			if !a.enabled(rule) && validate {
				rule = RuleChartValuesSchema
			}
			a.add(PackageFinding{RuleID: rule, Severity: SevErr, YqPath: mappingPath, Description: err.Error()})
			mappingOK = false
			continue
		}
		if _, err := a.vals.Extract(value.Path(mapping.SourcePath)); err != nil {
			if strings.Contains(err.Error(), "not found in path") {
				a.missingInput(a.source, mappingPath, ".Values"+mapping.SourcePath)
				mappingOK = false
				continue
			}
		}
	}
	var overrides map[string]any
	if checkMapping && mappingOK {
		var err error
		// Mapping multiple objects into one target can merge their backing maps.
		// Keep that chart-local work isolated from subsequent source checks.
		overrides, err = helm.GenerateChartOverrides(chart, a.vc, a.vals.DeepCopy(), nil)
		if err != nil {
			rule := RuleValuesMapping
			if !a.enabled(rule) {
				rule = RuleChartValuesSchema
			}
			a.add(PackageFinding{RuleID: rule, Severity: SevErr, YqPath: path + ".values", Description: err.Error()})
			mappingOK = false
		}
	}
	needsFiles := slices.ContainsFunc(chart.ValuesFiles, func(f api.ValuesFile) bool { return f.EnableTemplating }) && a.needsTemplates()
	if !validate && !needsFiles {
		return
	}
	paths := layout.ChartPaths{ChartsDir: filepath.Join(dir, "charts"), ValuesDir: filepath.Join(dir, "values")}
	if validate {
		if err := assemble.PackageChart(a.ctx, chart, a.loaded.Resources, paths, a.opts.CachePath, a.opts.RemoteOptions); err != nil {
			a.resourceError(path, err)
			return
		}
	}
	templatesOK := true
	for i, file := range chart.ValuesFiles {
		if !validate && !file.EnableTemplating {
			continue
		}
		physical, err := paths.ValuesFilePath(chart.Name, chart.LegacyVersion, i)
		if err != nil {
			a.resourceError(path, err)
			templatesOK = false
			continue
		}
		if !validate {
			if err := assemble.PackageFile(a.ctx, api.File{Source: file.Path}, physical, a.loaded.Resources); err != nil {
				a.resourceError(path, err)
				templatesOK = false
				continue
			}
		}
		if a.loaded.Definition.GetAPIVersion() == v1alpha1.APIVersion {
			if err := a.vc.ReplaceTextTemplate(physical); err != nil {
				a.resourceError(path, err)
				templatesOK = false
				continue
			}
		}
		if !file.EnableTemplating {
			continue
		}
		content, err := os.ReadFile(physical)
		if err != nil {
			a.resourceError(path, err)
			templatesOK = false
			continue
		}
		rendered, ok := a.checkTemplate(string(content), file.Path, fmt.Sprintf("%s.valuesFiles.[%d]", path, i), validate)
		if !ok {
			templatesOK = false
			continue
		}
		if validate {
			if err := os.WriteFile(physical, []byte(rendered), 0o600); err != nil {
				a.resourceError(path, err)
				templatesOK = false
			}
		}
	}
	if !validate || !mappingOK || !templatesOK {
		if validate && (!mappingOK || !templatesOK) {
			a.blocked(path, "chart schema validation requires resolved mappings and rendered values files")
		}
		return
	}
	loadedChart, vals, err := helm.LoadChartData(chart, paths, overrides)
	if err == nil {
		err = chartv2util.ProcessDependencies(loadedChart, vals)
	}
	if err != nil {
		a.resourceError(path, err)
		return
	}
	coalesced, err := chartutil.CoalesceValues(loadedChart, vals)
	if err == nil {
		err = chartutil.ValidateAgainstSchema(loadedChart, coalesced)
	}
	if err != nil {
		a.add(PackageFinding{RuleID: RuleChartValuesSchema, Severity: SevErr, YqPath: path, Description: "chart " + chart.Name + ": " + err.Error()})
	}
}

func (a *analysis) unused(values map[string]any, prefix []string) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		path := append(slices.Clone(prefix), key)
		if nested, ok := values[key].(map[string]any); ok && len(nested) > 0 {
			a.unused(nested, path)
			continue
		}
		used := false
		for _, consumer := range a.consumers {
			if hasPrefix(path, consumer) || hasPrefix(consumer, path) {
				used = true
				break
			}
		}
		for _, mapping := range a.mappings {
			if !hasPrefix(path, value.Path(mapping.SourcePath).Segments()) {
				continue
			}
			excluded := false
			for _, exclude := range mapping.ExcludePaths {
				if hasPrefix(path, value.Path(exclude).Segments()) {
					excluded = true
					break
				}
			}
			used = used || !excluded
		}
		if !used {
			a.add(PackageFinding{RuleID: RuleValuesUnused, Severity: SevWarn, YqPath: ".values." + strings.Join(path, "."), Description: "potentially unused default value; no static mapping or template consumer"})
		}
	}
}

func hasPrefix(path, prefix []string) bool {
	return len(path) >= len(prefix) && slices.Equal(path[:len(prefix)], prefix)
}
