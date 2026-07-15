package filetree

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
)

// prevent link cycles for paths that are self-referential (e.g. /home/wagoodman -> /home, which resolves
// to /home/wagoodman/home, which resolves to /home/wagoodman/home/home, and so on...).
// This is an arbitrarily large number (but not "too" large).
const maxDirDepth = 500

var ErrMaxTraversalDepth = errors.New("max allowable directory traversal depth reached (maybe a link cycle?)")

type FileNodeVisitor func(file.Path, filenode.FileNode) error

type WalkConditions struct {
	// Return true when the walker should stop traversing (before visiting current Node)
	ShouldTerminate func(file.Path, filenode.FileNode) bool

	// Whether we should visit the current Node. Note: this will continue down the same traversal
	// path, only "skipping" over a single Node (but still potentially visiting children later)
	// Return true to visit the current Node.
	ShouldVisit func(file.Path, filenode.FileNode) bool

	// Whether we should consider children of this Node to be included in the traversal path.
	// Return true to traverse children of this Node.
	ShouldContinueBranch func(file.Path, filenode.FileNode) bool

	LinkOptions []LinkResolutionOption
}

// DepthFirstPathWalker implements stateful depth-first Tree traversal.
type DepthFirstPathWalker struct {
	visitor      FileNodeVisitor
	tree         *FileTree
	pathStack    file.PathStack
	visitedPaths file.PathSet
	conditions   WalkConditions
}

func NewDepthFirstPathWalker(tree *FileTree, visitor FileNodeVisitor, conditions *WalkConditions) *DepthFirstPathWalker {
	w := &DepthFirstPathWalker{
		visitor:      visitor,
		tree:         tree,
		visitedPaths: file.NewPathSet(),
	}
	if conditions != nil {
		w.conditions = *conditions
	}
	return w
}

func (w *DepthFirstPathWalker) Walk(from file.Path) (file.Path, *filenode.FileNode, error) {
	w.pathStack.Push(from)

	var (
		currentPath file.Path
		currentNode *nodeAccess
		err         error
	)

	linkOpts := []LinkResolutionOption{followAncestorLinks}
	// Setup link options defaults
	if w.conditions.LinkOptions == nil {
		linkOpts = []LinkResolutionOption{followAncestorLinks, DoNotFollowDeadBasenameLinks, FollowBasenameLinks}
	}

	linkOpts = append(linkOpts, w.conditions.LinkOptions...)
	linkStrat := newLinkResolutionStrategy(linkOpts...)

	for w.pathStack.Size() > 0 {
		currentPath = w.pathStack.Pop()

		currentNode, err = w.tree.node(currentPath, linkStrat)
		if err != nil {
			return "", nil, err
		}

		if !currentNode.HasFileNode() {
			return "", nil, fmt.Errorf("nil Node at path=%q", currentPath)
		}

		// prevent infinite loop
		if strings.Count(string(currentPath.Normalize()), file.DirSeparator) >= maxDirDepth {
			return currentPath, currentNode.FileNode, ErrMaxTraversalDepth
		}

		if w.conditions.ShouldTerminate != nil && w.conditions.ShouldTerminate(currentPath, *currentNode.FileNode) {
			return currentPath, currentNode.FileNode, nil
		}
		currentPath = currentPath.Normalize()

		// visit
		if w.visitor != nil && !w.visitedPaths.Contains(currentPath) {
			if w.conditions.ShouldVisit == nil || w.conditions.ShouldVisit != nil && w.conditions.ShouldVisit(currentPath, *currentNode.FileNode) {
				err := w.visitor(currentPath, *currentNode.FileNode)
				if err != nil {
					return currentPath, currentNode.FileNode, err
				}
				w.visitedPaths.Add(currentPath)
			}
		}

		if w.conditions.ShouldContinueBranch != nil && !w.conditions.ShouldContinueBranch(currentPath, *currentNode.FileNode) {
			continue
		}

		// enqueue child paths
		childPaths, err := w.tree.ListPaths(currentPath)
		if err != nil {
			return "", nil, err
		}
		sort.Sort(sort.Reverse(file.Paths(childPaths)))
		for _, childPath := range childPaths {
			w.pathStack.Push(childPath)
		}
	}

	return currentPath, currentNode.FileNode, nil
}

func (w *DepthFirstPathWalker) WalkAll() error {
	_, _, err := w.Walk("/")
	return err
}

func (w *DepthFirstPathWalker) Visited(p file.Path) bool {
	return w.visitedPaths.Contains(p)
}
