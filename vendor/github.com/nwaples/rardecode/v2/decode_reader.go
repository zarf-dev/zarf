package rardecode

import (
	"errors"
	"io"
)

const (
	minWindowSize    = 0x40000
	maxQueuedFilters = 8192
)

var (
	ErrTooManyFilters   = errors.New("rardecode: too many filters")
	ErrInvalidFilter    = errors.New("rardecode: invalid filter")
	ErrMultipleDecoders = errors.New("rardecode: multiple decoders in a single archive not supported")
)

// filter functions take a byte slice, the current output offset and
// returns transformed data.
type filter func(b []byte, offset int64) ([]byte, error)

// filterBlock is a block of data to be processed by a filter.
type filterBlock struct {
	length int    // length of block
	offset int    // bytes to be read before start of block
	filter filter // filter function
}

// decoder is the interface for decoding compressed data
type decoder interface {
	init(r byteReader, reset bool, size int64, ver int) // initialize decoder for current file
	fill(dr *decodeReader) error                        // fill window with decoded data
	version() int                                       // decoder version
}

// decodeReader implements io.Reader for decoding compressed data in RAR archives.
type decodeReader struct {
	archiveFile
	tot    int64          // total bytes read from window
	outbuf []byte         // buffered output
	buf    []byte         // filter buffer
	fl     []*filterBlock // list of filters each with offset relative to previous in list
	dec    decoder        // decoder being used to unpack file
	err    error          // current decoder error output
	solid  bool           // archive is solid

	win  []byte // sliding window buffer
	size int    // win length
	r    int    // index in win for reads (beginning)
	w    int    // index in win for writes (end)
}

func (d *decodeReader) init(f archiveFile, ver int, size int, reset, arcSolid bool, unPackedSize int64) error {
	d.outbuf = nil
	d.tot = 0
	d.err = nil
	d.solid = arcSolid
	if reset {
		d.fl = nil
	}
	d.archiveFile = f

	// initialize window
	size = max(size, minWindowSize)
	if size > len(d.win) {
		b := make([]byte, size)
		if reset {
			d.w = 0
		} else if len(d.win) > 0 {
			n := copy(b, d.win[d.w:])
			n += copy(b[n:], d.win[:d.w])
			d.w = n
		}
		d.win = b
		d.size = size
	} else if reset {
		clear(d.win[:])
		d.w = 0
	}
	d.r = d.w

	// initialize decoder
	if d.dec == nil {
		switch ver {
		case decode29Ver:
			d.dec = new(decoder29)
		case decode50Ver, decode70Ver:
			d.dec = new(decoder50)
		case decode20Ver:
			d.dec = new(decoder20)
		default:
			return ErrUnknownDecoder
		}
	} else if d.dec.version() != ver {
		return ErrMultipleDecoders
	}
	d.dec.init(f, reset, unPackedSize, ver)
	return nil
}

// notFull returns if the window is not full
func (d *decodeReader) notFull() bool { return d.w < d.size }

// writeByte writes c to the end of the window
func (d *decodeReader) writeByte(c byte) {
	d.win[d.w] = c
	d.w++
}

// copyBytes copies len bytes at off distance from the end
// to the end of the window.
func (d *decodeReader) copyBytes(length, offset int) {
	length = (d.size + length) % d.size
	wend := min(d.w+length, d.size)
	i := (d.w - offset) % d.size
	if i < 0 {
		// offset can exceed d.size+d.w for a corrupt archive,
		// which makes the modulo negative; wrap it back into
		// the circular window so we never index out of range.
		i += d.size
	}
	if i == d.w {
		d.w = wend
		return
	} else if i > d.w {
		d.w += copy(d.win[d.w:wend], d.win[i:])
		i = 0
	}
	for d.w < wend {
		d.w += copy(d.win[d.w:wend], d.win[i:d.w])
	}
}

// queueFilter adds a filterBlock to the end decodeReader's filters.
func (d *decodeReader) queueFilter(f *filterBlock) error {
	if len(d.fl) >= maxQueuedFilters {
		return ErrTooManyFilters
	}
	// make offset relative to read index (from write index)
	f.offset += d.w - d.r
	// make offset relative to previous filter in list
	for _, fb := range d.fl {
		if f.offset < fb.offset {
			// filter block must not start before previous filter
			return ErrInvalidFilter
		}
		f.offset -= fb.offset
	}
	// offset & length must be < window size
	f.offset %= d.size
	if f.offset < 0 {
		f.offset += d.size
	}
	f.length %= d.size
	if f.length < 0 {
		f.length += d.size
	}
	d.fl = append(d.fl, f)
	return nil
}

