// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"
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

	remote *oci.OrasRemote
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

		// todo: explain me
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
			remote, err := ic.getRemote(node.Import.URL)
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

// Migrate performs migrations on the import chain
//
// TODO: is this the best place to perform migrations?
func (ic *ImportChain) Migrate(build types.ZarfBuildData) (warnings []string) {
	node := ic.head
	for node != nil {
		migrated, w := deprecated.MigrateComponent(build, node.ZarfComponent)
		node.ZarfComponent = migrated
		warnings = append(warnings, w...)
		node = node.next
	}
	// TODO: make a final warning if warnings are found
	return warnings
}

func (ic *ImportChain) getRemote(url string) (*oci.OrasRemote, error) {
	if ic.remote != nil {
		return ic.remote, nil
	}
	var err error
	ic.remote, err = oci.NewOrasRemote(url)
	if err != nil {
		return nil, err
	}
	return ic.remote, nil
}

func (ic *ImportChain) fetchOCISkeleton() error {
	// only the 2nd to last node will have a remote import
	node := ic.tail.prev
	if node.Import.URL == "" {
		// nothing to fetch
		return nil
	}
	remote, err := ic.getRemote(node.Import.URL)
	if err != nil {
		return err
	}

	manifest, err := remote.FetchRoot()
	if err != nil {
		return err
	}

	componentDesc := manifest.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf("%s.tar", node.Name)))

	if oci.IsEmptyDescriptor(componentDesc) {
		// nothing to fetch
		return nil
	}

	cache := filepath.Join(config.GetAbsCachePath(), "oci")
	store, err := ocistore.New(cache)
	if err != nil {
		return err
	}

	tb := filepath.Join(cache, "blobs", "sha256", componentDesc.Digest.String())

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dir := strings.TrimSuffix(tb, ".tar")
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		return err
	}
	// this node is the node importing the remote component
	// and has a filepath relative to the head of the import chain
	// the next (tail) node will have a filepath relative from cwd to the tarball in cache
	ic.tail.relativeToHead = rel

	if exists, err := store.Exists(context.TODO(), componentDesc); err != nil {
		return err
	} else if !exists {
		if err := remote.CopyWithProgress([]ocispec.Descriptor{componentDesc}, store, nil, cache); err != nil {
			return err
		}
	}

	if !utils.InvalidPath(dir) {
		// already extracted
		return nil
	}
	tu := archiver.Tar{}
	return tu.Unarchive(tb, dir)
}

// Compose merges the import chain into a single component
// fixing paths, overriding metadata, etc
func (ic *ImportChain) Compose() (composed types.ZarfComponent, err error) {
	if ic.tail.prev == nil {
		// only had one component in the import chain
		return composed, nil
	}

	if err := ic.fetchOCISkeleton(); err != nil {
		return composed, err
	}

	node := ic.tail
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

	return composed, nil
}

// MergeVariables merges variables from the import chain
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

// MergeConstants merges constants from the import chain
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
