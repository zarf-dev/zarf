package squashfslow

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/sylabs/squashfs/internal/decompress"
	"github.com/sylabs/squashfs/internal/metadata"
	"github.com/sylabs/squashfs/internal/toreader"
	"github.com/sylabs/squashfs/low/inode"
)

// The types of compression supported by squashfs
const (
	ZlibCompression = uint16(iota + 1)
	LZMACompression
	LZOCompression
	XZCompression
	LZ4Compression
	ZSTDCompression
)

var (
	ErrorMagic         = errors.New("magic incorrect. probably not reading squashfs archive or archive is corrupted")
	ErrorLog           = errors.New("block log is incorrect. possible corrupted archive")
	ErrorVersion       = errors.New("squashfs version of archive is not 4.0. may be corrupted")
	ErrorNotExportable = errors.New("archive does not have an export table")
)

type Reader struct {
	r           io.ReaderAt
	d           decompress.Decompressor
	Root        Directory
	fragTable   []fragEntry
	idTable     []uint32
	exportTable []uint64
	Superblock  superblock
}

func NewReader(r io.ReaderAt) (rdr *Reader, err error) {
	rdr = new(Reader)
	rdr.r = r
	err = binary.Read(toreader.NewReader(r, 0), binary.LittleEndian, &rdr.Superblock)
	if err != nil {
		return nil, errors.Join(errors.New("failed to read superblock"), err)
	}
	if !rdr.Superblock.ValidMagic() {
		return nil, ErrorMagic
	}
	if !rdr.Superblock.ValidBlockLog() {
		return nil, ErrorLog
	}
	if !rdr.Superblock.ValidVersion() {
		return nil, ErrorVersion
	}
	switch rdr.Superblock.CompType {
	case ZlibCompression:
		rdr.d = decompress.Zlib{}
	case LZMACompression:
		rdr.d = decompress.Lzma{}
	case XZCompression:
		rdr.d = decompress.Xz{}
	case LZ4Compression:
		rdr.d = decompress.Lz4{}
	case ZSTDCompression:
		rdr.d = &decompress.Zstd{}
	default:
		return nil, errors.New("invalid compression type. possible corrupted archive")
	}
	rdr.Root, err = rdr.directoryFromRef(rdr.Superblock.RootInodeRef, "")
	if err != nil {
		return nil, errors.Join(errors.New("failed to read root directory"), err)
	}
	return
}

// Get a uid/gid at the given index. Lazily populates the reader's Id table as necessary.
func (r *Reader) Id(i uint16) (uint32, error) {
	if len(r.idTable) > int(i) {
		return r.idTable[i], nil
	} else if i >= r.Superblock.IdCount {
		return 0, errors.New("id out of bounds")
	}
	// Populate the id table as needed
	var blockNum uint32
	if i != 0 { // If i == 0, we go negatives causing issues with uint32s
		blockNum = uint32(math.Ceil(float64(i+1)/2048)) - 1
	} else {
		blockNum = 0
	}
	blocksRead := len(r.idTable) / 2048
	blocksToRead := int(blockNum) - blocksRead + 1

	var offset uint64
	var idsToRead uint16
	var idsTmp []uint32
	var err error
	var rdr *metadata.Reader
	for i := blocksRead; i < int(blocksRead)+blocksToRead; i++ {
		err = binary.Read(toreader.NewReader(r.r, int64(r.Superblock.IdTableStart)+int64(8*i)), binary.LittleEndian, &offset)
		if err != nil {
			return 0, err
		}
		idsToRead = min(r.Superblock.IdCount-uint16(len(r.idTable)), 2048)
		idsTmp = make([]uint32, idsToRead)
		rdr = metadata.NewReader(toreader.NewReader(r.r, int64(offset)), r.d)
		err = binary.Read(rdr, binary.LittleEndian, &idsTmp)
		rdr.Close()
		if err != nil {
			return 0, err
		}
		r.idTable = append(r.idTable, idsTmp...)
	}
	return r.idTable[i], nil
}

// Get a fragment entry at the given index. Lazily populates the reader's fragment table as necessary.
func (r *Reader) fragEntry(i uint32) (fragEntry, error) {
	if len(r.fragTable) > int(i) {
		return r.fragTable[i], nil
	} else if i >= r.Superblock.FragCount {
		return fragEntry{}, errors.New("fragment out of bounds")
	}
	// Populate the fragment table as needed
	var blockNum uint32
	if i != 0 { // If i == 0, we go negatives causing issues with uint32s
		blockNum = uint32(math.Ceil(float64(i+1)/512)) - 1
	} else {
		blockNum = 0
	}
	blocksRead := len(r.fragTable) / 512
	blocksToRead := int(blockNum) - blocksRead + 1

	var offset uint64
	var fragsToRead uint32
	var fragsTmp []fragEntry
	var err error
	var rdr *metadata.Reader
	for i := blocksRead; i < int(blocksRead)+blocksToRead; i++ {
		err = binary.Read(toreader.NewReader(r.r, int64(r.Superblock.FragTableStart)+int64(8*i)), binary.LittleEndian, &offset)
		if err != nil {
			return fragEntry{}, err
		}
		fragsToRead = min(r.Superblock.FragCount-uint32(len(r.fragTable)), 512)
		fragsTmp = make([]fragEntry, fragsToRead)
		rdr = metadata.NewReader(toreader.NewReader(r.r, int64(offset)), r.d)
		err = binary.Read(rdr, binary.LittleEndian, &fragsTmp)
		rdr.Close()
		if err != nil {
			return fragEntry{}, err
		}
		r.fragTable = append(r.fragTable, fragsTmp...)
	}
	return r.fragTable[i], nil
}

// Get an inode reference at the given index. Lazily populates the reader's export table as necessary.
func (r *Reader) inodeRef(i uint32) (uint64, error) {
	if !r.Superblock.Exportable() {
		return 0, ErrorNotExportable
	}
	if len(r.exportTable) > int(i) {
		return r.exportTable[i], nil
	} else if i >= r.Superblock.InodeCount {
		return 0, errors.New("inode out of bounds")
	}
	// Populate the export table as needed
	var blockNum uint32
	if i != 0 { // If i == 0, we go negatives causing issues with uint32s
		blockNum = uint32(math.Ceil(float64(i+1)/1024)) - 1
	} else {
		blockNum = 0
	}
	blocksRead := len(r.exportTable) / 1024
	blocksToRead := int(blockNum) - blocksRead + 1

	var offset uint64
	var refsToRead uint32
	var refsTmp []uint64
	var err error
	var rdr *metadata.Reader
	for i := blocksRead; i < int(blocksRead)+blocksToRead; i++ {
		err = binary.Read(toreader.NewReader(r.r, int64(r.Superblock.ExportTableStart)+int64(8*i)), binary.LittleEndian, &offset)
		if err != nil {
			return 0, err
		}
		refsToRead = min(r.Superblock.InodeCount-uint32(len(r.exportTable)), 1024)
		refsTmp = make([]uint64, refsToRead)
		rdr = metadata.NewReader(toreader.NewReader(r.r, int64(offset)), r.d)
		err = binary.Read(rdr, binary.LittleEndian, &refsTmp)
		rdr.Close()
		if err != nil {
			return 0, err
		}
		r.exportTable = append(r.exportTable, refsTmp...)
	}
	return r.exportTable[i], nil
}

func (r *Reader) Inode(i uint32) (inode.Inode, error) {
	ref, err := r.inodeRef(i)
	if err != nil {
		return inode.Inode{}, err
	}
	return r.InodeFromRef(ref)
}
