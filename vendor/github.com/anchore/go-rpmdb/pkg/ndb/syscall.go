//go:build !linux

package ndb

const (
	syscallLOCK_SH = 0
	syscallLOCK_UN = 0
)

func syscallFlock(fd int, how int) error {
	return nil
}
