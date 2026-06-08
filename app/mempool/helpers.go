package mempool

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// PoolSnapshot returns a snapshot of the current mempool transactions.
func PoolSnapshot(ctx context.Context, mp sdkmempool.Mempool) []sdk.Tx {
	var snap []sdk.Tx
	sdkmempool.SelectBy(ctx, mp, nil, func(tx sdk.Tx) bool {
		snap = append(snap, tx)
		return true
	})
	return snap
}

// EncodeTx returns the raw bytes of a transaction, prioritising the cache if available.
func EncodeTx(encCache *EncoderCache, enc sdk.TxEncoder, tx sdk.Tx) (bz []byte, hit bool, err error) {
	if b, ok := encCache.Get(tx); ok {
		return b, true, nil
	}
	bz, err = enc(tx)
	return bz, false, err
}
