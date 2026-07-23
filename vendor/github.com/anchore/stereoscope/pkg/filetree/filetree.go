package filetree

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/scylladb/go-set/iset"
	"github.com/scylladb/go-set/strset"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/anchore/stereoscope/pkg/tree"
	"github.com/anchore/stereoscope/pkg/tree/node"
)

var ErrRemovingRoot = errors.New("cannot remove the root path (`/`) from the FileTree")
var ErrLinkCycleDetected = errors.New("cycle during symlink resolution")
var ErrLinkResolutionDepth = errors.New("maximum link resolution stack depth exceeded")
var maxLinkResolutionDepth = 100

// FileTree represents a file/directory Tree
type FileTree struct {
	tree *tree.Tree
}

// NewFileTree creates a new FileTree instance.
//
// Deprecated: use New() instead.
func NewFileTree() *FileTree {
	return New()
}

// New creates a new FileTree instance.
func New() *FileTree {
	t := tree.NewTree()

	// Initialize FileTree with a root "/" Node
	_ = t.AddRoot(filenode.NewDir("/", nil))

	return &FileTree{
		tree: t,
	}
}

// Copy returns a Copy of the current FileTree.
func (t *FileTree) Copy() (ReadWriter, error) {
	ct := New()
	ct.tree = t.tree.Copy()
	return ct, nil
}

// AllFiles returns all files within the FileTree (defaults to regular files only, but you can provide one or more allow types).
func (t *FileTree) AllFiles(types ...file.Type) []file.Reference {
	if len(types) == 0 {
		types = []file.Type{file.TypeRegular}
	}

	typeSet := iset.New()
	for _, t := range types {
		typeSet.Add(int(t))
	}

	var files []file.Reference
	for _, n := range t.tree.Nodes() {
		f := n.(*filenode.FileNode)
		if typeSet.Has(int(f.FileType)) && f.Reference != nil {
			files = append(files, *f.Reference)
		}
	}
	return files
}

func (t *FileTree) AllRealPaths() []file.Path {
	var files []file.Path
	for _, n := range t.tree.Nodes() {
		f := n.(*filenode.FileNode)
		if f != nil {
			files = append(files, f.RealPath)
		}
	}
	return files
}

func (t *FileTree) ListPaths(dir file.Path) ([]file.Path, error) {
	fna, err := t.node(dir, linkResolutionStrategy{
		FollowAncestorLinks: true,
		FollowBasenameLinks: true,
	})
	if err != nil {
		return nil, err
	}

	if !fna.HasFileNode() {
		return nil, nil
	}

	if fna.FileNode.FileType != file.TypeDirectory {
		return nil, nil
	}

	var listing []file.Path
	children := t.tree.Children(fna.FileNode)
	for _, child := range children {
		if child == nil {
			continue
		}
		childFn := child.(*filenode.FileNode)
		fn, err := t.node(childFn.RealPath, linkResolutionStrategy{
			FollowAncestorLinks: true,
			FollowBasenameLinks: false,
		})
		if err != nil {
			return nil, err
		}

		listing = append(listing, file.Path(path.Join(string(dir), fn.FileNode.RealPath.Basename())))
	}
	return listing, nil
}

// File fetches a file.Reference for the given path. Returns nil if the path does not exist in the FileTree.
func (t *FileTree) File(path file.Path, options ...LinkResolutionOption) (bool, *file.Resolution, error) {
	currentNode, err := t.file(path, options...)
	if err != nil {
		return false, nil, err
	}
	if currentNode.HasFileNode() {
		return true, currentNode.FileResolution(), err
	}
	return false, nil, err
}

