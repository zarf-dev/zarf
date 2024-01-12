// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/types"
)

// ComponentFilterStrategy is a strategy interface for filtering components.
type ComponentFilterStrategy interface {
	Apply([]types.ZarfComponent) ([]types.ZarfComponent, error)
}

// VersionBehavior is an interface for setting version behavior on filters
type VersionBehavior interface {
	UseVersionBehavior(*semver.Version)
}

// NewFilterManager creates a new filter manager for the given strategy.
func NewFilterManager(strategy ComponentFilterStrategy) *FilterManager {
	m := &FilterManager{}
	m.SetStrategy(strategy)
	return m
}

// FilterManager manages a filter strategy.
type FilterManager struct {
	strategy ComponentFilterStrategy
}

// SetStrategy sets the strategy for the filter manager.
func (m *FilterManager) SetStrategy(strategy ComponentFilterStrategy) {
	m.strategy = strategy
}

// SetVersionBehavior sets the version behavior for the filter strategy.
func (m *FilterManager) SetVersionBehavior(buildVersion *semver.Version) {
	if v, ok := m.strategy.(VersionBehavior); ok {
		v.UseVersionBehavior(buildVersion)
	}
}

// Execute executes the filter strategy.
func (m *FilterManager) Execute(components []types.ZarfComponent) ([]types.ZarfComponent, error) {
	return m.strategy.Apply(components)
}
