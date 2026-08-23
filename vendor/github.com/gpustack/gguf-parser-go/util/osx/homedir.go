package osx

import (
	"os"
	"path/filepath"
	"time"
)

// UserHomeDir is similar to os.UserHomeDir,
// but returns the temp dir if the home dir is not found.
func UserHomeDir() string {
	hd, err := os.UserHomeDir()
	if err != nil {
		hd = filepath.Join(os.TempDir(), time.Now().Format(time.DateOnly))
	}
	return hd
}
