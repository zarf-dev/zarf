package filetree

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/scylladb/go-set/strset"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/anchore/stereoscope/pkg/tree/node"
)

// Searcher is a facade for searching a file tree with optional indexing support. All three methods skip paths
// whose links are malformed (see IsUnresolvableLink) rather than failing the search, so a nil or absent result
// means "absent or unresolvable".
//
// Index-backed searches (SearchByMIMEType, and SearchByGlob when an index is present) return at most one resolution
// per file: at its real path when that matches, otherwise through the lexicographically first link path that does.
// A tree walk (SearchByGlob without an index) returns every matching access path, so /lib/x.so and /usr/lib/x.so
// both come back for a usrmerge image where an index-backed search returns only /usr/lib/x.so.
type Searcher interface {
	// SearchByPath returns nil with a nil error when the path is absent OR its link cannot be resolved.
	SearchByPath(path string, options ...LinkResolutionOption) (*file.Resolution, error)
	SearchByGlob(patterns string, options ...LinkResolutionOption) ([]file.Resolution, error)
	SearchByMIMEType(mimeTypes ...string) ([]file.Resolution, error)
}

type searchContext struct {
	tree  *FileTree   // this is the tree which all index search results are filtered against
	index IndexReader // this index is relative to one or more trees, not just necessarily one

	// the following enables correct link resolution when searching via the index
	linkBackwardRefs map[node.ID]node.IDSet // {link-destination-node-id: str([link-node-id, ...])}
}

// NewSearchContext returns a Searcher over the given tree, using the index to answer searches. Given a
// reader from NewLazySquash, neither the tree nor the link resolution index is built until the first
// search. The index is optional: when it is a nil interface value (not a typed nil pointer), glob searches
// fall back to walking the whole tree and MIME type searches return an error. Path searches never use the index.
func NewSearchContext(tree Reader, index IndexReader) Searcher {
	if l, ok := tree.(*lazyReader); ok {
		return newLazySearcher(l, index)
	}

	c := &searchContext{
		tree:             tree.(*FileTree),
		index:            index,
		linkBackwardRefs: make(map[node.ID]node.IDSet),
	}

	if err := c.buildLinkResolutionIndex(); err != nil {
		log.WithFields("error", err).Warn("unable to build link resolution index for filetree search context")
	}

	return c
}

func (sc *searchContext) buildLinkResolutionIndex() error {
	if sc.index == nil {
		// link resolution only matters for index-based searches
		return nil
	}

	entries, err := sc.index.GetByFileType(file.TypeSymLink, file.TypeHardLink)
	if err != nil {
		return err
	}

	// filter the results relative to the tree
	nodes := sc.fileNodesInTree(entries)

	// note: the remaining references are all links that exist in the tree

	for _, fn := range nodes {
		destinationFna, err := sc.tree.file(fn.RenderLinkDestination())
		if IsUnresolvableLink(err) {
			log.WithFields("path", fn.RealPath, "error", err).Trace("skipping malformed link while building link resolution index")
			continue
		}
		if err != nil {
			return fmt.Errorf("unable to get node for path=%q: %w", fn.RealPath, err)
		}

		if !destinationFna.HasFileNode() {
			// we were unable to resolve the link destination, this could be due to the fact that the destination simply
			continue
		}

		linkID := fn.ID()
		destinationID := destinationFna.FileNode.ID()

		// add backward reference...
		if _, ok := sc.linkBackwardRefs[destinationID]; !ok {
			sc.linkBackwardRefs[destinationID] = node.NewIDSet()
		}
		sc.linkBackwardRefs[destinationID].Add(linkID)
	}

	return nil
}

func (sc searchContext) SearchByPath(path string, options ...LinkResolutionOption) (*file.Resolution, error) {
	// TODO: one day this could leverage indexes outside of the tree, but today this is not implemented
	options = append(options, FollowBasenameLinks)
	_, ref, err := sc.tree.File(file.Path(path), options...)
	if IsUnresolvableLink(err) {
		log.WithFields("path", path, "error", err).Trace("skipping path with malformed link during search")
		return nil, nil
	}
	return ref, err
}

