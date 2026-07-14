// Copyright 2019 smallnest. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ringbuffer

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
	"unsafe"
)

var (
	// ErrTooMuchDataToWrite is returned when the data to write is more than the buffer size.
	ErrTooMuchDataToWrite = errors.New("too much data to write")

	// ErrIsFull is returned when the buffer is full and not blocking.
	ErrIsFull = errors.New("ringbuffer is full")

	// ErrIsEmpty is returned when the buffer is empty and not blocking.
	ErrIsEmpty = errors.New("ringbuffer is empty")

	// ErrIsNotEmpty is returned when the buffer is not empty and not blocking.
	ErrIsNotEmpty = errors.New("ringbuffer is not empty")

	// ErrAcquireLock is returned when the lock is not acquired on Try operations.
	ErrAcquireLock = errors.New("unable to acquire lock")

	// ErrWriteOnClosed is returned when write on a closed ringbuffer.
	ErrWriteOnClosed = errors.New("write on closed ringbuffer")

	// ErrReaderClosed is returned when a ReadClosed closed the ringbuffer.
	ErrReaderClosed = errors.New("reader closed")
)

// RingBuffer is a circular buffer that implements io.ReaderWriter interface.
// It operates like a buffered pipe, where data is written to a RingBuffer
// and can be read back from another goroutine.
// It is safe to concurrently read and write RingBuffer.
type RingBuffer struct {
	buf       []byte
	size      int
	r         int // next position to read
	w         int // next position to write
	isFull    bool
	err       error
	block     bool
	timeout   time.Duration
	mu        sync.Mutex
	wg        sync.WaitGroup
	readCond  *sync.Cond // Signaled when data has been read.
	writeCond *sync.Cond // Signaled when data has been written.
}

// New returns a new RingBuffer whose buffer has the given size.
func New(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// NewBuffer returns a new RingBuffer whose buffer is provided.
func NewBuffer(b []byte) *RingBuffer {
	return &RingBuffer{
		buf:  b,
		size: len(b),
	}
}

// SetBlocking sets the blocking mode of the ring buffer.
// If block is true, Read and Write will block when there is no data to read or no space to write.
// If block is false, Read and Write will return ErrIsEmpty or ErrIsFull immediately.
// By default, the ring buffer is not blocking.
// This setting should be called before any Read or Write operation or after a Reset.
func (r *RingBuffer) SetBlocking(block bool) *RingBuffer {
	r.block = block
	if block {
		r.readCond = sync.NewCond(&r.mu)
		r.writeCond = sync.NewCond(&r.mu)
	}
	return r
}

// WithCancel sets a context to cancel the ring buffer.
// When the context is canceled, the ring buffer will be closed with the context error.
// A goroutine will be started and run until the provided context is canceled.
func (r *RingBuffer) WithCancel(ctx context.Context) *RingBuffer {
	go func() {
		select {
		case <-ctx.Done():
			r.CloseWithError(ctx.Err())
		}
	}()
	return r
}

// WithTimeout will set a blocking read/write timeout.
// If no reads or writes occur within the timeout,
// the ringbuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts (default).
func (r *RingBuffer) WithTimeout(d time.Duration) *RingBuffer {
	r.mu.Lock()
	r.timeout = d
	r.mu.Unlock()
	return r
}

func (r *RingBuffer) setErr(err error, locked bool) error {
	if !locked {
		r.mu.Lock()
		defer r.mu.Unlock()
	}
	if r.err != nil && r.err != io.EOF {
		return r.err
	}

	switch err {
	// Internal errors are transient
	case nil, ErrIsEmpty, ErrIsFull, ErrAcquireLock, ErrTooMuchDataToWrite, ErrIsNotEmpty:
		return err
	default:
		r.err = err
		if r.block {
			r.readCond.Broadcast()
			r.writeCond.Broadcast()
		}
	}
	return err
}

func (r *RingBuffer) readErr(locked bool) error {
	if !locked {
		r.mu.Lock()
		defer r.mu.Unlock()
	}
	if r.err != nil {
		if r.err == io.EOF {
			if r.w == r.r && !r.isFull {
				return io.EOF
			}
			return nil
		}
		return r.err
	}
	return nil
}

// Read reads up to len(p) bytes into p. It returns the number of bytes read (0 <= n <= len(p)) and any error encountered.
// Even if Read returns n < len(p), it may use all of p as scratch space during the call.
// If some data is available but not len(p) bytes, Read conventionally returns what is available instead of waiting for more.
// When Read encounters an error or end-of-file condition after successfully reading n > 0 bytes, it returns the number of bytes read.
// It may return the (non-nil) error from the same call or return the error (and n == 0) from a subsequent call.
// Callers should always process the n > 0 bytes returned before considering the error err.
// Doing so correctly handles I/O errors that happen after reading some bytes and also both of the allowed EOF behaviors.
func (r *RingBuffer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, r.readErr(false)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.readErr(true); err != nil {
		return 0, err
	}

	r.wg.Add(1)
	defer r.wg.Done()
	n, err = r.read(p)
	for err == ErrIsEmpty && r.block {
		if !r.waitWrite() {
			return 0, context.DeadlineExceeded
		}
		if err = r.readErr(true); err != nil {
			break
		}
		n, err = r.read(p)
	}
	if r.block && n > 0 {
		r.readCond.Broadcast()
	}
	return n, err
}

// TryRead read up to len(p) bytes into p like Read, but it is never blocking.
// If it does not succeed to acquire the lock, it returns ErrAcquireLock.
func (r *RingBuffer) TryRead(p []byte) (n int, err error) {
	ok := r.mu.TryLock()
	if !ok {
		return 0, ErrAcquireLock
	}
	defer r.mu.Unlock()
	if err := r.readErr(true); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, r.readErr(true)
	}

	n, err = r.read(p)
	if r.block && n > 0 {
		r.readCond.Broadcast()
	}
	return n, err
}

