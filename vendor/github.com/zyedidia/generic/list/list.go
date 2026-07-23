// Package list provides an implementation of a doubly-linked list with a front
// and back. The individual nodes of the list are publicly exposed so that the
// user can have fine-grained control over the list.
package list

// List implements a doubly-linked list.
type List[V any] struct {
	Front, Back *Node[V]
}

// Node is a node in the linked list.
type Node[V any] struct {
	Value      V
	Prev, Next *Node[V]
}

// New returns an empty linked list.
func New[V any]() *List[V] {
	return &List[V]{}
}

// PushBack adds 'v' to the end of the list.
func (l *List[V]) PushBack(v V) {
	l.PushBackNode(&Node[V]{
		Value: v,
	})
}

// PushFront adds 'v' to the beginning of the list.
func (l *List[V]) PushFront(v V) {
	l.PushFrontNode(&Node[V]{
		Value: v,
	})
}

// PushBackNode adds the node 'n' to the back of the list.
func (l *List[V]) PushBackNode(n *Node[V]) {
	n.Next = nil
	n.Prev = l.Back
	if l.Back != nil {
		l.Back.Next = n
	} else {
		l.Front = n
	}
	l.Back = n
}

// PushFrontNode adds the node 'n' to the front of the list.
func (l *List[V]) PushFrontNode(n *Node[V]) {
	n.Next = l.Front
	n.Prev = nil
	if l.Front != nil {
		l.Front.Prev = n
	} else {
		l.Back = n
	}
	l.Front = n
}

// Remove removes the node 'n' from the list.
func (l *List[V]) Remove(n *Node[V]) {
	if n.Next != nil {
		n.Next.Prev = n.Prev
	} else {
		l.Back = n.Prev
	}
	if n.Prev != nil {
		n.Prev.Next = n.Next
	} else {
		l.Front = n.Next
	}
}

// Each calls 'fn' on every element from this node onward in the list.
func (n *Node[V]) Each(fn func(val V)) {
	node := n
	for node != nil {
		fn(node.Value)
		node = node.Next
	}
}

// EachReverse calls 'fn' on every element from this node backward in the list.
func (n *Node[V]) EachReverse(fn func(val V)) {
	node := n
	for node != nil {
		fn(node.Value)
		node = node.Prev
	}
}

// EachNode calls 'fn' on every node from this node onward in the list.
func (n *Node[V]) EachNode(fn func(n *Node[V])) {
	node := n
	for node != nil {
		fn(node)
		node = node.Next
	}
}

// EachReverseNode calls 'fn' on every node from this node backward in the list.
func (n *Node[V]) EachReverseNode(fn func(n *Node[V])) {
	node := n
	for node != nil {
		fn(node)
		node = node.Prev
	}
}