func (sc searchContext) SearchByMIMEType(mimeTypes ...string) ([]file.Resolution, error) {
	if sc.index == nil {
		// the tree alone has no MIME type information, so there is nothing to search against
		return nil, errors.New("unable to search by MIME type: no index available")
	}

	var fileEntries []IndexEntry

	// the index keeps each file under exactly one MIME type, so deduping the requested types keeps results unique
	for _, mType := range strset.New(mimeTypes...).List() {
		entries, err := sc.index.GetByMIMEType(mType)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch file references by MIME type (%q): %w", mType, err)
		}
		fileEntries = append(fileEntries, entries...)
	}

	refs, err := sc.firstMatchingReferences("**/*", fileEntries)
	if err != nil {
		return nil, err
	}

	sort.Sort(file.Resolutions(refs))

	return refs, nil
}

func (sc searchContext) SearchByGlob(pattern string, options ...LinkResolutionOption) ([]file.Resolution, error) {
	if sc.index == nil {
		options = append(options, FollowBasenameLinks)
		refs, err := sc.tree.FilesByGlob(pattern, options...)
		if err != nil {
			return nil, fmt.Errorf("unable to search by glob=%q: %w", pattern, err)
		}
		sort.Sort(file.Resolutions(refs))
		return refs, nil
	}

	var allRefs []file.Resolution
	for _, request := range parseGlob(pattern) {
		refs, err := sc.searchByRequest(request, options...)
		if err != nil {
			return nil, fmt.Errorf("unable to search by glob=%q: %w", pattern, err)
		}
		allRefs = append(allRefs, refs...)
	}

	// overlapping alternation sections (e.g. {a.txt,*.txt}) can surface the same request path more than once
	allRefs = uniqueResolutions(allRefs)

	sort.Sort(file.Resolutions(allRefs))

	return allRefs, nil
}

func uniqueResolutions(refs []file.Resolution) []file.Resolution {
	seen := file.NewPathSet()
	return slices.DeleteFunc(refs, func(ref file.Resolution) bool {
		if seen.Contains(ref.RequestPath) {
			return true
		}
		seen.Add(ref.RequestPath)
		return false
	})
}

// searchByRequest requires a non-nil index (see the guard in SearchByGlob).
func (sc searchContext) searchByRequest(request searchRequest, options ...LinkResolutionOption) ([]file.Resolution, error) {
	switch request.searchBasis {
	case searchByFullPath:
		options = append(options, FollowBasenameLinks)
		ref, err := sc.SearchByPath(request.indexLookup, options...)
		if err != nil {
			return nil, err
		}
		if ref == nil {
			return nil, nil
		}
		return []file.Resolution{*ref}, nil
	case searchByBasename:
		indexes, err := sc.index.GetByBasename(request.indexLookup)
		if err != nil {
			return nil, fmt.Errorf("unable to search by basename=%q: %w", request.indexLookup, err)
		}
		resolutions, err := sc.firstMatchingReferences(request.glob, indexes)
		if err != nil {
			return nil, err
		}
		return resolutions, nil
	case searchByBasenameGlob:
		indexes, err := sc.index.GetByBasenameGlob(request.indexLookup)
		if err != nil {
			return nil, fmt.Errorf("unable to search by basename-glob=%q: %w", request.indexLookup, err)
		}
		resolutions, err := sc.firstMatchingReferences(request.glob, indexes)
		if err != nil {
			return nil, err
		}
		return resolutions, nil
	case searchByExtension:
		indexes, err := sc.index.GetByExtension(request.indexLookup)
		if err != nil {
			return nil, fmt.Errorf("unable to search by extension=%q: %w", request.indexLookup, err)
		}
		resolutions, err := sc.firstMatchingReferences(request.glob, indexes)
		if err != nil {
			return nil, err
		}
		return resolutions, nil
	case searchBySubDirectory:
		return sc.searchByParentBasename(request)

	case searchByGlob:
		log.WithFields("glob", request.glob).Trace("glob provided is an expensive search, consider using a more specific indexed search")

		options = append(options, FollowBasenameLinks)
		return sc.tree.FilesByGlob(request.glob, options...)
	}

	return nil, fmt.Errorf("invalid search request: %+v", request.searchBasis)
}

