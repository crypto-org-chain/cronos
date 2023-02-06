package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"cosmossdk.io/errors"
	"github.com/alitto/pond"
	"github.com/cosmos/iavl"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"

	"github.com/crypto-org-chain/cronos/cmd/cronosd/open_db"
	"github.com/crypto-org-chain/cronos/versiondb/extsort"
	"github.com/crypto-org-chain/cronos/versiondb/memiavl"
)

const (
	int64Size = 8

	storeKeyPrefix   = "s/k:%s/"
	latestVersionKey = "s/latest"
	commitInfoKeyFmt = "s/%d" // s/<version>

	// We creates the temporary sst files in the target database to make sure the file renaming is cheap in ingestion
	// part.
	StoreSSTFileName = "tmp-%s-%d.sst"

	PipelineBufferSize = 1024
)

var (
	nodeKeyFormat = iavl.NewKeyFormat('n', memiavl.SizeHash) // n<hash>
	rootKeyFormat = iavl.NewKeyFormat('r', int64Size)        // r<version>
)

func RestoreAppDB(appCreator types.AppCreator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore-app-db snapshot-dir application.db",
		Short: "Restore `application.db` from memiavl snapshots",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sstFileSizeTarget, err := cmd.Flags().GetUint64(flagSSTFileSize)
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
			iavlDir := args[1]
			if err := os.MkdirAll(iavlDir, os.ModePerm); err != nil {
				return err
			}

			// load the snapshots and compute commit info first
			var lastestVersion int64
			storeInfos := []storetypes.StoreInfo{
				// https://github.com/cosmos/cosmos-sdk/issues/14916
				storetypes.StoreInfo{capabilitytypes.MemStoreKey, storetypes.CommitID{}},
			}
			snapshots := make([]*memiavl.Snapshot, len(stores))
			for i, store := range stores {
				path := filepath.Join(snapshotDir, store)
				snapshot, err := memiavl.OpenSnapshot(path)
				if err != nil {
					return errors.Wrapf(err, "open snapshot fail: %s", path)
				}
				snapshots[i] = snapshot

				tree := memiavl.NewFromSnapshot(snapshot)
				commitId := lastCommitID(tree)
				storeInfos = append(storeInfos, storetypes.StoreInfo{
					Name:     store,
					CommitId: commitId,
				})

				if commitId.Version > lastestVersion {
					lastestVersion = commitId.Version
				}
			}
			commitInfo := buildCommitInfo(storeInfos, lastestVersion)

			// create fixed size task pool with big enough buffer.
			pool := pond.New(concurrency, 0)
			defer pool.StopAndWait()

			group, _ := pool.GroupContext(context.Background())
			for i := 0; i < len(stores); i++ {
				// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
				i := i
				group.Submit(func() error {
					return oneStore(stores[i], snapshots[i], iavlDir, sstFileSizeTarget, sorterChunkSize)
				})
			}

			if err := group.Wait(); err != nil {
				return errors.Wrap(err, "worker pool wait fail")
			}

			// collect the sst files
			entries, err := os.ReadDir(iavlDir)
			if err != nil {
				return errors.Wrapf(err, "read directory fail: %s iavlDir")
			}
			sstFiles := make([]string, 0, len(entries))
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, "tmp-") {
					sstFiles = append(sstFiles, filepath.Join(iavlDir, name))
				}
			}

			// sst files ingestion
			ingestOpts := grocksdb.NewDefaultIngestExternalFileOptions()
			defer ingestOpts.Destroy()
			ingestOpts.SetMoveFiles(true)

			db, err := grocksdb.OpenDb(open_db.NewRocksdbOptions(false), iavlDir)
			if err != nil {
				return errors.Wrap(err, "open iavl db fail")
			}
			defer db.Close()

			if err := db.IngestExternalFile(sstFiles, ingestOpts); err != nil {
				return errors.Wrap(err, "ingset sst files fail")
			}

			// write the metadata part separately, because it overlaps with the other sst files
			if err := writeMetadata(db, &commitInfo); err != nil {
				return errors.Wrap(err, "write metadata fail")
			}

			fmt.Printf("version: %d, app hash: %X\n", commitInfo.Version, commitInfo.Hash())
			return nil
		},
	}

	cmd.Flags().Uint64(flagSSTFileSize, DefaultSSTFileSize, "the target sst file size, note the actual file size may be larger because sst files must be split on different key names")
	cmd.Flags().String(flagStores, "", "list of store names, default to the current store list in application")
	cmd.Flags().Uint64(flagSorterChunkSize, DefaultSorterChunkSize, "uncompressed chunk size for external sorter, it decides the peak ram usage, on disk it'll be snappy compressed")
	cmd.Flags().Int(flagConcurrency, runtime.NumCPU(), "Number concurrent goroutines to parallelize the work")

	return cmd
}

