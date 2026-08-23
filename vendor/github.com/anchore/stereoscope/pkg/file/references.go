package file

// References is a slice of file references (useful for attaching sorting-related methods)
type References []*Reference

func (f References) Len() int {
	return len(f)
}

func (f References) Swap(idx1, idx2 int) {
	f[idx1], f[idx2] = f[idx2], f[idx1]
}

func (f References) Less(idx1, idx2 int) bool {
	return f[idx1].RealPath < f[idx2].RealPath
}

func (f References) Equal(other References) bool {
	if len(f) != len(other) {
		return false
	}
	for i, v := range f {
		if v != other[i] {
			return false
		}
	}
	return true
}
