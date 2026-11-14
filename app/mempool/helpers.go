package mempool

import (
	"context"
	"math/bits"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// maxPreallocEntries bounds up-front map allocation; cache capacity tracks
// the derived tx-cache-size (which can scale with mempool.max-txs) and the
// map grows on demand past this.
const maxPreallocEntries = 4096

func preallocEntries(capacity int) int {
	if capacity < 0 {
		return 0
	}
	if capacity > maxPreallocEntries {
		return maxPreallocEntries
	}
	return capacity
}

// PoolSnapshot returns a snapshot of the current mempool transactions.
func PoolSnapshot(ctx context.Context, mp sdkmempool.Mempool) []sdk.Tx {
	return poolSnapshot(ctx, mp, func(ptx sdkmempool.PooledTx) sdk.Tx { return ptx.Tx })
}

// PooledSnapshot is PoolSnapshot keeping the ante-reported gas wanted of each tx.
func PooledSnapshot(ctx context.Context, mp sdkmempool.Mempool) []sdkmempool.PooledTx {
	return poolSnapshot(ctx, mp, func(ptx sdkmempool.PooledTx) sdkmempool.PooledTx { return ptx })
}

func poolSnapshot[T any](ctx context.Context, mp sdkmempool.Mempool, project func(sdkmempool.PooledTx) T) []T {
	defer telemetry.MeasureSince(telemetry.Now(), "cronos", "mempool", "pool", "snapshot")
	snap := make([]T, 0, mp.CountTx())
	sdkmempool.SelectBy(ctx, mp, nil, func(ptx sdkmempool.PooledTx) bool {
		snap = append(snap, project(ptx))
		return true
	})
	return snap
}

// UnorderedPoolSnapshot returns pooled txs without priority ordering.
func UnorderedPoolSnapshot(ctx context.Context, mp sdkmempool.Mempool) []sdk.Tx {
	defer telemetry.MeasureSince(telemetry.Now(), "cronos", "mempool", "pool", "snapshot", "unordered")
	return sdkmempool.UnorderedTxs(ctx, mp)
}

// BlockGas mirrors baseapp's ProcessProposal accounting: the ante-reported gas
// wanted, else the declared gas. Proposals built with it are never rejected for MaxGas.
// Must stay in sync with the unexported baseapp txGasForBlockAccounting.
func BlockGas(tx sdk.Tx, gasWanted uint64) uint64 {
	if gasWanted > 0 {
		return gasWanted
	}
	if gasTx, ok := tx.(sdk.GasTx); ok {
		return gasTx.GetGas()
	}
	return 0
}

// EncodeTx returns the raw bytes of a transaction, prioritizing the cache if available.
func EncodeTx(encCache *EncoderCache, enc sdk.TxEncoder, tx sdk.Tx) (bz []byte, hit bool, err error) {
	if b, ok := encCache.Get(tx); ok {
		return b, true, nil
	}
	bz, err = enc(tx)
	return bz, false, err
}

// ProtoSizeForTx returns the wire size one tx contributes to a CometBFT block's
// Data message. Same result as cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{bz})
// but without its ~4 allocs/tx ([]Tx slice + Data{}.ToProto) in the proposal hot
// loop. Data.Txs is `repeated bytes txs = 1`: each element is a 1-byte field tag
// + varint length + payload, matching gogoproto's generated Size().
func ProtoSizeForTx(bz []byte) int64 {
	l := len(bz)
	return int64(1 + sovLen(uint64(l)) + l)
}

// sovLen returns the byte count gogoproto uses to varint-encode x, matching the
// generated sov* helpers (7 bits per byte).
func sovLen(x uint64) int {
	return (bits.Len64(x|1) + 6) / 7
}
