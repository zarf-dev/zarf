package test

type (
	MockReadCloser struct {
		MockData     []byte
		MockReadErr  error
		MockCloseErr error
	}
)

// Read copies a tests expected data and returns an expectedError
func (mrc *MockReadCloser) Read(buf []byte) (n int, err error) {
	numBytes := copy(buf, mrc.MockData)
	mrc.MockData = mrc.MockData[numBytes:len(mrc.MockData)]
	return numBytes, mrc.MockReadErr
}

// Close simply returns the expected close err from a test
func (mrc *MockReadCloser) Close() error { return mrc.MockCloseErr }
