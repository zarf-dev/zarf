// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package mocks contains all the mocks used in Zarf tests
package mocks

// MockReadCloser is a mock for the go ReadCloser object
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
