package node

type Stack []Node

func (s *Stack) Size() int {
	return len(*s)
}

func (s *Stack) Pop() Node {
	v := *s
	v, n := v[:len(v)-1], v[len(v)-1]
	*s = v
	return n
}

func (s *Stack) Push(n Node) {
	*s = append(*s, n)
}
