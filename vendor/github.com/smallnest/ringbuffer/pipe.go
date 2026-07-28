// Copyright 2019 smallnest. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ringbuffer

import "io"

// Pipe creates an asynchronous in-memory pipe compatible with io.Pipe
// It can be used to connect code expecting an [io.Reader]
// with code expecting an [io.Writer].
//
// Reads and Writes will go to the ring buffer.
// Writes will complete as long as the data fits within the ring buffer.
// Reads will attempt to satisfy reads with data from the ring buffer.
// Only if the ring buffer is empty will the read block.
//
// It is safe (and intended) to call Read and Write in parallel with each other or with Close.
func (r *RingBuffer) Pipe() (*PipeReader, *PipeWriter) {
	r.SetBlocking(true)
	pr := PipeReader{pipe: r}
	return &pr, &PipeWriter{pipe: r}
}

// A PipeReader is the read half of a pipe.
type PipeReader struct{ pipe *RingBuffer }

// Read implements the standard Read interface:
// it reads data from the pipe, blocking until a writer
// arrives or the write end is closed.
// If the write end is closed with an error, that error is
// returned as err; otherwise err is io.EOF.
func (r *PipeReader) Read(data []byte) (n int, err error) {
	return r.pipe.Read(data)
}

// Close closes the reader; subsequent writes to the
// write half of the pipe will return the error [io.ErrClosedPipe].
func (r *PipeReader) Close() error {
	r.pipe.setErr(io.ErrClosedPipe, false)
	return nil
}

// CloseWithError closes the reader; subsequent writes
// to the write half of the pipe will return the error err.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
func (r *PipeReader) CloseWithError(err error) error {
	if err == nil {
		return r.Close()
	}
	r.pipe.setErr(err, false)
	return nil
}

// A PipeWriter is the write half of a pipe.
type PipeWriter struct{ pipe *RingBuffer }

// Write implements the standard Write interface:
// it writes data to the pipe.
// The Write will block until all data has been written to the ring buffer.
// If the read end is closed with an error, that err is
// returned as err; otherwise err is [io.ErrClosedPipe].
func (w *PipeWriter) Write(data []byte) (n int, err error) {
	if n, err = w.pipe.Write(data); err == ErrWriteOnClosed {
		// Replace error.
		err = io.ErrClosedPipe
	}
	return n, err
}

// Close closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and EOF.
func (w *PipeWriter) Close() error {
	w.pipe.setErr(io.EOF, false)
	return nil
}

// CloseWithError closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and the error err,
// or EOF if err is nil.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
func (w *PipeWriter) CloseWithError(err error) error {
	if err == nil {
		return w.Close()
	}
	w.pipe.setErr(err, false)
	return nil
}
