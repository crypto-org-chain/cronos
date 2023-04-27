package memiavl

import (
	"errors"
	"os"

	"github.com/ledgerwatch/erigon-lib/mmap"
)

// MmapFile manage the resources of a mmap-ed file
type MmapFile struct {
	file *os.File
	data []byte
	// mmap handle for windows (this is used to close mmap)
	handle *[mmap.MaxMapSize]byte
}

// Open openes the file and create the mmap.
// the mmap is created with flags: PROT_READ, MAP_SHARED, MADV_RANDOM.
func NewMmap(path string) (*MmapFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	data, handle, err := Mmap(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	return &MmapFile{
		file:   file,
		data:   data,
		handle: handle,
	}, nil
}

// Close closes the file and mmap handles
func (m *MmapFile) Close() error {
	return errors.Join(m.file.Close(), mmap.Munmap(m.data, m.handle))
}

// Data returns the mmap-ed buffer
func (m *MmapFile) Data() []byte {
	return m.data
}
