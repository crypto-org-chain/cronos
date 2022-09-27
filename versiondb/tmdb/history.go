package tmdb

import (
	"bytes"

	"github.com/RoaringBitmap/roaring/roaring64"

	"github.com/crypto-org-chain/cronos/versiondb"
	dbm "github.com/tendermint/tm-db"
)

// GetHistoryIndex returns the history index bitmap.
func GetHistoryIndex(db dbm.DB, key []byte) (*roaring64.Bitmap, error) {
	// try to seek the first chunk whose maximum is bigger or equal to the target height.
	bz, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	if len(bz) == 0 {
		return nil, nil
	}
	m := roaring64.New()
	_, err = m.ReadFrom(bytes.NewReader(bz))
	if err != nil {
		return nil, err
	}
	return m, nil
}

// SeekHistoryIndex locate the minimal version that changed the key and is larger than the target version,
// using the returned version can find the value for the target version in changeset table.
// If not found, return -1
func SeekHistoryIndex(db dbm.DB, key []byte, version uint64) (int64, error) {
	m, err := GetHistoryIndex(db, key)
	if err != nil {
		return -1, err
	}
	found, ok := versiondb.SeekInBitmap64(m, version+1)
	if !ok {
		return -1, nil
	}
	return int64(found), nil
}

// WriteHistoryIndex set the block height to the history bitmap.
// it try to set to the last chunk, if the last chunk exceeds chunk limit, split it.
func WriteHistoryIndex(db dbm.DB, batch dbm.Batch, key []byte, height uint64) error {
	bz, err := db.Get(key)
	if err != nil {
		return err
	}

	m := roaring64.New()
	if len(bz) > 0 {
		_, err = m.ReadFrom(bytes.NewReader(bz))
		if err != nil {
			return err
		}
	}
	m.Add(height)
	m.RunOptimize()
	bz, err = m.ToBytes()
	if err != nil {
		return err
	}
	return batch.Set(key, bz)
}
