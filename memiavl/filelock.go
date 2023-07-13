package memiavl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
)

var (
	lockedFiles      = make(map[string]struct{})
	lockedFilesMutex sync.Mutex
)

type FileLock struct {
	Fname string
	File  *os.File
}

func (fl *FileLock) Unlock() error {
	lockedFilesMutex.Lock()
	defer lockedFilesMutex.Unlock()

	err := errors.Join(
		LockOrUnlock(fl.File, false),
		fl.File.Close(),
	)

	delete(lockedFiles, fl.Fname)
	fl.File = nil
	return err
}

func LockFile(fname string) (*FileLock, error) {
	lockedFilesMutex.Lock()
	defer lockedFilesMutex.Unlock()

	if _, ok := lockedFiles[fname]; ok {
		return nil, errors.New("lock is already hold by current process")
	}

	lockFile, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	lockedFiles[fname] = struct{}{}

	if err := LockOrUnlock(lockFile, true); err != nil {
		return nil, fmt.Errorf("failed to lock file: %w", err)
	}

	return &FileLock{
		Fname: fname,
		File:  lockFile,
	}, nil
}

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
