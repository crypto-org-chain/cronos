package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/cosmos/iavl"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/versiondb/extsort"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

const (
	SSTFileExtension       = ".sst"
	DefaultSSTFileSize     = 128 * 1024 * 1024
	DefaultSorterChunkSize = 256 * 1024 * 1024

	// SizeKeyLength is the number of bytes used to encode key length in sort payload
	SizeKeyLength = 4
)

func ConvertToSSTTSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert-to-sst sst-output plain-1 [plain-2] ...",
		Short: "Convert change set files to versiondb/rocksdb sst files, which can be ingested into versiondb later",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sstFile := args[0]

			csFiles, err := sortChangeSetFiles(args[1:])
			if err != nil {
				return err
			}

			sstFileSize, err := cmd.Flags().GetUint64(flagSSTFileSize)
			if err != nil {
				return err
			}

			sorterChunkSize, err := cmd.Flags().GetInt64(flagSorterChunkSize)
			if err != nil {
				return err
			}
			store, err := cmd.Flags().GetString(flagStore)
			if err != nil {
				return err
			}

			var prefix []byte
			if len(store) > 0 {
				prefix = []byte(fmt.Sprintf(tsrocksdb.StorePrefixTpl, store))
			}

			sorter := extsort.New(filepath.Dir(sstFile), sorterChunkSize, compareSorterItem)
			defer sorter.Close()
			for _, plainFile := range csFiles {
				if err := withChangeSetFile(plainFile, func(reader Reader) error {
					_, err := IterateChangeSets(reader, func(version int64, changeSet *iavl.ChangeSet) (bool, error) {
						for _, pair := range changeSet.Pairs {
							item := encodeSorterItem(uint64(version), pair)
							if err := sorter.Feed(item); err != nil {
								return false, err
							}
						}
						return true, nil
					})

					return err
				}); err != nil {
					return err
				}
			}

			mergedReader, err := sorter.Finalize()
			if err != nil {
				return err
			}

			sstWriter := newSSTFileWriter()
			defer sstWriter.Destroy()
			sstSeq := 0
			if err := sstWriter.Open(sstFileName(sstFile, sstSeq)); err != nil {
				return err
			}
			sstSeq++

			var lastKey []byte
			for {
				item, err := mergedReader.Next()
				if err != nil {
					return err
				}
				if item == nil {
					break
				}

				ts, pair := decodeSorterItem(item)

				// Only breakup sst file when the next key different, don't cause overlap in keys without the timestamp part in sst files,
				// because the rocksdb ingestion logic checks for overlap in keys without the timestamp part currently.
				if sstWriter.FileSize() >= sstFileSize && !bytes.Equal(lastKey, pair.Key) {
					if err := sstWriter.Finish(); err != nil {
						return err
					}
					if err := sstWriter.Open(sstFileName(sstFile, sstSeq)); err != nil {
						return err
					}
					sstSeq++
				}

				key := cloneAppend(prefix, pair.Key)
				if pair.Delete {
					err = sstWriter.DeleteWithTS(key, ts)
				} else {
					err = sstWriter.PutWithTS(key, ts, pair.Value)
				}
				if err != nil {
					return err
				}

				lastKey = pair.Key
			}
			return sstWriter.Finish()
		},
	}
	cmd.Flags().Uint64(flagSSTFileSize, DefaultSSTFileSize, "the target sst file size, note the actual file size may be larger because sst files must be split on different key names")
	cmd.Flags().String(flagStore, "", "store name, the keys are prefixed with \"s/k:{store}/\"")
	cmd.Flags().Int64(flagSorterChunkSize, DefaultSorterChunkSize, "uncompressed chunk size for external sorter, it decides the peak ram usage, on disk it'll be snappy compressed")
	return cmd
}

// sstFileName inserts the seq integer into the base file name
func sstFileName(fileName string, seq int) string {
	stem := fileName[:len(fileName)-len(SSTFileExtension)]
	return stem + fmt.Sprintf("-%d", seq) + SSTFileExtension
}

func newSSTFileWriter() *grocksdb.SSTFileWriter {
	envOpts := grocksdb.NewDefaultEnvOptions()
	return grocksdb.NewSSTFileWriter(envOpts, tsrocksdb.NewVersionDBOpts(true))
}

// encodeSorterItem encode kv-pair for use in external sorter.
//
// layout: key + version(8) + delete(1) + [ value ] + key length(SizeKeyLength)
// we put the key and version in the front of payload so it can take advantage of the delta encoding in the `ExtSorter`.
func encodeSorterItem(version uint64, pair iavl.KVPair) []byte {
	item := make([]byte, sizeOfSorterItem(pair))
	copy(item, pair.Key)
	offset := len(pair.Key)

	binary.LittleEndian.PutUint64(item[offset:], version)
	offset += tsrocksdb.TimestampSize

	if pair.Delete {
		item[offset] = 1
		offset++
	} else {
		copy(item[offset+1:], pair.Value)
		offset += len(pair.Value) + 1
	}
	binary.LittleEndian.PutUint32(item[offset:], uint32(len(pair.Key)))
	return item
}

// sizeOfSorterItem compute the encoded size of pair
//
// see godoc of `encodeSorterItem` for layout
func sizeOfSorterItem(pair iavl.KVPair) int {
	size := len(pair.Key) + tsrocksdb.TimestampSize + 1 + SizeKeyLength
	if !pair.Delete {
		size += len(pair.Value)
	}
	return size
}

// decodeSorterItem decode the kv-pair from external sorter.
//
// see godoc of `encodeSorterItem` for layout
func decodeSorterItem(item []byte) ([]byte, iavl.KVPair) {
	var value []byte
	keyLen := binary.LittleEndian.Uint32(item[len(item)-SizeKeyLength:])
	key := item[:keyLen]

	offset := keyLen
	version := item[offset : offset+tsrocksdb.TimestampSize]

	offset += tsrocksdb.TimestampSize
	delete := item[offset] == 1

	if !delete {
		offset++
		value = item[offset : len(item)-SizeKeyLength]
	}

	return version, iavl.KVPair{
		Delete: delete,
		Key:    key,
		Value:  value,
	}
}

// compareSorterItem compare encoded kv-pairs return if a < b.
func compareSorterItem(a, b []byte) bool {
	// decode key and version
	aKeyLen := binary.LittleEndian.Uint32(a[len(a)-SizeKeyLength:])
	bKeyLen := binary.LittleEndian.Uint32(b[len(b)-SizeKeyLength:])
	ret := bytes.Compare(a[:aKeyLen], b[:bKeyLen])
	if ret != 0 {
		return ret == -1
	}

	aVersion := binary.LittleEndian.Uint64(a[aKeyLen:])
	bVersion := binary.LittleEndian.Uint64(b[bKeyLen:])
	// Compare version.
	// For the same user key with different timestamps, larger (newer) timestamp
	// comes first.
	return aVersion > bVersion
}

func cloneAppend(bz []byte, tail []byte) (res []byte) {
	res = make([]byte, len(bz)+len(tail))
	copy(res, bz)
	copy(res[len(bz):], tail)
	return
}
