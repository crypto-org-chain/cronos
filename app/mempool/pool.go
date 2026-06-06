package mempool

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// SnapshotPool materializes the pool's tx pointers under its lock so callers can
// encode/validate/Remove after release: SelectBy holds mp's mutex for the whole
// callback, blocking admission (Insert) and the reap ticker. Pre-sized to CountTx
// to avoid slice growth under that lock.
func SnapshotPool(ctx context.Context, mp sdkmempool.Mempool) []sdk.Tx {
	snap := make([]sdk.Tx, 0, mp.CountTx())
	sdkmempool.SelectBy(ctx, mp, nil, func(tx sdk.Tx) bool {
		snap = append(snap, tx)
		return true
	})
	return snap
}

// EncodeTx returns tx's wire bytes, preferring encCache over proto.Marshal. hit
// reports whether the cache served it so callers can tally the fallback rate; err
// is non-nil only on a real encode failure (caller skips the tx).
func EncodeTx(encCache *EncoderCache, enc sdk.TxEncoder, tx sdk.Tx) (bz []byte, hit bool, err error) {
	if b, ok := encCache.Bytes(tx); ok {
		return b, true, nil
	}
	bz, err = enc(tx)
	return bz, false, err
}
