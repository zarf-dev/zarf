package rardecode

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
)

const (
	maxSfxSize   = 0x100000 // maximum number of bytes to read when searching for RAR signature
	sigPrefix    = "Rar!\x1A\x07"
	sigPrefixLen = len(sigPrefix)

	maxEmptyReads = 100

	minBufSize     = 32
	defaultBufSize = 4096
)

var (
	ErrNoSig        = errors.New("rardecode: RAR signature not found")
	ErrNegativeRead = errors.New("rardecode: negative read from Reader")
)

type bufVolumeReader struct {
	r   io.Reader
	sr  io.Seeker
	buf []byte
	i   int
	n   int
	off int64
	err error
	ver int
}

func (br *bufVolumeReader) readErr() error {
	err := br.err
	br.err = nil
	return err
}

func (br *bufVolumeReader) fill() error {
	if br.err != nil {
		return br.readErr()
	}
	br.i = 0
	for i := 0; i < maxEmptyReads; i++ {
		br.n, br.err = br.r.Read(br.buf)
		if br.n > 0 {
			return nil
		}
		if br.n < 0 {
			return ErrNegativeRead
		}
		if br.err != nil {
			return br.readErr()
		}
	}
	return io.ErrNoProgress
}

func (br *bufVolumeReader) canSeek() bool {
	return br.sr != nil
}

func (br *bufVolumeReader) seek(offset int64) error {
	if br.sr == nil {
		return fs.ErrInvalid
	}
	start := br.off - int64(br.i)
	end := start + int64(br.n)
	if offset >= start && offset <= end {
		diff := offset - br.off
		br.off += diff
		br.i += int(diff)
		return nil
	}
	_, err := br.sr.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}
	br.i = 0
	br.n = 0
	br.off = offset
	return nil
}

func (br *bufVolumeReader) Read(p []byte) (int, error) {
	if br.i == br.n {
		err := br.fill()
		if err != nil {
			return 0, err
		}
	}
	n := copy(p, br.buf[br.i:br.n])
	br.i += n
	br.off += int64(n)
	return n, nil
}

func (br *bufVolumeReader) ReadByte() (byte, error) {
	if br.i == br.n {
		err := br.fill()
		if err != nil {
			return 0, err
		}
	}
	c := br.buf[br.i]
	br.i++
	br.off++
	return c, nil
}

func (br *bufVolumeReader) Discard(n int64) error {
	buffered := int64(br.n - br.i)
	if buffered >= n {
		br.i += int(n)
		br.off += n
		return nil
	}
	// empty buffer
	n -= buffered
	br.i = 0
	br.n = 0
	br.off += buffered
	// try seek
	if sr, ok := br.r.(io.Seeker); ok {
		_, err := sr.Seek(n, io.SeekCurrent)
		if err != nil {
			return err
		}
		br.off += n
		return nil
	}
	// copy to discard writer
	written, err := io.CopyN(io.Discard, br.r, n)
	br.off += written
	return err
}

// findSig searches for the RAR signature and version at the beginning of a file.
// It searches no more than maxSfxSize bytes from the file start.
func (br *bufVolumeReader) findSig() (int, error) {
	for br.off <= maxSfxSize {
		if br.i == br.n {
			err := br.fill()
			if err != nil {
				return 0, err
			}
		}
		n := bytes.IndexByte(br.buf[br.i:br.n], sigPrefix[0])
		if n < 0 {
			br.off += int64(n)
			continue
		}
		br.i += n
		br.off += int64(n)
		// ensure enough bytes available in buffer
		buffered := br.n - br.i
		if buffered < sigPrefixLen+2 {
			br.n = copy(br.buf, br.buf[br.i:br.n])
			br.i = 0
			l, err := io.ReadAtLeast(br.r, br.buf[br.n:], sigPrefixLen+2-buffered)
			br.n += l
			if err != nil {
				if errors.Is(err, io.ErrUnexpectedEOF) {
					err = ErrNoSig
				}
				return 0, err
			}
		}
		if !bytes.HasPrefix(br.buf[br.i:br.n], []byte(sigPrefix)) {
			br.i++
			br.off++
			continue
		}
		br.i += sigPrefixLen
		br.off += int64(sigPrefixLen)

		ver := int(br.buf[br.i])
		if ver == 0 {
			br.i++
			br.off++
		} else if br.buf[br.i+1] == 0 {
			br.i += 2
			br.off += 2
		} else {
			continue
		}
		return ver, nil
	}
	return 0, ErrNoSig
}

func (br *bufVolumeReader) Reset(r io.Reader) error {
	br.r = r
	br.sr, _ = r.(io.Seeker)
	br.i = 0
	br.n = 0
	br.off = 0
	ver, err := br.findSig()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return ErrNoSig
		}
		return err
	}
	br.ver = ver
	return nil
}

func newBufVolumeReader(r io.Reader, size int) (*bufVolumeReader, error) {
	if size == 0 {
		size = defaultBufSize
	} else {
		size = max(minBufSize, size)
	}
	br := &bufVolumeReader{
		buf: make([]byte, size),
	}
	err := br.Reset(r)
	if err != nil {
		return nil, err
	}
	return br, nil
}
