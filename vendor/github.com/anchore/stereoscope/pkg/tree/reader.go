package tree

import (
	"github.com/anchore/stereoscope/pkg/tree/node"
)

// Reader is read-only access to a Tree. Nodes returned from it are shared with copies of the Tree and must not be
// modified.
type Reader interface {
	Node(id node.ID) node.Node
	Nodes() node.Nodes
	Children(n node.Node) node.Nodes
	Parent(n node.Node) node.Node
	Roots() node.Nodes
}
