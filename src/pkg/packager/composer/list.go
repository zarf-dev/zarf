// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// Node is a node in the import chain
type Node struct {
	types.ZarfComponent

	index int

	vars   []types.ZarfPackageVariable
	consts []types.ZarfPackageConstant

	relativeToHead      string
	originalPackageName string

	prev *Node
	next *Node
}

// GetIndex returns the .components index location for this node's source `zarf.yaml`
func (n *Node) GetIndex() int {
	return n.index
}

// GetOriginalPackageName returns the .metadata.name of the zarf package the component originated from
func (n *Node) GetOriginalPackageName() string {
	return n.originalPackageName
}

// GetRelativeToHead gets the path from downstream zarf file to upstream imported zarf file
func (n *Node) GetRelativeToHead() string {
	return n.relativeToHead
}

// Next returns next node in the chain
func (n *Node) Next() *Node {
	return n.next
}

// Prev returns previous node in the chain
func (n *Node) Prev() *Node {
	return n.prev
}

// ImportName returns the name of the component to import
// If the component import has a ComponentName defined, that will be used
// otherwise the name of the component will be used
func (n *Node) ImportName() string {
	name := n.ZarfComponent.Name
	if n.Import.ComponentName != "" {
		name = n.Import.ComponentName
	}
	return name
}

// ImportChain is a doubly linked list of component import definitions
type ImportChain struct {
	head *Node
	tail *Node

	remote *oci.OrasRemote
}

// Head returns the first node in the import chain
func (ic *ImportChain) Head() *Node {
	return ic.head
}

// Tail returns the last node in the import chain
func (ic *ImportChain) Tail() *Node {
	return ic.tail
}

func (ic *ImportChain) append(c types.ZarfComponent, index int, originalPackageName string,
	relativeToHead string, vars []types.ZarfPackageVariable, consts []types.ZarfPackageConstant) {
	node := &Node{
		ZarfComponent:       c,
		index:               index,
		originalPackageName: originalPackageName,
		relativeToHead:      relativeToHead,
		vars:                vars,
		consts:              consts,
		prev:                nil,
		next:                nil,
	}
	if ic.head == nil {
		ic.head = node
		ic.tail = node
	} else {
		p := ic.tail
		node.prev = p
		p.next = node
		ic.tail = node
	}
}

// NewImportChain creates a new import chain from a component
// Returning the chain on error so we can have additional information to use during lint
func NewImportChain(head types.ZarfComponent, index int, originalPackageName, arch, flavor string) (*ImportChain, error) {
	ic := &ImportChain{}
	if arch == "" {
		return ic, fmt.Errorf("cannot build import chain: architecture must be provided")
	}

	ic.append(head, index, originalPackageName, ".", nil, nil)

	history := []string{}

	node := ic.head
	for node != nil {
		isLocal := node.Import.Path != ""
		isRemote := node.Import.URL != ""

		if !isLocal && !isRemote {
			// This is the end of the import chain,
			// as the current node/component is not importing anything
			return ic, nil
		}

		// TODO: stuff like this should also happen in linting
		if err := validate.ImportDefinition(&node.ZarfComponent); err != nil {
			return ic, err
		}

		// ensure that remote components are not importing other remote components
		if node.prev != nil && node.prev.Import.URL != "" && isRemote {
			return ic, fmt.Errorf("detected malformed import chain, cannot import remote components from remote components")
		}
		// ensure that remote components are not importing local components
		if node.prev != nil && node.prev.Import.URL != "" && isLocal {
			return ic, fmt.Errorf("detected malformed import chain, cannot import local components from remote components")
		}

		var pkg types.ZarfPackage

		var relativeToHead string
		if isLocal {
			history = append(history, node.Import.Path)
			relativeToHead = filepath.Join(history...)

			// prevent circular imports (including self-imports)
			// this is O(n^2) but the import chain should be small
			prev := node
			for prev != nil {
				if prev.relativeToHead == relativeToHead {
					return ic, fmt.Errorf("detected circular import chain: %s", strings.Join(history, " -> "))
				}
				prev = prev.prev
			}

			// this assumes the composed package is following the zarf layout
			if err := utils.ReadYaml(filepath.Join(relativeToHead, layout.ZarfYAML), &pkg); err != nil {
				return ic, err
			}
		} else if isRemote {
			relativeToHead = node.Import.URL
			remote, err := ic.getRemote(node.Import.URL)
			if err != nil {
				return ic, err
			}
			pkg, err = remote.FetchZarfYAML()
			if err != nil {
				return ic, err
			}
		}

		name := node.ImportName()

		found := []types.ZarfComponent{}
		index := []int{}
		for i, component := range pkg.Components {
			if component.Name == name && CompatibleComponent(component, arch, flavor) {
				found = append(found, component)
				index = append(index, i)
			}
		}

		if len(found) == 0 {
			return ic, fmt.Errorf("component %q not found in %q", name, relativeToHead)
		} else if len(found) > 1 {
			return ic, fmt.Errorf("multiple components named %q found in %q satisfying %q", name, relativeToHead, arch)
		}

		ic.append(found[0], index[0], pkg.Metadata.Name, relativeToHead, pkg.Variables, pkg.Constants)
		node = node.next
	}
	return ic, nil
}

