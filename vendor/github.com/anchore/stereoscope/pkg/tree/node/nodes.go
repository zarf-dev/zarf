package node

import "sort"

type Nodes []Node

func (n Nodes) Len() int {
	return len(n)
}

func (n Nodes) Swap(idx1, idx2 int) {
	n[idx1], n[idx2] = n[idx2], n[idx1]
}

func (n Nodes) Less(idx1, idx2 int) bool {
	return n[idx1].ID() < n[idx2].ID()
}

func (n Nodes) Equal(other Nodes) bool {
	// TODO: this is bad, since it changes the order of the nodes, which is unexpected for the caller
	// however, this is only supporting tests, which need to be refactored.
	sort.Sort(n)
	sort.Sort(other)

	if len(n) != len(other) {
		return false
	}
	for i, v := range n {
		if v != other[i] {
			return false
		}
	}
	return true
}