// file fetches a file.Reference for the given path. Returns nil if the path does not exist in the FileTree.
func (t *FileTree) file(path file.Path, options ...LinkResolutionOption) (*nodeAccess, error) {
	userStrategy := newLinkResolutionStrategy(options...)
	// For:             /some/path/here
	// Where:           /some/path -> /other/place
	// And resolves to: /other/place/here

	// This means a few things:
	//  - /some/path/here CANNOT exist in the Tree. If it did, the parent /some/path would have to be a directory,
	//      but since we know it is a link this cannot be true.
	//  - /other/place DOES NOT need to exist in the Tree --this would be a dead link and is allowable. Under this case
	//      we return NIL.
	//  - /other/place/here DOES NOT need to exist in the Tree, it either
	//          a) exists as a regular file --in which case return the discovered file.Reference
	//	        b) does not exist --return NIL
	//          c) or exists as a symlink that may or may not resolve --this last case does not matter since the
	//             PATH has been resolved to a file.Reference, which is all that matters)
	//
	// Therefore we can safely lookup the path first without worrying about symlink resolution yet... if there is a
	// hit, return it! If not, fallback to symlink resolution.
	currentNode, err := t.node(path, linkResolutionStrategy{})
	if err != nil {
		return nil, err
	}
	if currentNode.HasFileNode() && (!currentNode.FileNode.IsLink() || currentNode.FileNode.IsLink() && !userStrategy.FollowBasenameLinks) {
		return currentNode, nil
	}

	// symlink resolution!... within the context of container images (which is outside of the responsibility of this object)
	// the only really valid resolution of symlinks is in squash trees (both for an image and a layer --NOT for trees
	// that represent a single union FS layer.
	currentNode, err = t.node(path, linkResolutionStrategy{
		FollowAncestorLinks:          true,
		FollowBasenameLinks:          userStrategy.FollowBasenameLinks,
		DoNotFollowDeadBasenameLinks: userStrategy.DoNotFollowDeadBasenameLinks,
	})
	if currentNode.HasFileNode() {
		return currentNode, err
	}
	return nil, err
}

func newResolutions(nodePath []nodeAccess) []file.Resolution {
	var refPath []file.Resolution
	for i, n := range nodePath {
		if i == len(nodePath)-1 && n.FileNode != nil {
			// this is already on the parent Access object (unless it is a dead link)
			break
		}
		access := file.Resolution{
			RequestPath: n.RequestPath,
		}
		if n.FileNode != nil {
			access.Reference = n.FileNode.Reference
		}

		refPath = append(refPath, access)
	}
	return refPath
}

func (t *FileTree) node(p file.Path, strategy linkResolutionStrategy) (*nodeAccess, error) {
	normalizedPath := p.Normalize()
	nodeID := filenode.IDByPath(normalizedPath)
	if !strategy.FollowLinks() {
		n := t.tree.Node(nodeID)
		if n == nil {
			return &nodeAccess{
				RequestPath: normalizedPath,
				FileNode:    nil,
			}, nil
		}
		return &nodeAccess{
			RequestPath: normalizedPath,
			FileNode:    n.(*filenode.FileNode),
		}, nil
	}

	var currentNode *nodeAccess
	var err error
	if strategy.FollowAncestorLinks {
		currentNode, err = t.resolveAncestorLinks(normalizedPath, nil, maxLinkResolutionDepth)
		if err != nil {
			if currentNode != nil {
				currentNode.RequestPath = normalizedPath
			}
			return currentNode, err
		}
	} else {
		n := t.tree.Node(nodeID)
		if n != nil {
			currentNode = &nodeAccess{
				RequestPath: normalizedPath,
				FileNode:    n.(*filenode.FileNode),
			}
		}
	}

	// link resolution has come up with nothing, return what we have so far
	if !currentNode.HasFileNode() {
		if currentNode != nil {
			currentNode.RequestPath = normalizedPath
		}
		return currentNode, nil
	}

	if strategy.FollowBasenameLinks {
		currentNode, err = t.resolveNodeLinks(currentNode, !strategy.DoNotFollowDeadBasenameLinks, nil, maxLinkResolutionDepth)
	}
	if currentNode != nil {
		currentNode.RequestPath = normalizedPath
	}

	return currentNode, err
}

