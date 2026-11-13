package rardecode

import (
	"encoding/binary"
	"errors"
	"hash"
)

const (
	_ = iota
	decode20Ver
	decode29Ver
	decode50Ver
	decode70Ver

	archiveVersion15 = 0
	archiveVersion50 = 1
)

var (
	ErrCorruptBlockHeader    = errors.New("rardecode: corrupt block header")
	ErrCorruptFileHeader     = errors.New("rardecode: corrupt file header")
	ErrBadHeaderCRC          = errors.New("rardecode: bad header crc")
	ErrUnknownDecoder        = errors.New("rardecode: unknown decoder version")
	ErrDecoderOutOfData      = errors.New("rardecode: decoder expected more data than is in packed file")
	ErrArchiveEncrypted      = errors.New("rardecode: archive encrypted, password required")
	ErrArchivedFileEncrypted = errors.New("rardecode: archived files encrypted, password required")
	ErrMultiVolume           = errors.New("rardecode: multi-volume archive continues in next file")
	errVolumeOrArchiveEnd    = errors.New("rardecode: archive or volume end")
)

type readBuf []byte

func (b *readBuf) byte() byte {
	v := (*b)[0]
	*b = (*b)[1:]
	return v
}

func (b *readBuf) uint16() uint16 {
	v := binary.LittleEndian.Uint16(*b)
	*b = (*b)[2:]
	return v
}

func (b *readBuf) uint32() uint32 {
	v := binary.LittleEndian.Uint32(*b)
	*b = (*b)[4:]
	return v
}

func (b *readBuf) uint64() uint64 {
	v := binary.LittleEndian.Uint64(*b)
	*b = (*b)[8:]
	return v
}

func (b *readBuf) bytes(n int) []byte {
	v := (*b)[:n]
	*b = (*b)[n:]
	return v
}

func (b *readBuf) uvarint() uint64 {
	n, cnt := binary.Uvarint(*b)
	if cnt == 0 {
		cnt = len(*b)
	}
	*b = (*b)[cnt:]
	return n
}

// fileBlockHeader represents a file block in a RAR archive.
// Files may comprise one or more file blocks.
// Solid files retain decode tables and dictionary from previous solid files in the archive.
type fileBlockHeader struct {
	first     bool             // first block in file
	last      bool             // last block in file
	arcSolid  bool             // archive is solid
	dataOff   int64            // offset to data for file block in archive volume
	packedOff int64            // offset to data in packed file
	blocknum  int              // number for current block in file
	volnum    int              // archive volume number
	winSize   int64            // decode window size
	hash      func() hash.Hash // hash used for file checksum
	hashKey   []byte           // optional hmac key to be used calculate file checksum
	sum       []byte           // expected checksum for file contents
	decVer    int              // decoder to use for file
	key       []byte           // key for AES, non-empty if file encrypted
	iv        []byte           // iv for AES, non-empty if file encrypted
	errs      []error          // errors to return when trying to read file body
	FileHeader
}

// archiveBlockReader returns the next fileBlockHeader in an archive volume.
type archiveBlockReader interface {
	init(br *bufVolumeReader) (int, error)                   // init volume and returns optional (>=0) volume number
	nextBlock(br *bufVolumeReader) (*fileBlockHeader, error) // reads the volume and returns the next fileBlockHeader
	useOldNaming() bool
}
