package tree

import (
	"github.com/anchore/stereoscope/pkg/tree/node"
)

type Reader interface {
	Node(id node.ID) node.Node
	Nodes() node.Nodes
	Children(n node.Node) node.Nodes
	Parent(n node.Node) node.Node
	Roots() node.Nodes
}
