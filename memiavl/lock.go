package memiavl

import (
	"io"
	"os"
	"syscall"
)

// LockOrUnlock grab or release the exclusive lock on an opened file
func LockOrUnlock(file *os.File, lock bool) error {
	op := int16(syscall.F_WRLCK)
	if !lock {
		op = syscall.F_UNLCK
	}
	return syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &syscall.Flock_t{
		Start:  0,
		Len:    0,
		Type:   op,
		Whence: io.SeekStart,
	})
}
