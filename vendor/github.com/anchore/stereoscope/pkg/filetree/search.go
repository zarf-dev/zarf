package filetree

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/anchore/stereoscope/pkg/tree/node"
)

// Searcher is a facade for searching a file tree with optional indexing support.
type Searcher interface {
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

func NewSearchContext(tree Reader, index IndexReader) Searcher {
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
	entries, err := sc.index.GetByFileType(file.TypeSymLink, file.TypeHardLink)
	if err != nil {
		return err
	}

	// filter the results relative to the tree
	nodes, err := sc.fileNodesInTree(entries)
	if err != nil {
		return err
	}

	// note: the remaining references are all links that exist in the tree

	for _, fn := range nodes {
		destinationFna, err := sc.tree.file(fn.RenderLinkDestination())
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
	return ref, err
}

func (sc searchContext) SearchByMIMEType(mimeTypes ...string) ([]file.Resolution, error) {
	var fileEntries []IndexEntry

	for _, mType := range mimeTypes {
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

// add case for status.d/* like things that hook up directly into filetree.ListPaths()

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

	sort.Sort(file.Resolutions(allRefs))

	return allRefs, nil
}

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

func (sc searchContext) searchByParentBasename(request searchRequest) ([]file.Resolution, error) {
	indexes, err := sc.index.GetByBasename(request.indexLookup)
	if err != nil {
		return nil, fmt.Errorf("unable to search by extension=%q: %w", request.indexLookup, err)
	}

	var results []file.Resolution
	for _, i := range indexes {
		paths, err := sc.tree.ListPaths(i.RealPath)
		if err != nil {
			// this may not be a directory, that's alright, just continue...
			continue
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
	var references []file.Resolution
	for _, entry := range entries {
		ref, err := sc.firstMatchingReference(glob, string(entry.RealPath))
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
		if err != nil {
			return nil, fmt.Errorf("unable to get ref for path=%q: %w", fullPath, err)
		}

		// this filters out any entries that do not exist in the tree
		if !na.HasFileNode() {
			continue
		}

		nodeID := na.FileNode.ID()

		// check all paths to the node that are linked to any parent dir
		for _, linkSrcID := range sc.linkBackwardRefs[nodeID].List() {
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

func (sc searchContext) fileNodesInTree(fileEntries []IndexEntry) ([]*filenode.FileNode, error) {
	var nodes []*filenode.FileNode
allFileEntries:
	for _, entry := range fileEntries {
		// note: it is important that we don't enable any basename link resolution
		na, err := sc.tree.file(entry.RealPath)
		if err != nil {
			return nil, fmt.Errorf("unable to get ref for path=%q: %w", entry.RealPath, err)
		}

		if !na.HasFileNode() {
			continue
		}

		// only check the resolved node matches the index entries reference, not via link resolutions...
		if na.FileNode.Reference != nil && na.FileNode.Reference.ID() == entry.ID() {
			nodes = append(nodes, na.FileNode)
			continue allFileEntries
		}

		// we did not find a matching file ID in the tree, so drop this entry
	}
	return nodes, nil
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
