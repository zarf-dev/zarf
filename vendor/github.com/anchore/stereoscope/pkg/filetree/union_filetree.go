package filetree

import "fmt"

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

func (u *UnionFileTree) Squash() (ReadWriter, error) {
	switch len(u.trees) {
	case 0:
		return New(), nil
	case 1:
		return u.trees[0].Copy()
	}

	var squashedTree ReadWriter
	var err error
	for layerIdx, refTree := range u.trees {
		if layerIdx == 0 {
			squashedTree, err = refTree.Copy()
			if err != nil {
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
