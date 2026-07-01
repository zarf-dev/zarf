// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import "github.com/zarf-dev/zarf/src/internal/api/types"

// SetDeprecatedFromGeneric populates the unexported v1alpha1 backwards-compatibility
// shim fields on an already-converted package.
// This is intentionally not reachable outside Zarf as types.Package is internal.
func SetDeprecatedFromGeneric(g types.Package, pkg Package) Package {
	pkg.variables = interactiveVarsFromGeneric(g.Variables)
	pkg.constants = constantsFromGeneric(g.Constants)
	pkg.Metadata.yolo = g.Metadata.YOLO

	for i := range pkg.Components {
		gc := g.Components[i]
		pkg.Components[i].dataInjections = dataInjectionsFromGeneric(gc.DataInjections)
		pkg.Components[i].group = gc.Group

		for j := range pkg.Components[i].Charts {
			gch := gc.Charts[j]
			pkg.Components[i].Charts[j].version = gch.Version
			pkg.Components[i].Charts[j].variables = chartVarsFromGeneric(gch.Variables)
		}

		applyActionSetSetVariables(&pkg.Components[i].Actions.OnCreate, gc.Actions.OnCreate)
		applyActionSetSetVariables(&pkg.Components[i].Actions.OnDeploy, gc.Actions.OnDeploy)
		applyActionSetSetVariables(&pkg.Components[i].Actions.OnRemove, gc.Actions.OnRemove)
	}

	return pkg
}

func applyActionSetSetVariables(set *ComponentActionSet, g types.ComponentActionSet) {
	applyActionSliceSetVariables(set.Before, g.Before)
	// set.OnSuccess is [After..., OnSuccess...] after the fold in actionSetFromGeneric.
	afterLen := len(g.After)
	applyActionSliceSetVariables(set.OnSuccess[:afterLen], g.After)
	applyActionSliceSetVariables(set.OnSuccess[afterLen:], g.OnSuccess)
	applyActionSliceSetVariables(set.OnFailure, g.OnFailure)
}

func applyActionSliceSetVariables(actions []ComponentAction, g []types.ComponentAction) {
	for k := range g {
		setVars := setVarsFromGeneric(g[k].SetVariables)
		if g[k].DeprecatedSetVariable != "" {
			setVars = append(setVars, Variable{Name: g[k].DeprecatedSetVariable})
		}
		if len(setVars) > 0 {
			actions[k].setVariables = setVars
		}
	}
}

func variableFromGeneric(v types.Variable) Variable {
	return Variable{
		Name:       v.Name,
		Sensitive:  v.Sensitive,
		AutoIndent: v.AutoIndent,
		Pattern:    v.Pattern,
		Type:       VariableType(v.Type),
	}
}

func setVarsFromGeneric(in []types.Variable) []Variable {
	var out []Variable
	for _, v := range in {
		out = append(out, variableFromGeneric(v))
	}
	return out
}

func interactiveVarsFromGeneric(in []types.InteractiveVariable) []InteractiveVariable {
	var out []InteractiveVariable
	for _, v := range in {
		out = append(out, InteractiveVariable{
			Variable:    variableFromGeneric(v.Variable),
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}
	return out
}

func constantsFromGeneric(in []types.Constant) []Constant {
	var out []Constant
	for _, c := range in {
		out = append(out, Constant{
			Name:        c.Name,
			Value:       c.Value,
			Description: c.Description,
			AutoIndent:  c.AutoIndent,
			Pattern:     c.Pattern,
		})
	}
	return out
}

func chartVarsFromGeneric(in []types.ZarfChartVariable) []ZarfChartVariable {
	var out []ZarfChartVariable
	for _, v := range in {
		out = append(out, ZarfChartVariable{Name: v.Name, Description: v.Description, Path: v.Path})
	}
	return out
}

func dataInjectionsFromGeneric(in []types.ZarfDataInjection) []ZarfDataInjection {
	var out []ZarfDataInjection
	for _, d := range in {
		out = append(out, ZarfDataInjection{
			Source: d.Source,
			Target: ZarfContainerTarget{
				Namespace: d.Target.Namespace,
				Selector:  d.Target.Selector,
				Container: d.Target.Container,
				Path:      d.Target.Path,
			},
			Compress: d.Compress,
		})
	}
	return out
}