func (r *RingBuffer) read(p []byte) (n int, err error) {
	if r.w == r.r && !r.isFull {
		return 0, ErrIsEmpty
	}

	if r.w > r.r {
		n = r.w - r.r
		if n > len(p) {
			n = len(p)
		}
		copy(p, r.buf[r.r:r.r+n])
		r.r = (r.r + n) % r.size
		return
	}

	n = r.size - r.r + r.w
	if n > len(p) {
		n = len(p)
	}

	if r.r+n <= r.size {
		copy(p, r.buf[r.r:r.r+n])
	} else {
		c1 := r.size - r.r
		copy(p, r.buf[r.r:r.size])
		c2 := n - c1
		copy(p[c1:], r.buf[0:c2])
	}
	r.r = (r.r + n) % r.size

	r.isFull = false

	return n, r.readErr(true)
}

// waitRead will wait for a read unblock.
// Returns true if a read may have happened.
// Returns false if waited longer than timeout.
// Must be called when locked and returns locked.
func (r *RingBuffer) waitRead() (ok bool) {
	if r.timeout <= 0 {
		r.readCond.Wait()
		return true
	}
	start := time.Now()
	defer time.AfterFunc(r.timeout, r.readCond.Broadcast).Stop()

	r.readCond.Wait()
	if time.Since(start) >= r.timeout {
		r.setErr(context.DeadlineExceeded, true)
		return false
	}
	return true
}

// ReadByte reads and returns the next byte from the input or ErrIsEmpty.
func (r *RingBuffer) ReadByte() (b byte, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err = r.readErr(true); err != nil {
		return 0, err
	}
	for r.w == r.r && !r.isFull {
		if r.block {
			if !r.waitWrite() {
				return 0, context.DeadlineExceeded
			}
			err = r.readErr(true)
			if err != nil {
				return 0, err
			}
			continue
		}
		return 0, ErrIsEmpty
	}
	b = r.buf[r.r]
	r.r++
	if r.r == r.size {
		r.r = 0
	}

	r.isFull = false
	return b, r.readErr(true)
}

// Write writes len(p) bytes from p to the underlying buf.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// If blocking n < len(p) will be returned only if an error occurred.
// Write returns a non-nil error if it returns n < len(p).
// Write will not modify the slice data, even temporarily.
func (r *RingBuffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, r.setErr(nil, false)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.err; err != nil {
		if err == io.EOF {
			err = ErrWriteOnClosed
		}
		return 0, err
	}
	wrote := 0
	for len(p) > 0 {
		n, err = r.write(p)
		wrote += n
		if !r.block || err == nil {
			break
		}
		err = r.setErr(err, true)
		if r.block && (err == ErrIsFull || err == ErrTooMuchDataToWrite) {
			r.writeCond.Broadcast()
			r.waitRead()
			p = p[n:]
			err = nil
			continue
		}
		break
	}
	if r.block && wrote > 0 {
		r.writeCond.Broadcast()
	}

	return wrote, r.setErr(err, true)
}

