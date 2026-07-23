package lzo

import (
	"io"
)

// Reader implements io.Reader for LZO-compressed data
type Reader struct {
	src       io.Reader
	decompBuf []byte // buffer to hold decompressed data
	readPos   int    // current read position in decompBuf
	srcBuf    []byte // buffer to accumulate compressed input
	eof       bool   // whether we've reached end of compressed stream
	blockSize int    // size of decompression buffer
}

// NewReader creates a new LZO reader with default 64KB buffer
func NewReader(src io.Reader) *Reader {
	return NewReaderSize(src, 64*1024)
}

// NewReaderSize creates a new LZO reader with specified buffer size
func NewReaderSize(src io.Reader, blockSize int) *Reader {
	return &Reader{
		src:       src,
		blockSize: blockSize,
		srcBuf:    make([]byte, 0, blockSize),
		decompBuf: make([]byte, 0, blockSize),
	}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// if we have data in our decompression buffer, serve from there first
	if r.readPos < len(r.decompBuf) {
		n = copy(p, r.decompBuf[r.readPos:])
		r.readPos += n
		if r.readPos == len(r.decompBuf) {
			// reset buffer when fully consumed
			r.decompBuf = r.decompBuf[:0]
			r.readPos = 0
		}
		return n, nil
	}

	// if we've reached EOF and have no more buffered data
	if r.eof {
		return 0, io.EOF
	}

	// try to decompress the next block
	err = r.decompressNextBlock()
	if err != nil {
		if err == io.EOF {
			r.eof = true
		}
		return 0, err
	}

	// now serve from the newly decompressed data
	if len(r.decompBuf) > 0 {
		n = copy(p, r.decompBuf[r.readPos:])
		r.readPos += n
		if r.readPos == len(r.decompBuf) {
			// reset buffer when fully consumed
			r.decompBuf = r.decompBuf[:0]
			r.readPos = 0
		}
		return n, nil
	}

	return 0, io.EOF
}

// decompressNextBlock reads and decompresses the next LZO block
func (r *Reader) decompressNextBlock() error {
	// read more compressed data if needed

	// TODO: should we handle LZO block headers/framing format here?

	// try to fill source buffer
	if len(r.srcBuf) < 3 { // need at least 3 bytes for valid LZO stream
		tempBuf := make([]byte, r.blockSize)
		n, err := r.src.Read(tempBuf)
		if n > 0 {
			r.srcBuf = append(r.srcBuf, tempBuf[:n]...)
		}
		if err != nil {
			if err == io.EOF && len(r.srcBuf) == 0 {
				return io.EOF
			}
			if err != io.EOF {
				return err
			}
		}
	}

	// if we still don't have enough data, return EOF
	if len(r.srcBuf) < 3 {
		return io.EOF
	}

	// create output buffer for decompression
	outputBuf := make([]byte, r.blockSize)

	// decompress the entire remaining source buffer
	// TODO: this assumes the entire srcBuf contains one LZO stream... for framed formats, you'd need to parse block boundaries
	decompSize, err := Decompress(r.srcBuf, outputBuf)
	if err != nil {
		return err
	}

	// store decompressed data
	r.decompBuf = outputBuf[:decompSize]
	r.readPos = 0

	// clear source buffer since we consumed it all
	r.srcBuf = r.srcBuf[:0]

	return nil
}

func (r *Reader) Close() error {
	if closer, ok := r.src.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
