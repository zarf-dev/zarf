//go:build windows

package file

import (
	"os"
)

// getXid is a placeholder for windows file information
func getXid(info os.FileInfo) (uid, gid int) {
	return -1, -1
}
