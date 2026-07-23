// Copyright 2020 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFilesystem creates directories and files for testing
type TestFilesystem struct {
	// root is the tmp directory
	root string
}

// Setupd creates directories in the test filesystem, returning
// the TestFilesystem.
func Setup(t *testing.T, dirs ...string) TestFilesystem {
	tempDir := "" // Use the default temp directory
	d, err := os.MkdirTemp(tempDir, "test-filesystem")
	if !assert.NoError(t, err) {
		assert.FailNow(t, err.Error())
	}
	err = os.Chdir(d)
	if !assert.NoError(t, err) {
		assert.FailNow(t, err.Error())
	}
	for _, s := range dirs {
		err = os.MkdirAll(s, 0700)
		if !assert.NoError(t, err) {
			assert.FailNow(t, err.Error())
		}
	}
	return TestFilesystem{root: d}
}

// GetRootDir returns the path to the root of the
// test filesystem.
func (tf TestFilesystem) GetRootDir() string {
	return tf.root
}

// WriteFile writes a file in the test filesystem at relative "path"
// containing the bytes "value".
func (tf TestFilesystem) WriteFile(t *testing.T, path string, value []byte) {
	err := os.MkdirAll(filepath.Dir(filepath.Join(tf.root, path)), 0700)
	if !assert.NoError(t, err) {
		assert.FailNow(t, err.Error())
	}
	err = os.WriteFile(filepath.Join(tf.root, path), value, 0600)
	if !assert.NoError(t, err) {
		assert.FailNow(t, err.Error())
	}
}

// Clean deletes the test filesystem.
func (tf TestFilesystem) Clean() {
	os.RemoveAll(tf.root)
}
