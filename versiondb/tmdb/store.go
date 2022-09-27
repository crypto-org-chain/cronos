package tmdb

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/dbadapter"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogotypes "github.com/cosmos/gogoproto/types"
	"github.com/crypto-org-chain/cronos/versiondb"
	dbm "github.com/tendermint/tm-db"
)

const latestVersionKey = "s/latest"

var _ versiondb.VersionStore = (*Store)(nil)

// Store implements `VersionStore`.
type Store struct {
	// latest key-value pairs
	plainDB dbm.DB
	// history bitmap index of keys
	historyDB dbm.DB
	// changesets of each blocks
	changesetDB dbm.DB
}

func NewStore(plainDB, historyDB, changesetDB dbm.DB) *Store {
	return &Store{plainDB, historyDB, changesetDB}
}

// PutAtVersion implements VersionStore interface
// TODO reduce allocation within iterations.
func (s *Store) PutAtVersion(version int64, changeSet []types.StoreKVPair) error {
	plainBatch := s.plainDB.NewBatch()
	defer plainBatch.Close()
	historyBatch := s.historyDB.NewBatch()
	defer historyBatch.Close()
	changesetBatch := s.changesetDB.NewBatch()
	defer changesetBatch.Close()

	for _, pair := range changeSet {
		key := prependStoreKey(pair.StoreKey, pair.Key)

		if version == 0 {
			// genesis state is written into plain state directly
			if pair.Delete {
				return errors.New("can't delete at genesis")
			}
			if err := plainBatch.Set(key, pair.Value); err != nil {
				return err
			}
			continue
		}

		original, err := s.plainDB.Get(key)
		if err != nil {
			return err
		}
		if bytes.Equal(original, pair.Value) {
			// do nothing if the value is not changed
			continue
		}

		// write history index
		if err := WriteHistoryIndex(s.historyDB, historyBatch, key, uint64(version)); err != nil {
			return err
		}

		// write the old value to changeset
		if len(original) > 0 {
			changesetKey := append(sdk.Uint64ToBigEndian(uint64(version)), key...)
			if err := changesetBatch.Set(changesetKey, original); err != nil {
				return err
			}
		}

		// write the new value to plain state
		if pair.Delete {
			if err := plainBatch.Delete(key); err != nil {
				return err
			}
		} else {
			if err := plainBatch.Set(key, pair.Value); err != nil {
				return err
			}
		}
	}

	// write latest version to plain state
	if err := s.setLatestVersion(plainBatch, version); err != nil {
		return err
	}

	if err := changesetBatch.WriteSync(); err != nil {
		return err
	}
	if err := historyBatch.WriteSync(); err != nil {
		return err
	}
	return plainBatch.WriteSync()
}

// GetAtVersion implements VersionStore interface
func (s *Store) GetAtVersion(storeKey string, key []byte, version *int64) ([]byte, error) {
	rawKey := prependStoreKey(storeKey, key)
	if version == nil {
		return s.plainDB.Get(rawKey)
	}

	height := *version

	// optimize for latest version
	latest, err := s.GetLatestVersion()
	if err != nil {
		return nil, err
	}
	if height > latest {
		return nil, fmt.Errorf("height %d is in the future", height)
	}
	if latest == height {
		return s.plainDB.Get(rawKey)
	}

	found, err := SeekHistoryIndex(s.historyDB, rawKey, uint64(height))
	if err != nil {
		return nil, err
	}
	if found < 0 {
		// there's no change records found after the target version, query the latest state.
		return s.plainDB.Get(rawKey)
	}
	// get from changeset
	changesetKey := ChangesetKey(uint64(found), rawKey)
	return s.changesetDB.Get(changesetKey)
}

