package tmdb

import (
	"bytes"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/cosmos/cosmos-sdk/store/types"
	"github.com/crypto-org-chain/cronos/versiondb"
	dbm "github.com/tendermint/tm-db"
)

type Iterator struct {
	storeKey string
	version  int64

	start, end []byte

	plain, history types.Iterator
	changesetDB    dbm.DB

	key, value []byte

	reverse bool
	status  int
	err     error
}

var _ types.Iterator = (*Iterator)(nil)

func NewIterator(storeKey string, version int64, plainDB, historyDB types.KVStore, changesetDB dbm.DB, start, end []byte, reverse bool) (types.Iterator, error) {
	var plain, history types.Iterator

	if reverse {
		plain = plainDB.ReverseIterator(start, end)
	} else {
		plain = plainDB.Iterator(start, end)
	}

	if reverse {
		history = historyDB.ReverseIterator(start, end)
	} else {
		history = historyDB.Iterator(start, end)
	}
	iter := &Iterator{
		storeKey: storeKey, version: version,
		reverse: reverse,
		start:   start, end: end,
		plain: plain, history: history,
		changesetDB: changesetDB,
	}
	iter.err = iter.resolve()
	return iter, nil
}

// Domain implements types.Iterator.
func (iter *Iterator) Domain() ([]byte, []byte) {
	return iter.start, iter.end
}

func (iter *Iterator) Valid() bool {
	return iter.err == nil && len(iter.key) > 0
}

func (iter *Iterator) Next() {
	switch iter.status {
	case -2:
		return
	case 0:
		iter.plain.Next()
		iter.history.Next()
	case 1:
		iter.history.Next()
	case -1:
		iter.plain.Next()
	}
	iter.err = iter.resolve()
}

func (iter *Iterator) Key() []byte {
	return iter.key
}

func (iter *Iterator) Value() []byte {
	return iter.value
}

func (iter *Iterator) Close() error {
	err1 := iter.plain.Close()
	err2 := iter.history.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func (iter *Iterator) Error() error {
	return iter.err
}

func (iter *Iterator) getFromHistory(key []byte, bz []byte, getLatestValue func() []byte) ([]byte, error) {
	m := roaring64.New()
	_, err := m.ReadFrom(bytes.NewReader(bz))
	if err != nil {
		return nil, err
	}
	found, ok := versiondb.SeekInBitmap64(m, uint64(iter.version)+1)
	if !ok {
		// not changed, use the latest one
		return getLatestValue(), nil
	}
	changesetKey := ChangesetKey(found, prependStoreKey(iter.storeKey, key))
	return iter.changesetDB.Get(changesetKey)
}

func (iter *Iterator) resolve() (err error) {
	for {
		var pkey, hkey []byte
		if iter.plain.Valid() {
			pkey = iter.plain.Key()
		}
		if iter.history.Valid() {
			hkey = iter.history.Key()
		}

		iter.status = compareKey(pkey, hkey, iter.reverse)
		switch iter.status {
		case -2:
			// end of iteration
			iter.key = nil
			iter.value = nil
			return nil
		case 0:
			// find the historial value, or fallback to latest one.
			iter.key = hkey
			iter.value, err = iter.getFromHistory(hkey, iter.history.Value(), func() []byte {
				return iter.plain.Value()
			})
			if len(iter.value) > 0 {
				return
			}
			iter.plain.Next()
			iter.history.Next()
		case 1:
			// plain state exhausted or history cursor lag behind
			// the key is deleted in plain state, use the history state.
			iter.key = hkey
			iter.value, err = iter.getFromHistory(hkey, iter.history.Value(), func() []byte {
				return nil
			})
			if len(iter.value) > 0 {
				return
			}
			iter.history.Next()
		case -1:
			// history state exhausted or plain cursor lag behind
			// the key don't exist in history state, use the plain state value.
			iter.key = pkey
			iter.value = iter.plain.Value()
			return
		}
	}
}

// compareKey is similar to bytes.Compare, but it treat empty slice as biggest value.
func compareKey(k1, k2 []byte, reverse bool) int {
	switch {
	case len(k1) == 0 && len(k2) == 0:
		return -2
	case len(k1) == 0:
		return 1
	case len(k2) == 0:
		return -1
	default:
		result := bytes.Compare(k1, k2)
		if reverse {
			result = -result
		}
		return result
	}
}
