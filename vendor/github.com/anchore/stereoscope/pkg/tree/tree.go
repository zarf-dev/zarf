package tree

import (
	"fmt"
	"maps"

	"github.com/anchore/stereoscope/pkg/tree/node"
)

// Tree represents a simple Tree data structure.
//
// Nodes are shared between a Tree and its copies, so a node must never be mutated once it is in a Tree. To change
// a node, Replace it with a new one.
type Tree struct {
	nodes    map[node.ID]entry                // {node-id: node + parent-id}
	children map[node.ID]map[node.ID]struct{} // {parent-id: {child-id}} (absent or empty for nodes with no children)
}

// entry keeps the parent next to the node so a Tree costs one key per node, not two.
type entry struct {
	node      node.Node
	parent    node.ID
	hasParent bool
}

// NewTree returns an instance of a Tree.
func NewTree() *Tree {
	return &Tree{
		nodes:    make(map[node.ID]entry),
		children: make(map[node.ID]map[node.ID]struct{}),
	}
}

// Copy returns a structural copy of the Tree. Nodes are shared with the original (see Tree).
func (t *Tree) Copy() *Tree {
	ct := &Tree{
		nodes:    maps.Clone(t.nodes),
		children: make(map[node.ID]map[node.ID]struct{}, len(t.children)),
	}
	for from, set := range t.children {
		if len(set) > 0 {
			ct.children[from] = maps.Clone(set)
		}
	}
	return ct
}

// Roots is all of the nodes with no parents.
func (t *Tree) Roots() node.Nodes {
	nodes := make([]node.Node, 0)
	for _, e := range t.nodes {
		if !e.hasParent {
			nodes = append(nodes, e.node)
		}
	}
	return nodes
}

// HasNode indicates is the given node ID exists in the Tree.
func (t *Tree) HasNode(id node.ID) bool {
	_, exists := t.nodes[id]
	return exists
}

// Node returns a node object for the given ID.
func (t *Tree) Node(id node.ID) node.Node {
	return t.nodes[id].node
}

// Nodes returns all nodes in the Tree.
func (t *Tree) Nodes() node.Nodes {
	if len(t.nodes) == 0 {
		return nil
	}
	nodes := make([]node.Node, 0, len(t.nodes))
	for _, e := range t.nodes {
		nodes = append(nodes, e.node)
	}
	return nodes
}

// addNode adds the node to the Tree; returns an error on node ID collisions.
func (t *Tree) addNode(n node.Node) error {
	if _, exists := t.nodes[n.ID()]; exists {
		return fmt.Errorf("node ID collision: %+v", n.ID())
	}
	t.nodes[n.ID()] = entry{node: n}
	return nil
}

// Replace takes the given old node and replaces it with the given new one.
func (t *Tree) Replace(old node.Node, newNode node.Node) error {
	oid, nid := old.ID(), newNode.ID()
	oldEntry, ok := t.nodes[oid]
	if !ok {
		return fmt.Errorf("cannot replace node not in the Tree")
	}

	if oid == nid {
		// the underlying objects may be different, but the ID's match. Simply track the new node and keep all
		// existing relationships.
		oldEntry.node = newNode
		t.nodes[nid] = oldEntry
		return nil
	}

	if t.HasNode(nid) {
		return fmt.Errorf("node ID collision: %+v", nid)
	}

	// take over the old node's place under its parent
	t.nodes[nid] = entry{node: newNode, parent: oldEntry.parent, hasParent: oldEntry.hasParent}
	if oldEntry.hasParent {
		delete(t.children[oldEntry.parent], oid)
		t.children[oldEntry.parent][nid] = struct{}{}
	}

	// take over the old node's children
	if set, ok := t.children[oid]; ok {
		for cid := range set {
			t.setParent(cid, nid)
		}
		t.children[nid] = set
	}

	delete(t.children, oid)
	delete(t.nodes, oid)

	return nil
}

func (t *Tree) setParent(id, parent node.ID) {
	e := t.nodes[id]
	e.parent, e.hasParent = parent, true
	t.nodes[id] = e
}

// AddRoot adds a node to the Tree (with no parent).
func (t *Tree) AddRoot(n node.Node) error {
	return t.addNode(n)
}

// AddChild adds the child node (to) under the parent node (from), adding either to the Tree if missing (a missing
// parent becomes a root). The stored objects for both nodes are replaced with the ones given. If the child already
// has a different parent it is moved, along with its subtree, to the new parent. Every node has at most one parent,
// and an edge that would create a cycle is rejected.
func (t *Tree) AddChild(from, to node.Node) error {
	var (
		fid = from.ID()
		tid = to.ID()
	)

	if fid == tid {
		return fmt.Errorf("should not add self edge")
	}

	// a node not yet in the tree has no descendants, so only an existing child can close a cycle
	if t.HasNode(tid) {
		for e := t.nodes[fid]; e.hasParent; e = t.nodes[e.parent] {
			if e.parent == tid {
				return fmt.Errorf("adding %+v under %+v would create a cycle", tid, fid)
			}
		}
	}

	fe := t.nodes[fid]
	fe.node = from
	t.nodes[fid] = fe

	// reparenting an existing node must drop the edge from its old parent, otherwise removing the old parent
	// would take this node (and its subtree) with it
	if te := t.nodes[tid]; te.hasParent {
		delete(t.children[te.parent], tid)
	}
	t.nodes[tid] = entry{node: to, parent: fid, hasParent: true}

	set, ok := t.children[fid]
	if !ok {
		set = make(map[node.ID]struct{})
		t.children[fid] = set
	}
	set[tid] = struct{}{}
	return nil
}

// RemoveNode deletes the node from the Tree and returns the removed node.
func (t *Tree) RemoveNode(n node.Node) (node.Nodes, error) {
	removedNodes := make([]node.Node, 0)
	nid := n.ID()
	e, ok := t.nodes[nid]
	if !ok {
		return nil, fmt.Errorf("unable to remove node: %+v", nid)
	}
	for cid := range t.children[nid] {
		child, ok := t.nodes[cid]
		if !ok {
			// a stale edge to a node that is already gone (AddChild no longer leaves one, but keep the guard)
			continue
		}
		subNodes, err := t.RemoveNode(child.node)
		removedNodes = append(removedNodes, subNodes...)
		if err != nil {
			return nil, err
		}
	}

	removedNodes = append(removedNodes, e.node)

	delete(t.children, nid)
	if e.hasParent {
		delete(t.children[e.parent], nid)
	}
	delete(t.nodes, nid)
	return removedNodes, nil
}

// Children returns all children of the given node.
func (t *Tree) Children(n node.Node) node.Nodes {
	set := t.children[n.ID()]
	if len(set) == 0 {
		return nil
	}

	from := make([]node.Node, 0, len(set))
	for cid := range set {
		from = append(from, t.nodes[cid].node)
	}

	return from
}

// Parent returns the parent of the given node (or nil if it is a root)
func (t *Tree) Parent(n node.Node) node.Node {
	if e := t.nodes[n.ID()]; e.hasParent {
		return t.nodes[e.parent].node
	}
	return nil
}

func (t *Tree) Length() int {
	return len(t.nodes)
}
