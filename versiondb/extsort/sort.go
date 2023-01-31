// Package extsort implements external sorting algorithm, it has several differnet design choices compared with alternatives like https://github.com/lanrat/extsort:
//   - apply efficient compressions(delta encoding + snappy) to the chunk files to reduce IO cost,
//     since the items are sorted, delta encoding should be effective to it, and snappy is pretty efficient.
//   - chunks are stored in separate temporary files, so the chunk sorting and saving can run in parallel (eats more ram though).
//   - clean interface, user just feed `[]byte` directly, and provides a compare function based on `[]byte`.
package extsort

import (
	"errors"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/golang/snappy"
	"github.com/hashicorp/go-multierror"
)

// ExtSorter implements external sorting.
// It split the inputs into chunks, sort each chunk separately and save to disk file,
// then provide an iterator of sorted items by doing a k-way merge with all the sorted chunk files.
type ExtSorter struct {
	// directory to store temporary chunk files
	tmpDir string
	// target chunk size
	chunkSize  int64
	lesserFunc LesserFunc

	// current chunk
	currentChunk     [][]byte
	currentChunkSize int64

	// manage the chunk goroutines
	chunkWG sync.WaitGroup
	lock    sync.Mutex
	// finished chunk files
	chunkFiles []*os.File
	// chunking goroutine failure messages
	failures []string
}

// New creates a new `ExtSorter`.
func New(tmpDir string, chunkSize int64, lesserFunc LesserFunc) *ExtSorter {
	return &ExtSorter{
		tmpDir:     tmpDir,
		chunkSize:  chunkSize,
		lesserFunc: lesserFunc,
	}
}

// Feed add un-ordered items to the sorter.
func (s *ExtSorter) Feed(item []byte) error {
	if len(item) > math.MaxUint32 {
		return errors.New("item length overflows uint32")
	}

	s.currentChunkSize += int64(len(item)) + 4
	s.currentChunk = append(s.currentChunk, item)

	if s.currentChunkSize >= s.chunkSize {
		return s.sortChunkAndRotate()
	}
	return nil
}

// sortChunkAndRotate sort the current chunk and save to disk.
func (s *ExtSorter) sortChunkAndRotate() error {
	chunkFile, err := os.CreateTemp(s.tmpDir, "sort-chunk-*")
	if err != nil {
		return err
	}

	// rotate chunk
	chunk := s.currentChunk
	s.currentChunk = nil
	s.currentChunkSize = 0

	s.chunkWG.Add(1)
	go func() {
		defer s.chunkWG.Done()
		if err := sortAndSaveChunk(chunk, s.lesserFunc, chunkFile); err != nil {
			chunkFile.Close()
			s.lock.Lock()
			defer s.lock.Unlock()
			s.failures = append(s.failures, err.Error())
			return
		}
		s.lock.Lock()
		defer s.lock.Unlock()
		s.chunkFiles = append(s.chunkFiles, chunkFile)
	}()
	return nil
}

// Finalize wait for all chunking goroutines to finish, and return the merged sorted stream.
func (s *ExtSorter) Finalize() (*MultiWayMerge, error) {
	// handle the pending chunk
	if s.currentChunkSize > 0 {
		if err := s.sortChunkAndRotate(); err != nil {
			return nil, err
		}
	}

	s.chunkWG.Wait()
	if len(s.failures) > 0 {
		return nil, errors.New(strings.Join(s.failures, "\n"))
	}

	streams := make([]NextFunc, len(s.chunkFiles))
	for i, chunkFile := range s.chunkFiles {
		if _, err := chunkFile.Seek(0, 0); err != nil {
			return nil, err
		}
		decoder := NewDeltaDecoder()
		reader := snappy.NewReader(chunkFile)
		streams[i] = func() ([]byte, error) {
			item, err := decoder.Read(reader)
			if err == io.EOF {
				return nil, nil
			}
			return item, err
		}
	}

	return NewMultiWayMerge(streams, s.lesserFunc)
}

// Close closes and remove all the temporary chunk files
func (s *ExtSorter) Close() error {
	var err error
	for _, chunkFile := range s.chunkFiles {
		if merr := chunkFile.Close(); merr != nil {
			err = multierror.Append(err, merr)
		}
		if merr := os.Remove(chunkFile.Name()); merr != nil {
			err = multierror.Append(err, merr)
		}
	}
	return err
}

// sortAndSaveChunk sort the chunk in memory and save to disk in order,
// it applies delta encoding and snappy compression to the items.
func sortAndSaveChunk(chunk [][]byte, lesserFunc LesserFunc, output *os.File) error {
	// sort the chunk and write to file
	sort.Slice(chunk, func(i, j int) bool {
		return lesserFunc(chunk[i], chunk[j])
	})

	writer := snappy.NewBufferedWriter(output)

	encoder := NewDeltaEncoder()
	for _, item := range chunk {
		if err := encoder.Write(writer, item); err != nil {
			return err
		}
	}
	return writer.Flush()
}
