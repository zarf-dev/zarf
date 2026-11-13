package image

import (
	"fmt"
	"path"
	"strings"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
)

// deferredHardLink is one hardlink header waiting for the layers below its own.
type deferredHardLink struct {
	ref file.Reference
	// header is what the tar said, kept since a later header at the same path removes this one's catalog entry
	header file.Metadata
	// via is the index in deferredHardLinks of the link this one names, or -1 when it names a lower layer's file
	via int
}

// deferHardLink decides whether a hardlink header that indexing could not adopt waits for the layers below. It
// does when nothing in this layer is at its name yet (via -1), or when what is there is a link of this layer that
// is itself deferred (via is that link's index). Anything else this layer already has at the name, a directory or
// an unadoptable link, is what Windows links to, never a lower file, so the header is left as it is.
func deferHardLink(ft filetree.Reader, deferred map[file.ID]int, metadata file.Metadata) (via int, ok bool) {
	if metadata.LinkDestination == "" {
		return -1, false
	}
	n := literalNode(ft, file.Path(path.Clean(file.DirSeparator+metadata.LinkDestination)))
	if n == nil {
		// trace, not warn: a per-header warning would be noisy on any archive that trips this
		log.WithFields("path", metadata.Path, "linkName", metadata.LinkDestination).
			Trace("hardlink names a file that is not in this layer, deferring it to the layers below")
		return -1, true
	}
	if n.Reference != nil {
		if i, ok := deferred[n.Reference.ID()]; ok {
			return i, true
		}
	}
	return -1, false
}

// a Windows layer keeps its filesystem under Files/ and the utility VM's under UtilityVM/Files/. hcsshim only
// resolves a link into a lower layer when both its name and its target are under one of them.
const (
	windowsFilesPrefix          = "Files/"
	windowsUtilityVMFilesPrefix = "UtilityVM/Files/"
)

// maxWindowsLinkName bounds the link names this resolves, and so the work per link and layer.
// ponytail: real WinSxS names are a few hundred bytes. Raise it if a real image has longer ones
const maxWindowsLinkName = 4096

// underWindowsFiles reports whether p, a tar name without a leading separator, is somewhere hcsshim links.
func underWindowsFiles(p string) bool {
	return strings.HasPrefix(p, windowsFilesPrefix) || strings.HasPrefix(p, windowsUtilityVMFilesPrefix)
}

// adoptLowerLayerHardLinks adopts the hardlinks of layer that name a file in a lower layer, given below: every
// layer under it, bottom first. Only Windows images have these. Their update layers link WinSxS names to System32
// files that only the base layer carries. Linux builders keep a hardlink and its target in one layer, so callers
// pass no layers below for a Linux image and its links stay as their headers describe them. Which file a link
// gets is resolveWindowsLink's call.
//
// A link naming another deferred link of this layer gets what that link got, and fails with it. Links are in tar
// order, so a chain of any depth resolves in one pass.
//
// Each adopted name's link node is replaced in this layer's tree by a node of the adopted type, since lookups
// would otherwise follow the link to its target's path. The old reference leaves the catalog, and the layer's
// search context is rebuilt. It must run before the layer is merged into the squash.
func adoptLowerLayerHardLinks(below []*Layer, layer *Layer) error {
	pending := layer.deferredHardLinks
	layer.deferredHardLinks = nil
	if len(below) == 0 || len(pending) == 0 {
		return nil
	}

	tree, ok := layer.Tree.(filetree.ReadWriter)
	if !ok {
		return fmt.Errorf("layer %d tree is not writable", layer.Metadata.Index)
	}
	builder := filetree.NewBuilder(tree, layer.fileCatalog.Index)
	lookup := newLowerLayerLookup(layer, below)

	type boundLink struct {
		metadata file.Metadata
		opener   file.Opener
	}
	bound := make(map[int]boundLink, len(pending))

	adopted := 0
	for i, d := range pending {
		metadata := d.header
		var opener file.Opener
		switch {
		case !underWindowsFiles(strings.TrimPrefix(string(d.ref.RealPath), file.DirSeparator)):
			// hcsshim refuses the link
		case d.via >= 0:
			if target, ok := bound[d.via]; ok {
				bindHardLink(target.metadata, &metadata)
				opener = target.opener
			}
		default:
			opener = lookup.adopt(layer.fileCatalog, &metadata)
		}

		// the node must still be this link's: a later header at the path has its own reference. And replacing a
		// node with children would drop them, as would RemovePath under a non-directory, which follows it
		node := literalNode(tree, d.ref.RealPath)
		if opener == nil || node == nil || node.Reference == nil || node.Reference.ID() != d.ref.ID() ||
			len(tree.TreeReader().Children(node)) > 0 || hasNonDirAncestor(tree, d.ref.RealPath) {
			log.WithFields("path", metadata.Path, "linkName", metadata.LinkDestination).
				Trace("hardlink names no file hcsshim would link from the layers below, indexing it from its own header")
			continue
		}
		bound[i] = boundLink{metadata: metadata, opener: opener}

		if err := tree.RemovePath(d.ref.RealPath); err != nil {
			return fmt.Errorf("unable to replace hardlink %q: %w", d.ref.RealPath, err)
		}
		newRef, err := builder.Add(metadata)
		if err != nil {
			return fmt.Errorf("unable to replace hardlink %q: %w", d.ref.RealPath, err)
		}
		layer.fileCatalog.remove(d.ref)
		layer.fileCatalog.addImageReferences(newRef.ID(), layer, opener)
		adopted++
	}

	if adopted > 0 {
		layer.SearchContext = filetree.NewSearchContext(layer.Tree, layer.fileCatalog.Index)
	}
	log.WithFields("index", layer.Metadata.Index, "adopted", adopted, "pending", len(pending)).
		Trace("adopted hardlinks naming files in lower layers")
	return nil
}

