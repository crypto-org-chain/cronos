package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/alitto/pond"
	"github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/iavl"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/cmd/cronosd/open_db"
	"github.com/crypto-org-chain/cronos/versiondb/extsort"
	"github.com/crypto-org-chain/cronos/versiondb/memiavl"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

var (
	int64Size = 8
	hashSize  = sha256.Size

	nodeKeyFormat = iavl.NewKeyFormat('n', hashSize)  // n<hash>
	rootKeyFormat = iavl.NewKeyFormat('r', int64Size) // r<version>
)

func ConvertSnapshotToSST(appCreator types.AppCreator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert-snapshot-to-sst snapshot-dir sst-dir",
		Short: "convert memiavl snapshot to sst file, ready to be ingested into `application.db`",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sstFileSize, err := cmd.Flags().GetUint64(flagSSTFileSize)
			if err != nil {
				return err
			}
			sorterChunkSize, err := cmd.Flags().GetUint64(flagSorterChunkSize)
			if err != nil {
				return err
			}
			concurrency, err := cmd.Flags().GetInt(flagConcurrency)
			if err != nil {
				return err
			}

			stores, err := GetStoreNames(cmd, appCreator)
			if err != nil {
				return err
			}

			snapshotDir := args[0]
			sstDir := args[1]
			if err := os.MkdirAll(sstDir, os.ModePerm); err != nil {
				return err
			}

			// create fixed size task pool with big enough buffer.
			pool := pond.New(concurrency, 0)
			defer pool.StopAndWait()

			group, _ := pool.GroupContext(context.Background())
			for _, store := range stores {
				store := store
				group.Submit(func() error {
					return oneStore(store, snapshotDir, sstDir, sstFileSize, sorterChunkSize)
				})
			}

			return group.Wait()
		},
	}

	cmd.Flags().Uint64(flagSSTFileSize, DefaultSSTFileSize, "the target sst file size, note the actual file size may be larger because sst files must be split on different key names")
	cmd.Flags().String(flagStores, "", "list of store names, default to the current store list in application")
	cmd.Flags().Uint64(flagSorterChunkSize, DefaultSorterChunkSize, "uncompressed chunk size for external sorter, it decides the peak ram usage, on disk it'll be snappy compressed")
	cmd.Flags().Int(flagConcurrency, runtime.NumCPU(), "Number concurrent goroutines to parallelize the work")

	return cmd
}

// oneStore process a single store, can run in parallel with other stores
func oneStore(store string, snapshotDir, sstDir string, sstFileSize, sorterChunkSize uint64) error {
	prefix := []byte(fmt.Sprintf(tsrocksdb.StorePrefixTpl, store))

	snapshot, err := memiavl.OpenSnapshot(filepath.Join(snapshotDir, store))
	if err != nil {
		return err
	}

	isEmpty := true

	sorter := extsort.New(sstDir, int64(sorterChunkSize), compareSorterNode)
	defer sorter.Close()

	if err := snapshot.ScanNodes(func(node memiavl.PersistedNode) error {
		bz, err := encodeSorterNode(node)
		if err != nil {
			return err
		}
		isEmpty = false
		return sorter.Feed(bz)
	}); err != nil {
		return err
	}

	if isEmpty {
		// SSTFileWriter don't support writing empty files, return early
		return nil
	}

	sstWriter := newIAVLSSTFileWriter()
	defer sstWriter.Destroy()
	sstSeq := 0

	openNextFile := func() error {
		if err := sstWriter.Open(filepath.Join(sstDir, sstFileName(store, sstSeq))); err != nil {
			return err
		}
		sstSeq++
		return nil
	}

	if err := openNextFile(); err != nil {
		return err
	}

	reader, err := sorter.Finalize()
	for {
		item, err := reader.Next()
		if err != nil {
			return err
		}
		if item == nil {
			break
		}

		hash := item[:memiavl.SizeHash]
		value := item[memiavl.SizeHash:]
		key := cloneAppend(prefix, nodeKeyFormat.Key(hash))
		if err := sstWriter.Put(key, value); err != nil {
			return err
		}

		if sstWriter.FileSize() >= sstFileSize {
			if err := sstWriter.Finish(); err != nil {
				return err
			}
			if err := openNextFile(); err != nil {
				return err
			}
		}
	}

	// root record
	rootKey := cloneAppend(prefix, rootKeyFormat.Key(snapshot.Version))
	if err := sstWriter.Put(rootKey, snapshot.RootNode().Hash()); err != nil {
		return err
	}

	return sstWriter.Finish()

}

func newIAVLSSTFileWriter() *grocksdb.SSTFileWriter {
	envOpts := grocksdb.NewDefaultEnvOptions()
	return grocksdb.NewSSTFileWriter(envOpts, open_db.NewRocksdbOptions(true))
}

// encodeNode encodes the node using the same as the existing iavl implementation.
func encodeNode(w io.Writer, node memiavl.PersistedNode) error {
	var buf [binary.MaxVarintLen64]byte

	height := node.Height()
	n := binary.PutVarint(buf[:], int64(height))
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}
	n = binary.PutVarint(buf[:], node.Size())
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}
	n = binary.PutVarint(buf[:], node.Version())
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}

	// Unlike writeHashBytes, key is written for inner nodes.
	if err := encodeBytes(w, node.Key()); err != nil {
		return err
	}

	if height == 0 {
		if err := encodeBytes(w, node.Value()); err != nil {
			return err
		}
	} else {
		if err := encodeBytes(w, node.Left().Hash()); err != nil {
			return err
		}
		if err := encodeBytes(w, node.Right().Hash()); err != nil {
			return err
		}
	}

	return nil
}

func encodeBytes(w io.Writer, bz []byte) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(len(bz)))
	if _, err := w.Write(buf[:n]); err != nil {
		return err
	}
	_, err := w.Write(bz)
	return err
}

func encodeSorterNode(node memiavl.PersistedNode) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeNode(&buf, node); err != nil {
		return nil, err
	}
	return cloneAppend(node.Hash(), buf.Bytes()), nil

}

// compareSorterNode compare the hash part
func compareSorterNode(a, b []byte) bool {
	return bytes.Compare(a[:memiavl.SizeHash], b[:memiavl.SizeHash]) == -1
}