// HasAtVersion implements VersionStore interface
func (s *Store) HasAtVersion(storeKey string, key []byte, version *int64) (bool, error) {
	rawKey := prependStoreKey(storeKey, key)
	if version == nil {
		return s.plainDB.Has(rawKey)
	}

	height := *version

	// optimize for latest version
	latest, err := s.GetLatestVersion()
	if err != nil {
		return false, err
	}
	if height > latest {
		return false, fmt.Errorf("height %d is in the future", height)
	}
	if latest == height {
		return s.plainDB.Has(rawKey)
	}

	found, err := SeekHistoryIndex(s.historyDB, rawKey, uint64(height))
	if err != nil {
		return false, err
	}
	if found < 0 {
		// there's no change records after the target version, query the latest state.
		return s.plainDB.Has(rawKey)
	}
	// get from changeset
	changesetKey := ChangesetKey(uint64(found), rawKey)
	return s.changesetDB.Has(changesetKey)
}

// IteratorAtVersion implements VersionStore interface
func (s *Store) IteratorAtVersion(storeKey string, start, end []byte, version *int64) (types.Iterator, error) {
	storePrefix := StoreKeyPrefix(storeKey)
	prefixPlain := prefix.NewStore(dbadapter.Store{DB: s.plainDB}, storePrefix)
	if version == nil {
		return prefixPlain.Iterator(start, end), nil
	}

	// optimize for latest version
	height := *version
	latest, err := s.GetLatestVersion()
	if err != nil {
		return nil, err
	}
	if height > latest {
		return nil, fmt.Errorf("height %d is in the future", height)
	}
	if latest == height {
		return prefixPlain.Iterator(start, end), nil
	}

	prefixHistory := prefix.NewStore(dbadapter.Store{DB: s.historyDB}, storePrefix)
	return NewIterator(storeKey, height, prefixPlain, prefixHistory, s.changesetDB, start, end, false)
}

// ReverseIteratorAtVersion implements VersionStore interface
func (s *Store) ReverseIteratorAtVersion(storeKey string, start, end []byte, version *int64) (types.Iterator, error) {
	storePrefix := StoreKeyPrefix(storeKey)
	prefixPlain := prefix.NewStore(dbadapter.Store{DB: s.plainDB}, storePrefix)
	if version == nil {
		return prefixPlain.ReverseIterator(start, end), nil
	}

	// optimize for latest version
	height := *version
	latest, err := s.GetLatestVersion()
	if err != nil {
		return nil, err
	}
	if height > latest {
		return nil, fmt.Errorf("height %d is in the future", height)
	}
	if latest == height {
		return prefixPlain.ReverseIterator(start, end), nil
	}

	prefixHistory := prefix.NewStore(dbadapter.Store{DB: s.historyDB}, storePrefix)
	return NewIterator(storeKey, height, prefixPlain, prefixHistory, s.changesetDB, start, end, true)
}

// GetLatestVersion returns the latest version stored in plain state,
// it's committed after the changesets, so the data for this version is guaranteed to be persisted.
// returns -1 if the key don't exists.
func (s *Store) GetLatestVersion() (int64, error) {
	bz, err := s.plainDB.Get([]byte(latestVersionKey))
	if err != nil {
		return -1, err
	} else if bz == nil {
		return -1, nil
	}

	var latestVersion int64

	if err := gogotypes.StdInt64Unmarshal(&latestVersion, bz); err != nil {
		return -1, err
	}

	return latestVersion, nil
}

func (s *Store) setLatestVersion(plainBatch dbm.Batch, version int64) error {
	// write latest version to plain state
	bz, err := gogotypes.StdInt64Marshal(version)
	if err != nil {
		return err
	}
	return plainBatch.Set([]byte(latestVersionKey), bz)
}

// ChangesetKey build key changeset db
func ChangesetKey(version uint64, key []byte) []byte {
	return append(sdk.Uint64ToBigEndian(version), key...)
}

func StoreKeyPrefix(storeKey string) []byte {
	return []byte("s/k:" + storeKey + "/")
}

// prependStoreKey prepends storeKey to the key
func prependStoreKey(storeKey string, key []byte) []byte {
	return append(StoreKeyPrefix(storeKey), key...)
}
