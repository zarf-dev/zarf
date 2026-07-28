package tree

import (
	"sort"

	"github.com/anchore/stereoscope/pkg/tree/node"
)

type NodeVisitor func(node.Node) error

type WalkConditions struct {
	// Return true when the walker should stop traversing (before visiting current node)
	ShouldTerminate func(node.Node) bool

	// Whether we should visit the current node. Note: this will continue down the same traversal
	// path, only "skipping" over a single node (but still potentially visiting children later)
	// Return true to visit the current node.
	ShouldVisit func(node.Node) bool

	// Whether we should consider children of this node to be included in the traversal path.
	// Return true to traverse children of this node.
	ShouldContinueBranch func(node.Node) bool
}

// DepthFirstWalker implements stateful depth-first Tree traversal.
type DepthFirstWalker struct {
	visitor    NodeVisitor
	tree       Reader
	stack      node.Stack
	visited    node.IDSet
	conditions WalkConditions
}

func NewDepthFirstWalker(reader Reader, visitor NodeVisitor) *DepthFirstWalker {
	return &DepthFirstWalker{
		visitor: visitor,
		tree:    reader,
		visited: node.NewIDSet(),
	}
}

func NewDepthFirstWalkerWithConditions(reader Reader, visitor NodeVisitor, conditions WalkConditions) *DepthFirstWalker {
	return &DepthFirstWalker{
		visitor:    visitor,
		tree:       reader,
		visited:    node.NewIDSet(),
		conditions: conditions,
	}
}

func (w *DepthFirstWalker) Walk(from node.Node) (node.Node, error) {
	w.stack.Push(from)

	for w.stack.Size() > 0 {
		current := w.stack.Pop()
		if w.conditions.ShouldTerminate != nil && w.conditions.ShouldTerminate(current) {
			return current, nil
		}
		cid := current.ID()

		// visit
		if w.visitor != nil && !w.visited.Contains(cid) {
			if w.conditions.ShouldVisit == nil || w.conditions.ShouldVisit != nil && w.conditions.ShouldVisit(current) {
				if err := w.visitor(current); err != nil {
					return current, err
				}
				w.visited.Add(cid)
			}
		}

		if w.conditions.ShouldContinueBranch != nil && !w.conditions.ShouldContinueBranch(current) {
			continue
		}

		// enqueue children
		children := w.tree.Children(current)
		sort.Sort(sort.Reverse(children))
		for _, child := range children {
			w.stack.Push(child)
		}
	}

	return nil, nil
}

func (w *DepthFirstWalker) WalkAll() error {
	for _, from := range w.tree.Roots() {
		if _, err := w.Walk(from); err != nil {
			return err
		}
	}
	return nil
}

func (w *DepthFirstWalker) Visited(n node.Node) bool {
	return w.visited.Contains(n.ID())
}