// searchByParentBasename requires a non-nil index (see the guard in SearchByGlob).
func (sc searchContext) searchByParentBasename(request searchRequest) ([]file.Resolution, error) {
	indexes, err := sc.index.GetByBasename(request.indexLookup)
	if err != nil {
		return nil, fmt.Errorf("unable to search by parent-basename=%q: %w", request.indexLookup, err)
	}

	// unlike the other index searches, stale entries are deliberately kept (no fileNodesInTree filter). An entry only
	// claims a directory with this basename lives at its path, which still holds when an upper layer reroutes an
	// ancestor through a symlink, and when the link target was only created implicitly it has no index entry of its
	// own, so the stale entry is the only way to find it. Each entry is resolved to the directory it really is and
	// listed there once: this dedupes entries from several layers (or several paths) that land on one directory, and
	// like the other index searches, a path through a link is only returned when the real path does not match.
	var results []file.Resolution
	listed := file.NewPathSet()
	for _, i := range indexes {
		na, err := sc.tree.node(i.RealPath, linkResolutionStrategy{FollowAncestorLinks: true, FollowBasenameLinks: true})
		if IsUnresolvableLink(err) {
			log.WithFields("path", i.RealPath, "error", err).Trace("skipping parent directory with malformed link during search")
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("unable to resolve parent path=%q: %w", i.RealPath, err)
		}
		if !na.HasFileNode() || na.FileNode.FileType != file.TypeDirectory {
			// a missing path or a non-directory has nothing to list
			continue
		}
		dir := na.FileNode.RealPath
		if listed.Contains(dir) {
			continue
		}
		listed.Add(dir)

		paths, err := sc.tree.ListPaths(dir)
		if err != nil {
			return nil, fmt.Errorf("unable to list parent path=%q: %w", dir, err)
		}
		for _, p := range paths {
			nestedRef, err := sc.firstMatchingReference(request.glob, string(p))
			if err != nil {
				return nil, fmt.Errorf("unable to fetch file reference from parent path %q for path=%q: %w", i.RealPath, p, err)
			}
			if !nestedRef.HasReference() {
				continue
			}
			results = append(results, *nestedRef)
		}
	}

	return results, nil
}

func (sc searchContext) firstMatchingReferences(glob string, entries []IndexEntry) ([]file.Resolution, error) {
	// the index spans all layers, so drop entries that are not the node actually at their path in this tree
	// (e.g. a lower layer's file that was since overwritten). Otherwise the path resolves to whatever lives
	// there now, which may not match the property the entry was indexed by, and is duplicated per layer.
	nodes := sc.fileNodesInTree(entries)

	var references []file.Resolution
	for _, fn := range nodes {
		ref, err := sc.firstMatchingReference(glob, string(fn.RealPath))
		if err != nil {
			return nil, err
		}
		if ref != nil {
			references = append(references, *ref)
		}
	}
	return references, nil
}

