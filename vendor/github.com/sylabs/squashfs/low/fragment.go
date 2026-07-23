package squashfslow

type fragEntry struct {
	Start uint64
	Size  uint32
	_     uint32
}