// return FileNode of the basename in the given path (no resolution is done at or past the basename). Note: it is
// assumed that the given path has already been normalized.
func (t *FileTree) resolveAncestorLinks(path file.Path, currentlyResolvingLinkPaths file.PathCountSet, maxLinkDepth int) (*nodeAccess, error) {
	// performance optimization... see if there is a node at the path (as if it is a real path). If so,
	// use it, otherwise, continue with ancestor resolution
	currentNodeAccess, err := t.node(path, linkResolutionStrategy{})
	if err != nil {
		return nil, err
	}
	if currentNodeAccess.HasFileNode() {
		return currentNodeAccess, nil
	}

	var pathParts = strings.Split(string(path), file.DirSeparator)
	var currentPathStr string
	var currentPath file.Path

	// iterate through all parts of the path, replacing path elements with link resolutions where possible.
	for idx, part := range pathParts {
		if part == "" {
			// note: this means that we will NEVER resolve a symlink or file.Reference for /, which is OK
			continue
		}

		// cumulatively gather where we are currently at and provide a rich object
		currentPath = file.Path(currentPathStr + file.DirSeparator + part)
		currentPathStr = string(currentPath)

		// fetch the Node with NO link resolution strategy
		currentNodeAccess, err = t.node(currentPath, linkResolutionStrategy{})
		if err != nil {
			// should never occur
			return nil, err
		}

		if !currentNodeAccess.HasFileNode() {
			// we've reached a point where the given path that has never been observed. This can happen for one reason:
			// 1. the current path is really invalid and we should return NIL indicating that it cannot be resolved.
			// 2. the current path is a link? no, this isn't possible since we are iterating through constituent paths
			//      in order, so we are guaranteed to hit parent links in which we should adjust the search path accordingly.
			return currentNodeAccess, nil
		}

		// keep track of what we've resolved to so far...
		currentPath = currentNodeAccess.FileNode.RealPath

		// this is positively a path, however, there is no information about this Node. This may be OK since we
		// allow for adding children before parents (and even don't require the parent to ever be added --which is
		// potentially valid given the underlying messy data [tar headers]). In this case we keep building the path
		// (which we've already done at this point) and continue.
		if currentNodeAccess.FileNode.Reference == nil {
			continue
		}

		// by this point we definitely have a file reference, if this is a link (and not the basename) resolve any
		// links until the next Node is resolved (or not).
		isLastPart := idx == len(pathParts)-1
		if !isLastPart && currentNodeAccess.FileNode.IsLink() {
			currentNodeAccess, err = t.resolveNodeLinks(currentNodeAccess, true, currentlyResolvingLinkPaths, maxLinkDepth)
			if err != nil {
				// only expected to happen on cycles
				return currentNodeAccess, err
			}
			if currentNodeAccess.HasFileNode() {
				currentPath = currentNodeAccess.FileNode.RealPath
			}
			currentPathStr = string(currentPath)
		}
	}
	// by this point we have processed all constituent paths; there were no un-added paths and the path is guaranteed
	// to have followed link resolution.
	return currentNodeAccess, nil
}