// String returns a string representation of the import chain
func (ic *ImportChain) String() string {
	if ic.head.next == nil {
		return fmt.Sprintf("component %q imports nothing", ic.head.Name)
	}

	s := strings.Builder{}

	name := ic.head.ImportName()

	if ic.head.Import.Path != "" {
		s.WriteString(fmt.Sprintf("component %q imports %q in %s", ic.head.Name, name, ic.head.Import.Path))
	} else {
		s.WriteString(fmt.Sprintf("component %q imports %q in %s", ic.head.Name, name, ic.head.Import.URL))
	}

	node := ic.head.next
	for node != ic.tail {
		name := node.ImportName()
		s.WriteString(", which imports ")
		if node.Import.Path != "" {
			s.WriteString(fmt.Sprintf("%q in %s", name, node.Import.Path))
		} else {
			s.WriteString(fmt.Sprintf("%q in %s", name, node.Import.URL))
		}

		node = node.next
	}

	return s.String()
}

// Migrate performs migrations on the import chain
func (ic *ImportChain) Migrate(build types.ZarfBuildData) (warnings []string) {
	node := ic.head
	for node != nil {
		migrated, w := deprecated.MigrateComponent(build, node.ZarfComponent)
		node.ZarfComponent = migrated
		warnings = append(warnings, w...)
		node = node.next
	}
	if len(warnings) > 0 {
		final := fmt.Sprintf("migrations were performed on the import chain of: %q", ic.head.Name)
		warnings = append(warnings, final)
	}
	return warnings
}

// Compose merges the import chain into a single component
// fixing paths, overriding metadata, etc
func (ic *ImportChain) Compose() (composed *types.ZarfComponent, err error) {
	composed = &ic.tail.ZarfComponent

	if ic.tail.prev == nil {
		// only had one component in the import chain
		return composed, nil
	}

	if err := ic.fetchOCISkeleton(); err != nil {
		return nil, err
	}

	// start with an empty component to compose into
	composed = &types.ZarfComponent{}

	// start overriding with the tail node
	node := ic.tail
	for node != nil {
		fixPaths(&node.ZarfComponent, node.relativeToHead)

		// perform overrides here
		err := overrideMetadata(composed, node.ZarfComponent)
		if err != nil {
			return nil, err
		}

		overrideDeprecated(composed, node.ZarfComponent)
		overrideResources(composed, node.ZarfComponent)
		overrideActions(composed, node.ZarfComponent)

		composeExtensions(composed, node.ZarfComponent, node.relativeToHead)

		node = node.prev
	}

	return composed, nil
}

// MergeVariables merges variables from the import chain
func (ic *ImportChain) MergeVariables(existing []types.ZarfPackageVariable) (merged []types.ZarfPackageVariable) {
	exists := func(v1 types.ZarfPackageVariable, v2 types.ZarfPackageVariable) bool {
		return v1.Name == v2.Name
	}

	node := ic.tail
	for node != nil {
		// merge the vars
		merged = helpers.MergeSlices(node.vars, merged, exists)
		node = node.prev
	}
	merged = helpers.MergeSlices(existing, merged, exists)

	return merged
}

// MergeConstants merges constants from the import chain
func (ic *ImportChain) MergeConstants(existing []types.ZarfPackageConstant) (merged []types.ZarfPackageConstant) {
	exists := func(c1 types.ZarfPackageConstant, c2 types.ZarfPackageConstant) bool {
		return c1.Name == c2.Name
	}

	node := ic.tail
	for node != nil {
		// merge the consts
		merged = helpers.MergeSlices(node.consts, merged, exists)
		node = node.prev
	}
	merged = helpers.MergeSlices(existing, merged, exists)

	return merged
}

// CompatibleComponent determines if this component is compatible with the given create options
func CompatibleComponent(c types.ZarfComponent, arch, flavor string) bool {
	satisfiesArch := c.Only.Cluster.Architecture == "" || c.Only.Cluster.Architecture == arch
	satisfiesFlavor := c.Only.Flavor == "" || c.Only.Flavor == flavor
	return satisfiesArch && satisfiesFlavor
}
