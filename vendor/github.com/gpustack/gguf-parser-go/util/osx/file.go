package osx

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InlineTilde replaces the leading ~ with the home directory.
func InlineTilde(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		hd, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(hd, path[2:])
		}
	}
	return path
}

// Open is similar to os.Open but supports ~ as the home directory.
func Open(path string) (*os.File, error) {
	p := filepath.Clean(path)
	p = InlineTilde(p)
	return os.Open(p)
}

// Exists checks if the given path exists.
func Exists(path string, checks ...func(os.FileInfo) bool) bool {
	p := filepath.Clean(path)
	p = InlineTilde(p)

	stat, err := os.Lstat(p)
	if err != nil {
		return false
	}

	for i := range checks {
		if checks[i] == nil {
			continue
		}

		if !checks[i](stat) {
			return false
		}
	}

	return true
}

// ExistsDir checks if the given path exists and is a directory.
func ExistsDir(path string) bool {
	return Exists(path, func(stat os.FileInfo) bool {
		return stat.Mode().IsDir()
	})
}

// ExistsLink checks if the given path exists and is a symbolic link.
func ExistsLink(path string) bool {
	return Exists(path, func(stat os.FileInfo) bool {
		return stat.Mode()&os.ModeSymlink != 0
	})
}

// ExistsFile checks if the given path exists and is a regular file.
func ExistsFile(path string) bool {
	return Exists(path, func(stat os.FileInfo) bool {
		return stat.Mode().IsRegular()
	})
}

// ExistsSocket checks if the given path exists and is a socket.
func ExistsSocket(path string) bool {
	return Exists(path, func(stat os.FileInfo) bool {
		return stat.Mode()&os.ModeSocket != 0
	})
}

// ExistsDevice checks if the given path exists and is a device.
func ExistsDevice(path string) bool {
	return Exists(path, func(stat os.FileInfo) bool {
		return stat.Mode()&os.ModeDevice != 0
	})
}

// Close closes the given io.Closer without error.
func Close(c io.Closer) {
	if c == nil {
		return
	}
	_ = c.Close()
}

// WriteFile is similar to os.WriteFile but supports ~ as the home directory,
// and also supports the parent directory creation.
func WriteFile(name string, data []byte, perm os.FileMode) error {
	p := filepath.Clean(name)
	p = InlineTilde(p)

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}

	return os.WriteFile(p, data, perm)
}

// CreateFile is similar to os.Create but supports ~ as the home directory,
// and also supports the parent directory creation.
func CreateFile(name string, perm os.FileMode) (*os.File, error) {
	p := filepath.Clean(name)
	p = InlineTilde(p)

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, err
	}

	return os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
}

// OpenFile is similar to os.OpenFile but supports ~ as the home directory,
// and also supports the parent directory creation.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	p := filepath.Clean(name)
	p = InlineTilde(p)

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, err
	}

	return os.OpenFile(p, flag, perm)
}