// waitWrite will wait for a write event.
// Returns true if a write may have happened.
// Returns false if waited longer than timeout.
// Must be called when locked and returns locked.
func (r *RingBuffer) waitWrite() (ok bool) {
	if r.timeout <= 0 {
		r.writeCond.Wait()
		return true
	}
	start := time.Now()
	defer time.AfterFunc(r.timeout, r.writeCond.Broadcast).Stop()

	r.writeCond.Wait()
	if time.Since(start) >= r.timeout {
		r.setErr(context.DeadlineExceeded, true)
		return false
	}
	return true
}

// ReadFrom will fulfill the write side of the ringbuffer.
// This will do writes directly into the buffer,
// therefore avoiding a mem-copy when using the Write.
//
// ReadFrom will not automatically close the buffer even after returning.
// For that call CloseWriter().
//
// ReadFrom reads data from r until EOF or error.
// The return value n is the number of bytes read.
// Any error except EOF encountered during the read is also returned,
// and the error will cause the Read side to fail as well.
// ReadFrom only available in blocking mode.
func (r *RingBuffer) ReadFrom(rd io.Reader) (n int64, err error) {
	if !r.block {
		return 0, errors.New("RingBuffer: ReadFrom only available in blocking mode")
	}
	zeroReads := 0
	r.mu.Lock()
	defer r.mu.Unlock()
	for {
		if err = r.readErr(true); err != nil {
			return n, err
		}
		if r.isFull {
			// Wait for a read
			if !r.waitRead() {
				return 0, context.DeadlineExceeded
			}
			continue
		}

		var toRead []byte
		if r.w >= r.r {
			// After reader, read until end of buffer
			toRead = r.buf[r.w:]
		} else {
			// Before reader, read until reader.
			toRead = r.buf[r.w:r.r]
		}
		// Unlock while reading
		r.mu.Unlock()
		nr, rerr := rd.Read(toRead)
		r.mu.Lock()
		if rerr != nil && rerr != io.EOF {
			err = r.setErr(err, true)
			break
		}
		if nr == 0 && rerr == nil {
			zeroReads++
			if zeroReads >= 100 {
				err = r.setErr(io.ErrNoProgress, true)
			}
			continue
		}
		zeroReads = 0
		r.w += nr
		if r.w == r.size {
			r.w = 0
		}
		r.isFull = r.r == r.w && nr > 0
		n += int64(nr)
		r.writeCond.Broadcast()
		if rerr == io.EOF {
			// We do not close.
			break
		}
	}
	return n, err
}

// WriteTo writes data to w until there's no more data to write or
// when an error occurs. The return value n is the number of bytes
// written. Any error encountered during the write is also returned.
//
// If a non-nil error is returned the write side will also see the error.
func (r *RingBuffer) WriteTo(w io.Writer) (n int64, err error) {
	if !r.block {
		return 0, errors.New("RingBuffer: WriteTo only available in blocking mode")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Don't write more than half, to unblock reads earlier.
	maxWrite := len(r.buf) / 2
	// But write at least 8K if possible
	if maxWrite < 8<<10 {
		maxWrite = len(r.buf)
	}
	for {
		if err = r.readErr(true); err != nil {
			break
		}
		if r.r == r.w && !r.isFull {
			// Wait for a write to make space
			if !r.waitWrite() {
				return 0, context.DeadlineExceeded
			}
			continue
		}

		var toWrite []byte
		if r.r >= r.w {
			// After writer, we can write until end of buffer
			toWrite = r.buf[r.r:]
		} else {
			// Before reader, we can read until writer.
			toWrite = r.buf[r.r:r.w]
		}
		if len(toWrite) > maxWrite {
			toWrite = toWrite[:maxWrite]
		}
		// Unlock while reading
		r.mu.Unlock()
		nr, werr := w.Write(toWrite)
		r.mu.Lock()
		if werr != nil {
			err = r.setErr(werr, true)
			break
		}
		if nr != len(toWrite) {
			err = r.setErr(io.ErrShortWrite, true)
			break
		}
		r.r += nr
		if r.r == r.size {
			r.r = 0
		}
		r.isFull = false
		n += int64(nr)
		r.readCond.Broadcast()
	}
	if err == io.EOF {
		err = nil
	}
	return n, err
}

// Copy will pipe all data from the reader to the writer through the ringbuffer.
// The ringbuffer will switch to blocking mode.
// Reads and writes will be done async.
// No internal mem-copies are used for the transfer.
//
// Calling CloseWithError will cancel the transfer and make the function return when
// any ongoing reads or writes have finished.
//
// Calling Read or Write functions concurrently with running this will lead to unpredictable results.
func (r *RingBuffer) Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	r.SetBlocking(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.ReadFrom(src)
		r.CloseWriter()
	}()
	defer wg.Wait()
	return r.WriteTo(dst)
}

