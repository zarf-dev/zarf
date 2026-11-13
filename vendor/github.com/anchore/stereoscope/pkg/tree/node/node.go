package node

// Node is a single element within a tree.Tree. A node in a Tree is shared with every copy of that Tree, so it must
// never be modified once added. To change one, swap in a new node with tree.Tree.Replace.
type Node interface {
	ID() ID
	Copy() Node
}