// resolveNodeLinks takes the given FileNode and resolves all links at the base of the real path for the node (this implies
// that NO ancestors are considered).
func (t *FileTree) resolveNodeLinks(n *nodeAccess, followDeadBasenameLinks bool, currentlyResolvingLinkPaths file.PathCountSet, maxLinkDepth int) (*nodeAccess, error) {
	if n == nil {
		return nil, fmt.Errorf("cannot resolve links with nil Node given")
	}

	// we need to short-circuit link resolution that never resolves (cycles) due to a cycle referencing nodes that do not exist.
	// this represents current link resolution requests that are in progress. This set is pruned once the resolution
	// has been completed.
	if currentlyResolvingLinkPaths == nil {
		currentlyResolvingLinkPaths = file.NewPathCountSet()
	}

	// note: this assumes that callers are passing paths in which the constituent parts are NOT symlinks
	var lastNode *nodeAccess
	var nodePath []nodeAccess
	var nextPath file.Path
	currentNodeAccess := n

	// keep resolving links until a regular file or directory is found.
	// Note: this is NOT redundant relative to the 'currentlyResolvingLinkPaths' set. This set is used to short-circuit
	// real paths that have been revisited through potentially different links (or really anyway).
	realPathsVisited := strset.New()
	var err error
	for {
		// we need to short-circuit link resolution that never resolves (depth) due to a cycle referencing
		// maxLinkDepth is counted across all calls to resolveAncestorLinks and resolveNodeLinks
		maxLinkDepth--
		if maxLinkDepth < 1 {
			return nil, ErrLinkResolutionDepth
		}

		nodePath = append(nodePath, *currentNodeAccess)

		// if there is no next path, return this reference (dead link)
		if !currentNodeAccess.HasFileNode() {
			// the last path we tried to resolve is a dead link, persist the original path as the failed request
			if len(nodePath) > 0 {
				nodePath[len(nodePath)-1].RequestPath = nextPath
			}
			break
		}

		if realPathsVisited.Has(string(currentNodeAccess.FileNode.RealPath)) {
			return nil, ErrLinkCycleDetected
		}

		if !currentNodeAccess.FileNode.IsLink() {
			// no resolution and there is no next link (pseudo dead link)... return what you found
			// any content fetches will fail, but that's ok
			break
		}

		// prepare for the next iteration
		// already seen is important for the context of this loop
		realPathsVisited.Add(string(currentNodeAccess.FileNode.RealPath))

		nextPath = currentNodeAccess.FileNode.RenderLinkDestination()

		// no more links to follow
		if string(nextPath) == "" {
			break
		}

		// preserve the current Node for the next loop (in case we shouldn't follow a potentially dead link)
		lastNode = currentNodeAccess

		// break any cycles with non-existent paths (before attempting to look the path up again)
		if currentlyResolvingLinkPaths.Contains(nextPath) {
			return nil, ErrLinkCycleDetected
		}

		// get the next Node (based on the next path)
		// attempted paths maintains state across calls to resolveAncestorLinks
		currentlyResolvingLinkPaths.Add(nextPath)
		currentNodeAccess, err = t.resolveAncestorLinks(nextPath, currentlyResolvingLinkPaths, maxLinkDepth)
		if err != nil {
			if currentNodeAccess != nil {
				currentNodeAccess.LeafLinkResolution = append(currentNodeAccess.LeafLinkResolution, nodePath...)
			}

			// only expected to occur upon cycle detection
			return currentNodeAccess, err
		}
		currentlyResolvingLinkPaths.Remove(nextPath)
	}

	if !currentNodeAccess.HasFileNode() && !followDeadBasenameLinks {
		if lastNode != nil {
			lastNode.LeafLinkResolution = append(lastNode.LeafLinkResolution, nodePath...)
		}
		return lastNode, nil
	}

	if currentNodeAccess != nil {
		currentNodeAccess.LeafLinkResolution = append(currentNodeAccess.LeafLinkResolution, nodePath...)
	}
	return currentNodeAccess, nil
}

