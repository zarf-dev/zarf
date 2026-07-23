package squashfslow

import "math"

type superblock struct {
	Magic            uint32
	InodeCount       uint32
	ModTime          uint32
	BlockSize        uint32
	FragCount        uint32
	CompType         uint16
	BlockLog         uint16
	Flags            uint16
	IdCount          uint16
	VerMaj           uint16
	VerMin           uint16
	RootInodeRef     uint64
	Size             uint64
	IdTableStart     uint64
	XattrTableStart  uint64
	InodeTableStart  uint64
	DirTableStart    uint64
	FragTableStart   uint64
	ExportTableStart uint64
}

func (s superblock) ValidMagic() bool {
	return s.Magic == 0x73717368
}

func (s superblock) ValidBlockLog() bool {
	return s.BlockLog == uint16(math.Log2(float64(s.BlockSize)))
}

func (s superblock) ValidVersion() bool {
	return s.VerMaj == 4 && s.VerMin == 0
}

func (s superblock) UncompressedInodes() bool {
	return s.Flags&0x1 == 0x1
}

func (s superblock) UncompressedData() bool {
	return s.Flags&0x2 == 0x2
}
func (s superblock) UncompressedFragments() bool {
	return s.Flags&0x8 == 0x8
}

func (s superblock) NoFragments() bool {
	return s.Flags&0x10 == 0x10
}

func (s superblock) AlwaysFragment() bool {
	return s.Flags&0x20 == 0x20
}

func (s superblock) Duplicates() bool {
	return s.Flags&0x40 == 0x40
}

func (s superblock) Exportable() bool {
	return s.Flags&0x80 == 0x80
}

func (s superblock) UncompressedXattrs() bool {
	return s.Flags&0x100 == 0x100
}

func (s superblock) NoXattrs() bool {
	return s.Flags&0x200 == 0x200
}

func (s superblock) CompressionOptions() bool {
	return s.Flags&0x400 == 0x400
}

func (s superblock) UncompressedIDs() bool {
	return s.Flags&0x800 == 0x800
}