func matchesGlob(ref file.Resolution, glob string) (bool, error) {
	allRefPaths := ref.AllRequestPaths()
	for _, p := range allRefPaths {
		matched, err := doublestar.Match(glob, string(p))
		if err != nil {
			return false, fmt.Errorf("unable to match glob pattern=%q to path=%q: %w", glob, p, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// firstPathToNode returns the first path matching the given glob, this is done by:
// * testing the provided path, returning the path if matching
// * expanding parent paths, replacing paths that are symlinks
func (sc searchContext) firstPathToNode(observedPaths file.PathSet, glob string, symlinkCheckedPath, suffix string) (*file.Resolution, error) {
	fullPath := file.Path(symlinkCheckedPath)
	if suffix != "" {
		if strings.HasPrefix(suffix, "/") {
			fullPath = file.Path(symlinkCheckedPath + suffix)
		} else {
			fullPath = file.Path(path.Join(symlinkCheckedPath, suffix))
		}
	}

	if observedPaths.Contains(fullPath) {
		// we've already observed this path, so we can stop here
		return nil, nil
	}
	observedPaths.Add(fullPath)

	// first, test the path against the glob and return it if matches
	_, ref, err := sc.tree.File(fullPath, FollowBasenameLinks)
	if IsUnresolvableLink(err) {
		log.WithFields("path", fullPath, "error", err).Trace("skipping path with malformed link during search")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ref != nil {
		matches, err := matchesGlob(*ref, glob)
		if err != nil {
			return nil, err
		}
		// path matches, don't need to check for more symlink path references
		if matches {
			return ref, nil
		}
	}

	// the first segment should is always an absolute path, starting with /, ensure it does here
	if !strings.HasPrefix(symlinkCheckedPath, "/") {
		symlinkCheckedPath = "/" + symlinkCheckedPath
	}

	for i := nextSegment(symlinkCheckedPath, 1); i > 0; i = nextSegment(symlinkCheckedPath, i+1) {
		dir := file.Path(symlinkCheckedPath[:i])
		remain := string(fullPath[i:])

		if observedPaths.Contains(dir) {
			// we've already observed this path, don't get in a loop, e.g. /usr/bin/X11 -> /usr/bin
			continue
		}
		observedPaths.Add(dir)

		na, err := sc.tree.file(dir) // do not follow symlinks here; this call is effectively following symlinks manually
		// note: unlike the fullPath lookup above, this one needs no malformed-link guard. dir is a prefix of
		// symlinkCheckedPath, so its ancestors are a subset of fullPath's, and the only way the lookup above
		// returns without doing ancestor resolution is fullPath existing as a real node -- which implies every
		// prefix exists as a real node too. So if dir could cycle here, fullPath already did and we returned.
		if err != nil {
			return nil, fmt.Errorf("unable to get ref for path=%q: %w", fullPath, err)
		}

		// this filters out any entries that do not exist in the tree
		if !na.HasFileNode() {
			continue
		}

		nodeID := na.FileNode.ID()

		// check all paths to the node that are linked to any parent dir. IDs are paths, so sorting makes the first
		// matching link path (and therefore the search result) the same from run to run
		for _, linkSrcID := range sc.linkBackwardRefs[nodeID].Sorted() {
			pfn := sc.tree.tree.Node(linkSrcID)
			if pfn == nil {
				log.WithFields("id", nodeID, "parent", linkSrcID).Trace("found non-existent parent link")
				continue
			}

			linkPath := string(pfn.(*filenode.FileNode).RealPath)
			ref, err = sc.firstPathToNode(observedPaths, glob, linkPath, remain)
			if ref != nil || err != nil {
				return ref, err
			}
		}
	}

	return nil, nil
}

func (sc searchContext) fileNodesInTree(fileEntries []IndexEntry) []*filenode.FileNode {
	var nodes []*filenode.FileNode
	for _, entry := range fileEntries {
		// a reference only ever lives at its own real path, so a literal lookup is enough. Following ancestor links
		// could only land on a different node (whose ID cannot match), and a literal lookup cannot fail.
		na, _ := sc.tree.node(entry.RealPath, linkResolutionStrategy{})
		if !na.HasFileNode() {
			continue
		}

		// keep the entry only if it is the node at its path in this tree, dropping entries from other layers
		if na.FileNode.Reference != nil && na.FileNode.Reference.ID() == entry.ID() {
			nodes = append(nodes, na.FileNode)
		}
	}
	return nodes
}

// firstMatchingReference returns the first file reference matching the glob
func (sc searchContext) firstMatchingReference(glob string, realPath string) (*file.Resolution, error) {
	return sc.firstPathToNode(file.PathSet{}, glob, realPath, "")
}

// nextSegment returns the next index a slash is found, the end of the string, or -1 if the request is >= string len
func nextSegment(s string, start int) int {
	if start >= len(s) {
		return -1
	}
	idx := strings.IndexRune(s[start:], '/')
	if idx < 0 {
		return len(s)
	}
	return idx + start
}
