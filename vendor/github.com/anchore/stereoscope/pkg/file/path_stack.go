package file

type PathStack []Path

func (s *PathStack) Size() int {
	return len(*s)
}

func (s *PathStack) Pop() Path {
	v := *s
	v, n := v[:len(v)-1], v[len(v)-1]
	*s = v
	return n
}

func (s *PathStack) Push(n Path) {
	*s = append(*s, n)
}