// TryWrite writes len(p) bytes from p to the underlying buf like Write, but it is not blocking.
// If it does not succeed to acquire the lock, it returns ErrAcquireLock.
func (r *RingBuffer) TryWrite(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, r.setErr(nil, false)
	}
	ok := r.mu.TryLock()
	if !ok {
		return 0, ErrAcquireLock
	}
	defer r.mu.Unlock()
	if err := r.err; err != nil {
		if err == io.EOF {
			err = ErrWriteOnClosed
		}
		return 0, err
	}

	n, err = r.write(p)
	if r.block && n > 0 {
		r.writeCond.Broadcast()
	}
	return n, r.setErr(err, true)
}

func (r *RingBuffer) write(p []byte) (n int, err error) {
	if r.isFull {
		return 0, ErrIsFull
	}

	var avail int
	if r.w >= r.r {
		avail = r.size - r.w + r.r
	} else {
		avail = r.r - r.w
	}

	if len(p) > avail {
		err = ErrTooMuchDataToWrite
		p = p[:avail]
	}
	n = len(p)

	if r.w >= r.r {
		c1 := r.size - r.w
		if c1 >= n {
			copy(r.buf[r.w:], p)
			r.w += n
		} else {
			copy(r.buf[r.w:], p[:c1])
			c2 := n - c1
			copy(r.buf[0:], p[c1:])
			r.w = c2
		}
	} else {
		copy(r.buf[r.w:], p)
		r.w += n
	}

	if r.w == r.size {
		r.w = 0
	}
	if r.w == r.r {
		r.isFull = true
	}

	return n, err
}

// WriteByte writes one byte into buffer, and returns ErrIsFull if the buffer is full.
func (r *RingBuffer) WriteByte(c byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.err; err != nil {
		if err == io.EOF {
			err = ErrWriteOnClosed
		}
		return err
	}
	err := r.writeByte(c)
	for err == ErrIsFull && r.block {
		if !r.waitRead() {
			return context.DeadlineExceeded
		}
		err = r.setErr(r.writeByte(c), true)
	}
	if r.block && err == nil {
		r.writeCond.Broadcast()
	}
	return err
}

// TryWriteByte writes one byte into buffer without blocking.
// If it does not succeed to acquire the lock, it returns ErrAcquireLock.
func (r *RingBuffer) TryWriteByte(c byte) error {
	ok := r.mu.TryLock()
	if !ok {
		return ErrAcquireLock
	}
	defer r.mu.Unlock()
	if err := r.err; err != nil {
		if err == io.EOF {
			err = ErrWriteOnClosed
		}
		return err
	}

	err := r.writeByte(c)
	if err == nil && r.block {
		r.writeCond.Broadcast()
	}
	return err
}

func (r *RingBuffer) writeByte(c byte) error {
	if r.err != nil {
		return r.err
	}
	if r.w == r.r && r.isFull {
		return ErrIsFull
	}
	r.buf[r.w] = c
	r.w++

	if r.w == r.size {
		r.w = 0
	}
	if r.w == r.r {
		r.isFull = true
	}

	return nil
}

// Length returns the number of bytes that can be read without blocking.
func (r *RingBuffer) Length() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.w == r.r {
		if r.isFull {
			return r.size
		}
		return 0
	}

	if r.w > r.r {
		return r.w - r.r
	}

	return r.size - r.r + r.w
}

// Capacity returns the size of the underlying buffer.
func (r *RingBuffer) Capacity() int {
	return r.size
}

// Free returns the number of bytes that can be written without blocking.
func (r *RingBuffer) Free() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.w == r.r {
		if r.isFull {
			return 0
		}
		return r.size
	}

	if r.w < r.r {
		return r.r - r.w
	}

	return r.size - r.w + r.r
}

// WriteString writes the contents of the string s to buffer, which accepts a slice of bytes.
func (r *RingBuffer) WriteString(s string) (n int, err error) {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	buf := *(*[]byte)(unsafe.Pointer(&h))
	return r.Write(buf)
}

