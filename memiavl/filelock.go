package memiavl

import "github.com/zbiljic/go-filelock"

type FileLock interface {
	Unlock() error
}

func LockFile(fname string) (FileLock, error) {
	fl, err := filelock.New(fname)
	if err != nil {
		return nil, err
	}
	if _, err := fl.TryLock(); err != nil {
		return nil, err
	}

	return fl, nil
}