// oneStore process a single store, can run in parallel with other stores,
// it spawns another goroutines to parallelize the pipeline:
// - main thread scan the snapshot and pass the nodes to worker thread.
// - worker thread encode the nodes and feed the external sorter.
// - after finished the sorting, worker thread scan the external sorter, pass the result to main thread.
// - main thread do the sst file writing.
func oneStore(store string, snapshot *memiavl.Snapshot, sstDir string, sstFileSizeTarget, sorterChunkSize uint64) error {
	defer snapshot.Close()

	prefix := []byte(fmt.Sprintf(storeKeyPrefix, store))

	// main thread pass the unsorted `PersistedNode`s to worker thread
	toSortChan := make(chan memiavl.PersistedNode, PipelineBufferSize)
	// worker thread do the external sorting and pass the sorted key-value pairs to main thread
	sortedChan := make(chan kvPair, PipelineBufferSize)

	go func() {
		defer close(sortedChan)

		sorter := extsort.New(sstDir, int64(sorterChunkSize), compareSorterNode)
		defer sorter.Close()

		for node := range toSortChan {
			bz, err := encodeSorterNode(node)
			if err != nil {
				panic(errors.Wrap(err, "encode sorter node"))
			}
			if err := sorter.Feed(bz); err != nil {
				panic(err)
			}
		}

		reader, err := sorter.Finalize()
		if err != nil {
			panic(errors.Wrap(err, "external sorter finalize fail"))
		}
		for {
			item, err := reader.Next()
			if err != nil {
				panic(err)
			}
			if item == nil {
				break
			}

			hash := item[:memiavl.SizeHash]
			value := item[memiavl.SizeHash:]
			sortedChan <- kvPair{
				key:   cloneAppend(prefix, nodeKeyFormat.Key(hash)),
				value: value,
			}
		}
	}()

	err := snapshot.ScanNodes(func(node memiavl.PersistedNode) error {
		_ = node.Height() // trigger the IO on main thread
		toSortChan <- node
		return nil
	})
	close(toSortChan)
	if err != nil {
		return err
	}

	sstWriter := newIAVLSSTFileWriter()
	defer sstWriter.Destroy()

	sstSeq := 0
	openNextFile := func() error {
		sstFileName := filepath.Join(sstDir, fmt.Sprintf(StoreSSTFileName, store, sstSeq))
		if err := sstWriter.Open(sstFileName); err != nil {
			return errors.Wrapf(err, "open sst file fail: %s", sstFileName)
		}
		sstSeq++
		return nil
	}

	if err := openNextFile(); err != nil {
		return err
	}
	for pair := range sortedChan {
		if err := sstWriter.Put(pair.key, pair.value); err != nil {
			return errors.Wrap(err, "sst write node fail")
		}

		if sstWriter.FileSize() >= sstFileSizeTarget {
			if err := sstWriter.Finish(); err != nil {
				return errors.Wrap(err, "sst writer finish fail")
			}
			if err := openNextFile(); err != nil {
				return err
			}
		}
	}

	// root record
	rootKey := cloneAppend(prefix, rootKeyFormat.Key(int64(snapshot.Version())))
	var rootHash []byte
	if !snapshot.IsEmpty() {
		rootHash = snapshot.RootNode().Hash()
	}
	if err := sstWriter.Put(rootKey, rootHash); err != nil {
		return errors.Wrap(err, "sst write root fail")
	}

	if err := sstWriter.Finish(); err != nil {
		return errors.Wrap(err, "sst writer finish fail")
	}

	return nil
}

type kvPair struct {
	key   []byte
	value []byte
}

// writeMetadata writes the rootmulti commit info and latest version to the db
func writeMetadata(db *grocksdb.DB, cInfo *storetypes.CommitInfo) error {
	writeOpts := grocksdb.NewDefaultWriteOptions()

	bz, err := cInfo.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal CommitInfo fail")
	}

	cInfoKey := fmt.Sprintf(commitInfoKeyFmt, cInfo.Version)
	if err := db.Put(writeOpts, []byte(cInfoKey), bz); err != nil {
		return err
	}

	bz, err = gogotypes.StdInt64Marshal(cInfo.Version)
	if err != nil {
		return err
	}

	return db.Put(writeOpts, []byte(latestVersionKey), bz)
}

func newIAVLSSTFileWriter() *grocksdb.SSTFileWriter {
	envOpts := grocksdb.NewDefaultEnvOptions()
	return grocksdb.NewSSTFileWriter(envOpts, open_db.NewRocksdbOptions(true))
}

// encodeNode encodes the node in the same way as the existing iavl implementation.
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
	if _, err := buf.Write(node.Hash()); err != nil {
		return nil, err
	}
	if err := encodeNode(&buf, node); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// compareSorterNode compare the hash part
func compareSorterNode(a, b []byte) bool {
	return bytes.Compare(a[:memiavl.SizeHash], b[:memiavl.SizeHash]) == -1
}
