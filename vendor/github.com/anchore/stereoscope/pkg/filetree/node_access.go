package filetree

import (
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
)

// nodeAccess represents a request into the tree for a specific path and the resulting node, which may have a different path.
type nodeAccess struct {
	RequestPath        file.Path
	FileNode           *filenode.FileNode // note: it is important that nodeAccess does not behave like FileNode (then it can be added to the tree directly)
	LeafLinkResolution []nodeAccess
}

func (na *nodeAccess) HasFileNode() bool {
	if na == nil {
		return false
	}
	return na.FileNode != nil
}

func (na *nodeAccess) FileResolution() *file.Resolution {
	if !na.HasFileNode() {
		return nil
	}
	return file.NewResolution(
		na.RequestPath,
		na.FileNode.Reference,
		newResolutions(na.LeafLinkResolution),
	)
}

func (na *nodeAccess) References() []file.Reference {
	if !na.HasFileNode() {
		return nil
	}
	var refs []file.Reference

	if na.FileNode.Reference != nil {
		refs = append(refs, *na.FileNode.Reference)
	}

	for _, l := range na.LeafLinkResolution {
		if l.HasFileNode() && l.FileNode.Reference != nil {
			refs = append(refs, *l.FileNode.Reference)
		}
	}

	return refs
}
