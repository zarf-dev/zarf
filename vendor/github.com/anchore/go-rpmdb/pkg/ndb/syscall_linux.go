//go:build linux

package ndb

import (
	"syscall"
)

const (
	syscallLOCK_SH = syscall.LOCK_SH
	syscallLOCK_UN = syscall.LOCK_UN
)

func syscallFlock(fd int, how int) error {
	return syscall.Flock(fd, how)
}
