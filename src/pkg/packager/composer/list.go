// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// Node is a node in the import chain
type Node struct {
	types.ZarfComponent

	prev *Node
	next *Node
}

// ImportChain is a doubly linked list of components
type ImportChain struct {
	head *Node
	tail *Node
}

func (ic *ImportChain) append(c types.ZarfComponent) {
	node := &Node{ZarfComponent: c, prev: nil, next: nil}
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
	ic := &ImportChain{}

	ic.append(head)

	history := []string{}

	node := ic.head
	for node != nil {
		isLocal := node.Import.Path != "" && node.Import.URL == ""
		isRemote := node.Import.Path == "" && node.Import.URL != ""

		if !isLocal && !isRemote {
			// EOL
			return ic, nil
		}

		if node.prev != nil && node.prev.Import.URL != "" {
			return ic, fmt.Errorf("detected malformed import chain, cannot import remote components from remote components")
		}

		var pkg types.ZarfPackage
		name := node.Name

		if isLocal {
			history = append(history, node.Import.Path)
			paths := append(history, layout.ZarfYAML)
			if err := utils.ReadYaml(filepath.Join(paths...), &pkg); err != nil {
				return ic, err
			}
		} else if isRemote {
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

		found := helpers.Find(pkg.Components, func(c types.ZarfComponent) bool {
			return c.Name == name
		})

		if found.Name == "" {
			if isLocal {
				return ic, fmt.Errorf("component %q not found in %q", name, filepath.Join(history...))
			} else if isRemote {
				return ic, fmt.Errorf("component %q not found in %q", name, node.Import.URL)
			}
		}

		if node.Only.Cluster.Architecture != "" {
			arch = node.Only.Cluster.Architecture
		}

		if arch != "" && found.Only.Cluster.Architecture != "" && found.Only.Cluster.Architecture != arch {
			if isLocal {
				return ic, fmt.Errorf("component %q is not compatible with %q architecture in %q", name, arch, filepath.Join(history...))
			} else if isRemote {
				return ic, fmt.Errorf("component %q is not compatible with %q architecture in %q", name, arch, node.Import.URL)
			}
		}

		ic.append(found)
		node = node.next
	}
	return ic, nil
}

// History returns the history of the import chain
func (ic *ImportChain) History() []string {
	history := []string{}
	node := ic.head
	for node != nil {
		history = append(history, node.Import.Path)
		if node.Import.URL != "" {
			history = append(history, node.Import.URL)
		}
		node = node.next
	}
	return history
}

// Compose merges the import chain into a single component
// fixing paths, overriding metadata, etc
func (ic *ImportChain) Compose() (composed types.ZarfComponent) {
	node := ic.tail

	if ic.tail.Import.URL != "" {
		composed = ic.tail.ZarfComponent
		// TODO: handle remote components
		// this should download the remote component tarball, fix the paths, then compose it
		node = node.prev
	}

	for node != nil {
		// if we are on the last node, set the starting point
		if composed.Name == "" {
			composed = node.ZarfComponent
			node = node.prev
			continue
		}

		// TODO: fix the paths to be relative to the head node
		// use history for that

		// perform overrides here
		overrideMetadata(&composed, node.ZarfComponent)
		overrideDeprecated(&composed, node.ZarfComponent)
		overrideResources(&composed, node.ZarfComponent)
		overrideExtensions(&composed, node.ZarfComponent)
		overrideActions(&composed, node.ZarfComponent)

		node = node.prev
	}

	return composed
}
