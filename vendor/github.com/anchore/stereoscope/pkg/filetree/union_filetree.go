package filetree

import "fmt"

// UnionFileTree stacks file trees as OCI changesets, lowest pushed first. Trees pushed here are interpreted
// as layer diffs rather than as plain filesystems: whiteout entries (".wh.*", and the opaque directory marker
// ".wh..wh..opq") are changeset metadata that Squash consumes and never emits. Use FileTree.Copy directly for
// a faithful copy with no changeset interpretation.
type UnionFileTree struct {
	trees []ReadWriter
}

func NewUnionFileTree() *UnionFileTree {
	return &UnionFileTree{
		trees: make([]ReadWriter, 0),
	}
}

func (u *UnionFileTree) PushTree(t ReadWriter) {
	u.trees = append(u.trees, t)
}

// Squash applies each pushed tree onto the one below it and returns the result. The result never contains
// whiteout entries, including any carried by the lowest tree, which means a Squash of a single tree is not the
// same as a Copy of that tree.
func (u *UnionFileTree) Squash() (ReadWriter, error) {
	if len(u.trees) == 0 {
		return New(), nil
	}

	var squashedTree ReadWriter
	var err error
	for layerIdx, refTree := range u.trees {
		if layerIdx == 0 {
			squashedTree, err = refTree.Copy()
			if err != nil {
				return nil, err
			}
			// every tree above this one is merged, which applies and drops its whiteout entries, but the
			// lowest tree is only copied. Whiteouts are changeset metadata and never materialize as files
			// in an applied rootfs, so strip them here.
			if err = removeWhiteouts(squashedTree); err != nil {
				return nil, err
			}
			continue
		}

		if err = squashedTree.Merge(refTree); err != nil {
			return nil, fmt.Errorf("unable to squash layer=%d : %w", layerIdx, err)
		}
	}
	return squashedTree, nil
}

// removeWhiteouts drops any whiteout marker from the given tree. Note that markers cannot be dropped when a
// layer tree is built, since Merge reads them off the upper tree to know what to remove.
func removeWhiteouts(t ReadWriter) error {
	for _, p := range t.AllRealPaths() {
		if !p.IsWhiteout() {
			continue
		}
		if err := t.RemovePath(p); err != nil {
			return fmt.Errorf("unable to remove whiteout marker (path=%s): %w", p, err)
		}
	}
	return nil
}
