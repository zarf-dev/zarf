// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// Node is a node in the import chain
type Node struct {
	types.ZarfComponent

	vars   []types.ZarfPackageVariable
	consts []types.ZarfPackageConstant

	relativeToHead string

	prev *Node
	next *Node
}

// ImportChain is a doubly linked list of components
type ImportChain struct {
	head *Node
	tail *Node

	progress *message.ProgressBar
}

func (ic *ImportChain) append(c types.ZarfComponent, relativeToHead string, vars []types.ZarfPackageVariable, consts []types.ZarfPackageConstant) {
	node := &Node{
		ZarfComponent:  c,
		relativeToHead: relativeToHead,
		vars:           vars,
		consts:         consts,
		prev:           nil,
		next:           nil,
	}
	if ic.head == nil {
		ic.head = node
		ic.tail = node
	} else {
		p := ic.head
		for p.next != nil {
			p = p.next
		}
		node.prev = p

		p.next = node
		ic.tail = node
	}
}

// NewImportChain creates a new import chain from a component
func NewImportChain(head types.ZarfComponent, arch string) (*ImportChain, error) {
	if arch == "" {
		return nil, fmt.Errorf("cannot build import chain: architecture must be provided")
	}

	ic := &ImportChain{}

	ic.append(head, "", nil, nil)

	history := []string{}

	node := ic.head
	// todo: verify this behavior
	if ic.head.Only.Cluster.Architecture != "" {
		arch = node.Only.Cluster.Architecture
	}
	for node != nil {
		isLocal := node.Import.Path != "" && node.Import.URL == ""
		isRemote := node.Import.Path == "" && node.Import.URL != ""

		if !isLocal && !isRemote {
			// This is the end of the import chain,
			// as the current node/component is not importing anything
			return ic, nil
		}

		if node.prev != nil && node.prev.Import.URL != "" && isRemote {
			return ic, fmt.Errorf("detected malformed import chain, cannot import remote components from remote components")
		}

		var pkg types.ZarfPackage
		name := node.Name

		if isLocal {
			history = append(history, node.Import.Path)
			relativeToHead := filepath.Join(history...)
			// this assumes the composed package is following the zarf layout
			if err := utils.ReadYaml(filepath.Join(relativeToHead, layout.ZarfYAML), &pkg); err != nil {
				return ic, err
			}
		} else if isRemote {
			if !strings.HasSuffix(node.Import.URL, oci.SkeletonSuffix) {
				return ic, fmt.Errorf("remote component %q does not have a %q suffix", node.Import.URL, oci.SkeletonSuffix)
			}
			remote, err := oci.NewOrasRemote(node.Import.URL)
			if err != nil {
				return ic, err
			}
			pkg, err = remote.FetchZarfYAML()
			if err != nil {
				return ic, err
			}
		}

		if node.Import.ComponentName != "" {
			name = node.Import.ComponentName
		}

		found := helpers.Filter(pkg.Components, func(c types.ZarfComponent) bool {
			matchesName := c.Name == name
			satisfiesArch := c.Only.Cluster.Architecture == "" || c.Only.Cluster.Architecture == arch
			return matchesName && satisfiesArch
		})

		if len(found) == 0 {
			if isLocal {
				return ic, fmt.Errorf("component %q not found in %q", name, filepath.Join(history...))
			} else if isRemote {
				return ic, fmt.Errorf("component %q not found in %q", name, node.Import.URL)
			}
		} else if len(found) > 1 {
			// TODO: improve this error message / figure out the best way to present this error
			if isLocal {
				return ic, fmt.Errorf("multiple components named %q found in %q", name, filepath.Join(history...))
			} else if isRemote {
				return ic, fmt.Errorf("multiple components named %q found in %q", name, node.Import.URL)
			}
		}

		ic.append(found[0], filepath.Join(history...), pkg.Variables, pkg.Constants)
		node = node.next
	}
	return ic, nil
}

// History returns the history of the import chain
func (ic *ImportChain) History() (history [][]string) {
	node := ic.head
	for node != nil {
		if node.Import.URL != "" {
			history = append(history, []string{node.Name, node.Import.URL})
			continue
		}
		history = append(history, []string{node.Name, node.relativeToHead, node.Import.Path})
		node = node.next
	}
	return history
}

// TODO: is this the best place to perform migrations?
func (ic *ImportChain) Migrate(build types.ZarfBuildData) (warnings []string) {
	node := ic.head
	for node != nil {
		migrated, w := deprecated.MigrateComponent(build, node.ZarfComponent)
		node.ZarfComponent = migrated
		warnings = append(warnings, w...)
		node = node.next
	}
	return warnings
}

// Compose merges the import chain into a single component
// fixing paths, overriding metadata, etc
func (ic *ImportChain) Compose() (composed types.ZarfComponent) {
	composed = ic.tail.ZarfComponent

	node := ic.tail

	if ic.tail.prev.Import.URL != "" {
		// TODO: handle remote components
		// this should download the remote component tarball, fix the paths, then compose it
		node = node.prev
	}

	for node != nil {
		fixPaths(&node.ZarfComponent, node.relativeToHead)

		// perform overrides here
		overrideMetadata(&composed, node.ZarfComponent)
		overrideDeprecated(&composed, node.ZarfComponent)
		overrideResources(&composed, node.ZarfComponent)
		overrideActions(&composed, node.ZarfComponent)

		composeExtensions(&composed, node.ZarfComponent, node.relativeToHead)

		node = node.prev
	}

	return composed
}

func (ic *ImportChain) MergeVariables(vars []types.ZarfPackageVariable) (merged []types.ZarfPackageVariable) {
	merged = vars

	node := ic.head
	for node != nil {
		// // merge the vars
		for _, v := range node.vars {
			exists := false
			for _, vv := range merged {
				if v.Name == vv.Name {
					exists = true
					break
				}
			}
			if !exists {
				merged = append(merged, v)
			}
		}
		node = node.next
	}
	return merged
}

func (ic *ImportChain) MergeConstants(consts []types.ZarfPackageConstant) (merged []types.ZarfPackageConstant) {
	merged = consts

	node := ic.head
	for node != nil {
		// merge the consts
		for _, c := range node.consts {
			exists := false
			for _, cc := range merged {
				if c.Name == cc.Name {
					exists = true
					break
				}
			}
			if !exists {
				merged = append(merged, c)
			}
		}
		node = node.next
	}
	return merged
}
