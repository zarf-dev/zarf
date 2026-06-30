// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"sort"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// ByVariable creates a filter that keeps components whose Only.Variable map fully matches
// the supplied resolved variable/constant values. Missing keys are treated as empty strings.
func ByVariable(resolved map[string]string) ComponentFilterStrategy {
	return &variableFilter{resolved: resolved}
}

type variableFilter struct {
	resolved map[string]string
}

// Apply applies the filter.
func (f *variableFilter) Apply(pkg v1alpha1.ZarfPackage) ([]v1alpha1.ZarfComponent, error) {
	filtered := []v1alpha1.ZarfComponent{}
	for _, component := range pkg.Components {
		if matchesVariables(component.Only.Variable, f.resolved) {
			filtered = append(filtered, component)
		}
	}
	return filtered, nil
}

func matchesVariables(required map[string]string, resolved map[string]string) bool {
	for name, want := range required {
		if resolved[name] != want {
			return false
		}
	}
	return true
}

// CheckVariableFilterDropsRequested returns an error if a component that was explicitly
// requested via --components was dropped by the only.variable filter, so the user gets a
// clear "filtered by only.variable" message instead of a downstream "no compatible components"
// error from ForDeploy. Components matched only by a default-group fallback or a glob are not
// considered "explicitly requested" here.
func CheckVariableFilterDropsRequested(before, after []v1alpha1.ZarfComponent, optionalComponents string, resolved map[string]string) error {
	requested := helpers.StringToSlice(optionalComponents)
	if len(requested) == 0 || requested[0] == "" {
		return nil
	}
	survivors := map[string]struct{}{}
	for _, c := range after {
		survivors[c.Name] = struct{}{}
	}
	var dropped []string
	for _, c := range before {
		if _, ok := survivors[c.Name]; ok {
			continue
		}
		state, _, err := includedOrExcluded(c.Name, requested)
		if err != nil {
			return err
		}
		if state != included {
			continue
		}
		mismatches := make([]string, 0, len(c.Only.Variable))
		for k, want := range c.Only.Variable {
			if got := resolved[k]; got != want {
				mismatches = append(mismatches, fmt.Sprintf("%s=%q (want %q)", k, got, want))
			}
		}
		sort.Strings(mismatches)
		dropped = append(dropped, fmt.Sprintf("%q filtered by only.variable: %s", c.Name, strings.Join(mismatches, ", ")))
	}
	if len(dropped) == 0 {
		return nil
	}
	sort.Strings(dropped)
	return fmt.Errorf("requested component(s) excluded by only.variable: %s", strings.Join(dropped, "; "))
}