// lowerLayerLookup resolves one layer's deferred links against the layers below it, folding each layer's tree
// once, on first use, since most links are found in the nearest layer that could have them and never reach the
// others.
type lowerLayerLookup struct {
	own   func() foldedIndex
	below []func() foldedIndex
}

func newLowerLayerLookup(own *Layer, bottomFirst []*Layer) *lowerLayerLookup {
	once := func(l *Layer) func() foldedIndex {
		var idx foldedIndex
		return func() foldedIndex {
			if idx == nil {
				idx = foldPaths(l.Tree)
			}
			return idx
		}
	}
	below := make([]func() foldedIndex, len(bottomFirst))
	for i, l := range bottomFirst {
		below[len(bottomFirst)-1-i] = once(l)
	}
	return &lowerLayerLookup{own: once(own), below: below}
}

// adopt rewrites metadata, a deferred hardlink header's, to describe the file it links to and returns that file's
// opener, or nil when it links to nothing this can describe.
func (l *lowerLayerLookup) adopt(fileCatalog *FileCatalog, metadata *file.Metadata) file.Opener {
	n := resolveWindowsLink(metadata.LinkDestination, l.own, l.below)
	if n == nil {
		return nil
	}
	target, err := fileCatalog.Get(*n.Reference)
	// a target that is still a hardlink is one that failed to adopt (see adoptHardLinkInode)
	if err != nil || target.Type == file.TypeDirectory || target.Type == file.TypeHardLink {
		return nil
	}
	opener := fileCatalog.opener(n.Reference.ID())
	if opener != nil {
		bindHardLink(target.Metadata, metadata)
	}
	return opener
}