// FilesByGlob fetches zero to many file.References for the given glob pattern (considers symlinks).
func (t *FileTree) FilesByGlob(query string, options ...LinkResolutionOption) ([]file.Resolution, error) {
	var results []file.Resolution

	if len(query) == 0 {
		return nil, fmt.Errorf("no glob pattern given")
	}

	if query[0] != file.DirSeparator[0] {
		// this is for an image, so it should always be relative to root
		query = file.DirSeparator + query
	}

	doNotFollowDeadBasenameLinks := false
	for _, o := range options {
		if o == DoNotFollowDeadBasenameLinks {
			doNotFollowDeadBasenameLinks = true
		}
	}

	matches, err := doublestar.Glob(&osAdapter{
		filetree:                     t,
		doNotFollowDeadBasenameLinks: doNotFollowDeadBasenameLinks,
	}, query)
	if err != nil {
		return nil, err
	}

	for _, match := range matches {
		// consumers need to understand that these are absolute paths and not relative
		// ex: directory resolver should stop at the dir input and not traverse up the filetree
		matchPath := file.Path(match)
		if !path.IsAbs(match) {
			matchPath = file.Path(path.Join("/", match))
		}
		fna, err := t.node(matchPath, linkResolutionStrategy{
			FollowAncestorLinks:          true,
			FollowBasenameLinks:          true,
			DoNotFollowDeadBasenameLinks: doNotFollowDeadBasenameLinks,
		})
		if err != nil {
			return nil, err
		}
		// the Node must exist and should not be a directory
		if fna.HasFileNode() && fna.FileNode.FileType != file.TypeDirectory {
			result := file.NewResolution(
				matchPath,
				fna.FileNode.Reference,
				newResolutions(fna.LeafLinkResolution),
			)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	sort.Sort(file.Resolutions(results))

	return results, nil
}

// AddFile adds a new path representing a REGULAR file to the Tree. It also adds any ancestors of the path that are not already
// present in the Tree. The resulting file.Reference of the new (leaf) addition is returned. Note: NO symlink or
// hardlink resolution is performed on the given path --which implies that the given path MUST be a real path (have no
// links in constituent paths)
func (t *FileTree) AddFile(realPath file.Path) (*file.Reference, error) {
	fna, err := t.node(realPath, linkResolutionStrategy{})
	if err != nil {
		return nil, err
	}
	if fna.HasFileNode() {
		// this path already exists
		if fna.FileNode.FileType != file.TypeRegular {
			return nil, fmt.Errorf("path=%q already exists but is NOT a regular file", realPath)
		}
		// this is a regular file, provide a new or existing file.Reference
		if fna.FileNode.Reference == nil {
			fna.FileNode.Reference = file.NewFileReference(realPath)
		}
		return fna.FileNode.Reference, nil
	}

	// this is a new path... add the new Node + parents
	if err := t.addParentPaths(realPath); err != nil {
		return nil, err
	}
	newFn := filenode.NewFile(realPath, file.NewFileReference(realPath))
	return newFn.Reference, t.setFileNode(newFn)
}

// AddSymLink adds a new path to the Tree that represents a SYMLINK. A new file.Reference with a absolute or relative
// link path captured and returned. Note: NO symlink or hardlink resolution is performed on the given path --which
// implies that the given path MUST be a real path (have no links in constituent paths)
func (t *FileTree) AddSymLink(realPath file.Path, linkPath file.Path) (*file.Reference, error) {
	fna, err := t.node(realPath, linkResolutionStrategy{})
	if err != nil {
		return nil, err
	}
	if fna.HasFileNode() {
		// this path already exists
		if fna.FileNode.FileType != file.TypeSymLink {
			return nil, fmt.Errorf("path=%q already exists but is NOT a symlink file", realPath)
		}
		// this is a symlink file, provide a new or existing file.Reference
		if fna.FileNode.Reference == nil {
			fna.FileNode.Reference = file.NewFileReference(realPath)
		}
		return fna.FileNode.Reference, nil
	}

	// this is a new path... add the new Node + parents
	if err := t.addParentPaths(realPath); err != nil {
		return nil, err
	}
	newFn := filenode.NewSymLink(realPath, linkPath, file.NewFileReference(realPath))
	return newFn.Reference, t.setFileNode(newFn)
}

// AddHardLink adds a new path to the Tree that represents a HARDLINK. A new file.Reference with a absolute link
// path captured and returned. Note: NO symlink or hardlink resolution is performed on the given path --which
// implies that the given path MUST be a real path (have no links in constituent paths)
func (t *FileTree) AddHardLink(realPath file.Path, linkPath file.Path) (*file.Reference, error) {
	fna, err := t.node(realPath, linkResolutionStrategy{})
	if err != nil {
		return nil, err
	}
	if fna.HasFileNode() {
		// this path already exists
		if fna.FileNode.FileType != file.TypeHardLink {
			return nil, fmt.Errorf("path=%q already exists but is NOT a symlink file", realPath)
		}
		// this is a symlink file, provide a new or existing file.Reference
		if fna.FileNode.Reference == nil {
			fna.FileNode.Reference = file.NewFileReference(realPath)
		}
		return fna.FileNode.Reference, nil
	}

	// this is a new path... add the new Node + parents
	if err := t.addParentPaths(realPath); err != nil {
		return nil, err
	}

	newFn := filenode.NewHardLink(realPath, linkPath, file.NewFileReference(realPath))

	return newFn.Reference, t.setFileNode(newFn)
}

// AddDir adds a new path representing a DIRECTORY to the Tree. It also adds any ancestors of the path that are
// not already present in the Tree. The resulting file.Reference of the new (leaf) addition is returned.
// Note: NO symlink or hardlink resolution is performed on the given path --which implies that the given path MUST
// be a real path (have no links in constituent paths)
func (t *FileTree) AddDir(realPath file.Path) (*file.Reference, error) {
	fna, err := t.node(realPath, linkResolutionStrategy{})
	if err != nil {
		return nil, err
	}
	if fna.HasFileNode() {
		// this path already exists
		if fna.FileNode.FileType != file.TypeDirectory {
			return nil, fmt.Errorf("path=%q already exists but is NOT a symlink file", realPath)
		}
		// this is a directory, provide a new or existing file.Reference
		if fna.FileNode.Reference == nil {
			fna.FileNode.Reference = file.NewFileReference(realPath)
		}
		return fna.FileNode.Reference, nil
	}

	// this is a new path... add the new Node + parents
	if err := t.addParentPaths(realPath); err != nil {
		return nil, err
	}

	newFn := filenode.NewDir(realPath, file.NewFileReference(realPath))
	return newFn.Reference, t.setFileNode(newFn)
}

// addParentPaths adds paths into the Tree for all constituent paths, but does NOT attach a file.Reference for each new path.
// if the parent already exists, nothing is done and the function returns with no error. Note: NO symlink or hardlink
// resolution is performed on the given path --which implies that the given path MUST be a real path (have no
// links in constituent paths)
func (t *FileTree) addParentPaths(realPath file.Path) error {
	parentPath, err := realPath.ParentPath()
	if err != nil {
		return fmt.Errorf("unable to determine parent path while adding path=%q: %w", realPath, err)
	}

	fna, err := t.node(parentPath, linkResolutionStrategy{})
	if err != nil {
		return err
	}

	if !fna.HasFileNode() {
		// add parents of the Node until an existent parent is found it's important to do this in reverse order
		// to ensure we are checking the fewest amount of parents possible.
		var pathsToAdd []file.Path
		parentPaths := realPath.ConstituentPaths()
		for idx := len(parentPaths) - 1; idx >= 0; idx-- {
			resolvedFna, err := t.node(parentPaths[idx], linkResolutionStrategy{})
			if err != nil {
				return err
			}
			if resolvedFna.HasFileNode() {
				break
			}
			pathsToAdd = append(pathsToAdd, parentPaths[idx])
		}

		// add each path with no file reference; add these in sorted path order (which is guaranteed to be
		// the reverse of the order of insertion)
		for idx := len(pathsToAdd) - 1; idx >= 0; idx-- {
			newFn := filenode.NewDir(pathsToAdd[idx], nil)
			if err = t.setFileNode(newFn); err != nil {
				return err
			}
		}
	}
	return nil
}

// setFileNode adds the given path to the Tree with the specific file.Reference.
func (t *FileTree) setFileNode(fn *filenode.FileNode) error {
	if fn == nil {
		return fmt.Errorf("must provide a FileNode when adding paths")
	}

	if existingNode := t.tree.Node(filenode.IDByPath(fn.RealPath)); existingNode != nil {
		return t.tree.Replace(existingNode, fn)
	}

	parentPath, err := fn.RealPath.ParentPath()
	if err != nil {
		return fmt.Errorf("unable to determine parent path while adding path=%q: %w", fn.RealPath, err)
	}

	parentNode, err := t.node(parentPath, linkResolutionStrategy{})
	if err != nil {
		return err
	}
	if !parentNode.HasFileNode() {
		return fmt.Errorf("unable to find parent path=%q while adding path=%q", parentPath, fn.RealPath)
	}

	return t.tree.AddChild(parentNode.FileNode, fn)
}

// RemovePath deletes the file.Reference from the FileTree by the given path. If the basename of the given path
// is a symlink then the symlink is removed (not the destination of the symlink). If the path does not exist, this is a
// nop.
func (t *FileTree) RemovePath(path file.Path) error {
	if path.Normalize() == "/" {
		return ErrRemovingRoot
	}

	fna, err := t.node(path, linkResolutionStrategy{
		FollowAncestorLinks: true,
		FollowBasenameLinks: false,
	})
	if err != nil {
		return err
	}
	if !fna.HasFileNode() {
		return nil
	}

	_, err = t.tree.RemoveNode(fna.FileNode)
	if err != nil {
		return err
	}
	return nil
}

// RemoveChildPaths deletes all children of the given path (not including the given path). Note: if the given path
// basename is a symlink, then the symlink is followed before resolving children. If the path does not exist, this is a
// nop.
func (t *FileTree) RemoveChildPaths(path file.Path) error {
	fna, err := t.node(path, linkResolutionStrategy{
		FollowAncestorLinks: true,
		FollowBasenameLinks: true,
	})
	if err != nil {
		return err
	}
	if !fna.HasFileNode() {
		// can't remove child paths for Node that doesn't exist!
		return nil
	}
	for _, child := range t.tree.Children(fna.FileNode) {
		_, err := t.tree.RemoveNode(child)
		if err != nil {
			return err
		}
	}
	return nil
}

// TreeReader returns a tree.Reader useful for Tree traversal.
func (t *FileTree) TreeReader() tree.Reader {
	return t.tree
}

// PathDiff shows the path differences between two trees (useful for testing)
func (t *FileTree) PathDiff(other *FileTree) (extra, missing []file.Path) {
	ourPaths := strset.New()
	for _, fn := range t.tree.Nodes() {
		ourPaths.Add(string(fn.ID()))
	}

	theirPaths := strset.New()
	for _, fn := range other.tree.Nodes() {
		theirPaths.Add(string(fn.ID()))
	}

	for _, fn := range other.tree.Nodes() {
		if !ourPaths.Has(string(fn.ID())) {
			extra = append(extra, file.Path(fn.ID()))
		}
	}

	for _, fn := range t.tree.Nodes() {
		if !theirPaths.Has(string(fn.ID())) {
			missing = append(missing, file.Path(fn.ID()))
		}
	}

	return
}

// Equal indicates if the two trees have the same paths or not.
func (t *FileTree) Equal(other *FileTree) bool {
	if t.tree.Length() != other.tree.Length() {
		return false
	}

	extra, missing := t.PathDiff(other)

	return len(extra) == 0 && len(missing) == 0
}

// HasPath indicates is the given path is in the file Tree (with optional link resolution options).
func (t *FileTree) HasPath(path file.Path, options ...LinkResolutionOption) bool {
	exists, _, err := t.File(path, options...)
	if err != nil {
		return false
	}
	return exists
}

// Walk takes a visitor function and invokes it for all paths within the FileTree in depth-first ordering.
func (t *FileTree) Walk(fn func(path file.Path, f filenode.FileNode) error, conditions *WalkConditions) error {
	return NewDepthFirstPathWalker(t, fn, conditions).WalkAll()
}

// Merge takes the given Tree and combines it with the current Tree, preferring files in the other Tree if there
// are path conflicts. This is the basis function for squashing (where the current Tree is the bottom Tree and the
// given Tree is the top Tree).
func (t *FileTree) Merge(upper Reader) error {
	conditions := tree.WalkConditions{
		ShouldContinueBranch: func(n node.Node) bool {
			p := file.Path(n.ID())
			return !p.IsWhiteout()
		},
		ShouldVisit: func(n node.Node) bool {
			p := file.Path(n.ID())
			return !p.IsDirWhiteout()
		},
	}

	visitor := func(n node.Node) error {
		if n == nil {
			return fmt.Errorf("found nil Node while traversing %+v", upper)
		}
		upperNode := n.(*filenode.FileNode)
		// opaque directories must be processed first
		if hasOpaqueDirectory(upper, upperNode.RealPath) {
			err := t.RemoveChildPaths(upperNode.RealPath)
			if err != nil {
				return fmt.Errorf("filetree Merge failed to remove child paths (upperPath=%s): %w", upperNode.RealPath, err)
			}
		}

		if upperNode.RealPath.IsWhiteout() {
			lowerPath, err := upperNode.RealPath.UnWhiteoutPath()
			if err != nil {
				return fmt.Errorf("filetree Merge failed to find original upperPath for whiteout (upperPath=%s): %w", upperNode.RealPath, err)
			}

			err = t.RemovePath(lowerPath)
			if err != nil {
				return fmt.Errorf("filetree Merge failed to remove upperPath (upperPath=%s): %w", lowerPath, err)
			}

			return nil
		}

		lowerNode, err := t.node(upperNode.RealPath, linkResolutionStrategy{
			FollowAncestorLinks: false,
			FollowBasenameLinks: false,
		})
		if err != nil {
			return fmt.Errorf("filetree Merge failed when looking for path=%q : %w", upperNode.RealPath, err)
		}
		if !lowerNode.HasFileNode() {
			// there is no existing Node... add parents and prepare to set
			if err := t.addParentPaths(upperNode.RealPath); err != nil {
				return fmt.Errorf("could not add parent paths to lower: %w", err)
			}
		}

		nodeCopy := *upperNode

		// keep original file references if the upper tree does not have them (only for the same file types)
		if lowerNode.HasFileNode() && lowerNode.FileNode.Reference != nil && upperNode.Reference == nil && upperNode.FileType == lowerNode.FileNode.FileType {
			nodeCopy.Reference = lowerNode.FileNode.Reference
		}

		if lowerNode.HasFileNode() && upperNode.FileType != file.TypeDirectory && lowerNode.FileNode.FileType == file.TypeDirectory {
			// NOTE: both upperNode and lowerNode paths are the same, and does not have an effect
			// on removal of child paths
			err := t.RemoveChildPaths(upperNode.RealPath)
			if err != nil {
				return fmt.Errorf("filetree Merge failed to remove children for non-directory upper node (%s): %w", upperNode.RealPath, err)
			}
		}
		// graft a copy of the upper Node with potential lower information into the lower tree
		if err := t.setFileNode(&nodeCopy); err != nil {
			return fmt.Errorf("filetree Merge failed to set file Node (Node=%+v): %w", nodeCopy, err)
		}

		return nil
	}

	// we are using the tree walker instead of the path walker to only look at an resolve merging of real files
	// with no consideration to virtual paths (paths that are valid in the filetree because constituent paths
	// contain symlinks).
	return tree.NewDepthFirstWalkerWithConditions(upper.TreeReader(), visitor, conditions).WalkAll()
}

func hasOpaqueDirectory(t Reader, directoryPath file.Path) bool {
	opaqueWhiteoutChild := file.Path(path.Join(string(directoryPath), file.OpaqueWhiteout))
	return t.HasPath(opaqueWhiteoutChild)
}