func (d *decodeReader) readErr() error {
	err := d.err
	d.err = nil
	return err
}

// fill the decodeReader window
func (d *decodeReader) fill() error {
	if d.err != nil {
		return d.readErr()
	}
	if d.w == d.size {
		// wrap to beginning of buffer
		d.r = 0
		d.w = 0
	}
	d.err = d.dec.fill(d) // fill window using decoder
	if d.w == d.r {
		return d.readErr()
	}
	return nil
}

// bufBytes returns n bytes from the window in a new buffer.
func (d *decodeReader) bufBytes(n int) ([]byte, error) {
	if cap(d.buf) < n {
		d.buf = make([]byte, n)
	}
	// copy into buffer
	ns := 0
	for {
		nn := copy(d.buf[ns:n], d.win[d.r:d.w])
		d.r += nn
		ns += nn
		if ns == n {
			break
		}
		if err := d.fill(); err != nil {
			return nil, err
		}
	}
	return d.buf[:n], nil
}

// processFilters processes any filters valid at the current read index
// and returns the output in outbuf.
func (d *decodeReader) processFilters() ([]byte, error) {
	f := d.fl[0]
	flen := f.length

	// get filter input
	b, err := d.bufBytes(flen)
	if err != nil {
		return nil, err
	}
	for {
		d.fl = d.fl[1:]
		// run filter passing buffer and total bytes read so far
		b, err = f.filter(b, d.tot)
		if err != nil {
			return nil, err
		}
		if len(d.fl) == 0 {
			d.fl = nil
			return b, nil
		}
		// get next filter
		f = d.fl[0]
		if f.offset != 0 {
			// next filter not at current offset
			f.offset -= flen
			return b, nil
		}
		if f.length != len(b) {
			return nil, ErrInvalidFilter
		}
	}
}

// decode fills the window, processes filters and sets outbuf to the current valid output.
func (d *decodeReader) decode() error {
	// fill window if needed
	if d.w == d.r {
		err := d.fill()
		if err != nil {
			return err
		}
	}
	n := d.w - d.r

	// return current unread bytes if there are no filters
	if len(d.fl) == 0 {
		d.outbuf = d.win[d.r:d.w]
		d.r = d.w
		d.tot += int64(n)
		return nil
	}

	// check filters
	f := d.fl[0]
	if f.offset < 0 {
		return ErrInvalidFilter
	}
	if f.offset > 0 {
		// filter not at current read index, output bytes before it
		n = min(f.offset, n)
		d.outbuf = d.win[d.r : d.r+n]
		d.r += n
		f.offset -= n
		d.tot += int64(n)
		return nil
	}

	// process filters at current index
	var err error
	d.outbuf, err = d.processFilters()
	if err != nil {
		return err
	}
	if cap(d.outbuf) > cap(d.buf) {
		// filter returned a larger buffer, cache it
		d.buf = d.outbuf
	}
	d.tot += int64(len(d.outbuf))
	return nil
}

// Read decodes data and stores it in p.
func (d *decodeReader) Read(p []byte) (int, error) {
	if len(d.outbuf) == 0 {
		err := d.decode()
		if err != nil {
			return 0, err
		}
	}
	n := copy(p, d.outbuf)
	d.outbuf = d.outbuf[n:]
	return n, nil
}

func (d *decodeReader) ReadByte() (byte, error) {
	if len(d.outbuf) == 0 {
		err := d.decode()
		if err != nil {
			return 0, err
		}
	}
	b := d.outbuf[0]
	d.outbuf = d.outbuf[1:]
	return b, nil
}

func (d *decodeReader) writeToN(w io.Writer, n int64) (int64, error) {
	if n == 0 {
		return 0, nil
	}
	var tot int64
	var err error
	for tot != n && err == nil {
		if len(d.outbuf) == 0 {
			err = d.decode()
			if err != nil {
				break
			}
		}
		buf := d.outbuf
		if n > 0 {
			todo := n - tot
			if todo < int64(len(buf)) {
				buf = buf[:todo]
			}
		}
		var l int
		l, err = w.Write(buf)
		tot += int64(l)
		d.outbuf = d.outbuf[l:]
	}
	if n < 0 && err == io.EOF {
		err = nil
	}
	return tot, err
}

func (d *decodeReader) WriteTo(w io.Writer) (int64, error) {
	return d.writeToN(w, -1)
}

func (d *decodeReader) nextFile() (*fileBlockList, error) {
	if d.solid {
		_, err := io.Copy(io.Discard, d)
		if err != nil {
			return nil, err
		}
	}
	return d.archiveFile.nextFile()
}