// resolveWindowsLink returns the node that hcsshim's AddLink, which imports a Windows layer, links name to when the
// link's own layer doesn't have it: name is the header's link name, own the link's layer, and below the layers under
// it, nearest first. Each is a folded index, built when first reached. It returns nil when hcsshim would refuse the
// link, or when this can't be sure which node it gets, so the link keeps its own header.
//
// A Files/ target is looked up in each parent layer's own files, nearest first, and the first layer that has the
// path wins. Whiteouts in between are not consulted, and no reparse point (symlink) is followed: one on the way
// fails the import. NTFS matches names case-insensitively, so a nearer layer with the name in a different case is
// the one Windows links to, and is refused here rather than skipped.
//
// A UtilityVM/Files/ target is different: hcsshim clones the parent's whole utility VM into this layer, where
// whiteouts delete files and this layer's own entries land too. There a whiteout on the way, or anything this
// layer has at the name, refuses the link.
func resolveWindowsLink(name string, own func() foldedIndex, below []func() foldedIndex) *filenode.FileNode {
	// hcsshim cleans the name without a root, so ".." can climb out of Files/ and fail the import, and it reads "\"
	// as a separator where a tar name does not
	name = path.Clean(name)
	if !underWindowsFiles(name) || strings.Contains(name, `\`) || len(name) > maxWindowsLinkName {
		return nil
	}
	layers := below
	uvm := strings.HasPrefix(name, windowsUtilityVMFilesPrefix)
	if uvm {
		layers = append([]func() foldedIndex{own}, below...)
	}
	want := file.Path(file.DirSeparator + name)
	// ponytail: Unicode simple case folding, which NTFS's upcase table follows outside a few scripts. A name that
	// folds differently there is matched in the wrong layer
	components := strings.Split(strings.ToUpper(name), file.DirSeparator)

	for i, index := range layers {
		hit, refused := index().find(components, uvm)
		if refused {
			return nil
		}
		if hit == nil {
			continue
		}
		// a utility VM target the link's own layer has may have been written before or after the link, which the
		// tree doesn't order
		if hit.RealPath != want || hit.Reference == nil || hit.FileType == file.TypeDirectory || (uvm && i == 0) {
			return nil
		}
		return hit
	}
	return nil
}

// foldedIndex is a layer's tree indexed by upper-cased path components, the way NTFS matches names.
type foldedIndex map[foldedName]foldedNode

// foldedName is one upper-cased path component inside a folded directory. Looking a name up one component at a
// time keeps the work linear in its length, where hashing every prefix would cost its depth times its length.
type foldedName struct {
	dir  int
	name string
	// whiteout is set for the whiteout of name, rather than name itself
	whiteout bool
}

// foldedNode is what a foldedName names: its node, or nil when two paths fold together, and its own folded id.
type foldedNode struct {
	node *filenode.FileNode
	id   int
}

// foldPaths indexes every path in tree by its upper-cased components.
func foldPaths(tree filetree.Reader) foldedIndex {
	nodes := tree.TreeReader().Nodes()
	// ids numbers each folded directory path; the root is 0
	ids := map[string]int{file.DirSeparator: 0}
	id := func(p string) int {
		if i, ok := ids[p]; ok {
			return i
		}
		ids[p] = len(ids)
		return ids[p]
	}
	whiteoutPrefix := strings.ToUpper(file.WhiteoutPrefix)
	m := make(foldedIndex, len(nodes))
	for _, n := range nodes {
		fn, ok := n.(*filenode.FileNode)
		if !ok || fn == nil || fn.RealPath == file.DirSeparator {
			continue
		}
		p := strings.ToUpper(string(fn.RealPath))
		i := strings.LastIndex(p, file.DirSeparator)
		parent, base := p[:i], p[i+1:]
		if parent == "" {
			parent = file.DirSeparator
		}
		k := foldedName{dir: id(parent), name: base}
		if _, dup := m[k]; dup {
			fn = nil
		}
		m[k] = foldedNode{node: fn, id: id(p)}
		if strings.HasPrefix(base, whiteoutPrefix) {
			m[foldedName{dir: k.dir, name: strings.TrimPrefix(base, whiteoutPrefix), whiteout: true}] = foldedNode{}
		}
	}
	return m
}

// find walks components, an upper-cased path split at each separator, down the index. It returns the node at the
// end, or nil when the layer doesn't have the path, which moves the lookup on to the next layer. It refuses when the
// layer has a name on the way in more than one case, or a non-directory on the way (a reparse point or a file:
// either fails the lookup with an error that stops the import, rather than a miss), or with whiteouts, a whiteout of
// the path or a directory on the way.
func (f foldedIndex) find(components []string, whiteouts bool) (hit *filenode.FileNode, refused bool) {
	dir := 0
	for i, c := range components {
		if _, ok := f[foldedName{dir: dir, name: c, whiteout: true}]; ok && whiteouts {
			return nil, true
		}
		n, ok := f[foldedName{dir: dir, name: c}]
		if !ok {
			// nothing deeper is in this layer
			return nil, false
		}
		if n.node == nil {
			return nil, true
		}
		if i == len(components)-1 {
			return n.node, false
		}
		if n.node.FileType != file.TypeDirectory {
			return nil, true
		}
		dir = n.id
	}
	return nil, false
}