// Bytes returns all available read bytes.
// It does not move the read pointer and only copy the available data.
// If the dst is big enough, it will be used as destination,
// otherwise a new buffer will be allocated.
func (r *RingBuffer) Bytes(dst []byte) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	getDst := func(n int) []byte {
		if cap(dst) < n {
			return make([]byte, n)
		}
		return dst[:n]
	}

	if r.w == r.r {
		if r.isFull {
			buf := getDst(r.size)
			copy(buf, r.buf[r.r:])
			copy(buf[r.size-r.r:], r.buf[:r.w])
			return buf
		}
		return nil
	}

	if r.w > r.r {
		buf := getDst(r.w - r.r)
		copy(buf, r.buf[r.r:r.w])
		return buf
	}

	n := r.size - r.r + r.w
	buf := getDst(n)

	if r.r+n < r.size {
		copy(buf, r.buf[r.r:r.r+n])
	} else {
		c1 := r.size - r.r
		copy(buf, r.buf[r.r:r.size])
		c2 := n - c1
		copy(buf[c1:], r.buf[0:c2])
	}

	return buf
}

// IsFull returns true when the ringbuffer is full.
func (r *RingBuffer) IsFull() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.isFull
}

// IsEmpty returns true when the ringbuffer is empty.
func (r *RingBuffer) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return !r.isFull && r.w == r.r
}

// CloseWithError closes the writer; reads will return
// no bytes and the error err, or EOF if err is nil.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
func (r *RingBuffer) CloseWithError(err error) {
	if err == nil {
		err = io.EOF
	}
	r.setErr(err, false)
}

// CloseWriter closes the writer.
// Reads will return any remaining bytes and io.EOF.
func (r *RingBuffer) CloseWriter() {
	r.setErr(io.EOF, false)
}

// Flush waits for the buffer to be empty and fully read.
// If not blocking ErrIsNotEmpty will be returned if the buffer still contains data.
func (r *RingBuffer) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for r.w != r.r || r.isFull {
		err := r.readErr(true)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return err
		}
		if !r.block {
			return ErrIsNotEmpty
		}
		if !r.waitRead() {
			return context.DeadlineExceeded
		}
	}

	err := r.readErr(true)
	if err == io.EOF {
		return nil
	}
	return err
}

// Reset the read pointer and writer pointer to zero.
func (r *RingBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Set error so any readers/writers will return immediately.
	r.setErr(errors.New("reset called"), true)
	if r.block {
		r.readCond.Broadcast()
		r.writeCond.Broadcast()
	}

	// Unlock the mutex so readers/writers can finish.
	r.mu.Unlock()
	r.wg.Wait()
	r.mu.Lock()
	r.r = 0
	r.w = 0
	r.err = nil
	r.isFull = false
}

// WriteCloser returns a WriteCloser that writes to the ring buffer.
// When the returned WriteCloser is closed, it will wait for all data to be read before returning.
func (r *RingBuffer) WriteCloser() io.WriteCloser {
	return &writeCloser{RingBuffer: r}
}

type writeCloser struct {
	*RingBuffer
}

// Close provides a close method for the WriteCloser.
func (wc *writeCloser) Close() error {
	wc.CloseWriter()
	return wc.Flush()
}

// ReadCloser returns a io.ReadCloser that reads to the ring buffer.
// When the returned ReadCloser is closed, ErrReaderClosed will be returned on any writes done afterwards.
func (r *RingBuffer) ReadCloser() io.ReadCloser {
	return &readCloser{RingBuffer: r}
}

type readCloser struct {
	*RingBuffer
}

// Close provides a close method for the ReadCloser.
func (rc *readCloser) Close() error {
	rc.CloseWithError(ErrReaderClosed)
	err := rc.readErr(false)
	if err == ErrReaderClosed {
		err = nil
	}
	return err
}

// Peek reads up to len(p) bytes into p without moving the read pointer.
func (r *RingBuffer) Peek(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, r.readErr(false)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.readErr(true); err != nil {
		return 0, err
	}

	return r.peek(p)
}

func (r *RingBuffer) peek(p []byte) (n int, err error) {
	if r.w == r.r && !r.isFull {
		return 0, ErrIsEmpty
	}

	if r.w > r.r {
		n = r.w - r.r
		if n > len(p) {
			n = len(p)
		}
		copy(p, r.buf[r.r:r.r+n])
		return
	}

	n = r.size - r.r + r.w
	if n > len(p) {
		n = len(p)
	}

	if r.r+n <= r.size {
		copy(p, r.buf[r.r:r.r+n])
	} else {
		c1 := r.size - r.r
		copy(p, r.buf[r.r:r.size])
		c2 := n - c1
		copy(p[c1:], r.buf[0:c2])
	}

	return n, r.readErr(true)
}
