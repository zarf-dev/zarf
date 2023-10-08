package composer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

type node struct {
	cwd string
	types.ZarfComponent

	prev *node
	next *node
}

type importChain struct {
	head *node
	tail *node
}

func (ic *importChain) append(c types.ZarfComponent, cwd string) {
	node := &node{ZarfComponent: c, cwd: cwd, prev: nil, next: nil}
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

func NewImportChain(head types.ZarfComponent, arch string) (*importChain, error) {
	ic := &importChain{}

	cwd, err := os.Getwd()
	if err != nil {
		return ic, err
	}
	ic.append(head, cwd)

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
			cwd = filepath.Join(cwd, node.Import.Path)
			if err := utils.ReadYaml(filepath.Join(cwd, layout.ZarfYAML), &pkg); err != nil {
				return ic, err
			}
		} else if isRemote {
			cwd = ""
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
				return ic, fmt.Errorf("component %q not found in package %q", name, filepath.Join(cwd, layout.ZarfYAML))
			} else if isRemote {
				return ic, fmt.Errorf("component %q not found in package %q", name, node.Import.URL)
			}
		}

		if node.Only.Cluster.Architecture != "" {
			arch = node.Only.Cluster.Architecture
		}

		if arch != "" && found.Only.Cluster.Architecture != "" && found.Only.Cluster.Architecture != arch {
			if isLocal {
				return ic, fmt.Errorf("component %q is not compatible with %q architecture in package %q", name, arch, filepath.Join(cwd, layout.ZarfYAML))
			} else if isRemote {
				return ic, fmt.Errorf("component %q is not compatible with %q architecture in package %q", name, arch, node.Import.URL)
			}
		}

		ic.append(found, cwd)
		node = node.next
	}
	return ic, nil
}

func (ic *importChain) Print() {
	// components := []types.ZarfComponent{}
	paths := []string{}
	node := ic.head
	for node != nil {
		// components = append(components, node)
		paths = append(paths, node.cwd)
		if node.cwd == "" && node.Import.URL != "" {
			paths = append(paths, node.Import.URL)
		}
		node = node.next
	}
	// fmt.Println(message.JSONValue(components))
	fmt.Println(message.JSONValue(paths))
}

func (ic *importChain) Compose() (composed types.ZarfComponent) {
	node := ic.tail

	if ic.tail.Import.URL != "" {
		composed = ic.tail.ZarfComponent
		// TODO: handle remote components
		// this should download the remote component, fix the paths, then compose it
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
		// use node.cwd for that

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
