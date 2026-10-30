package mempool

import (
	"context"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

const TypeApp = "app"

// NewReapTxsHandler scans the app mempool to gather txs to be gossiped to other peers,
// stopping at MaxBytes/MaxGas (0 = no cap, per CometBFT convention). Prefix scan: breaks
// at the first tx over a cap (not best-fit), so a large high-priority tx may
// leave budget unused. Encode errors and per-tx panics skip the tx; encCache hits skip proto.Marshal.
func NewReapTxsHandler(mpool mempool.Mempool, txEncoder sdk.TxEncoder, encCache *EncoderCache, ttl time.Duration, maxPerReap int, logger log.Logger) sdk.ReapTxsHandler {
	tracker := newGossipTracker(ttl)
	return func(req *abci.RequestReapTxs) (resp *abci.ResponseReapTxs, _ error) {
		defer func() {
			if r := recover(); r != nil {
				logRecoveredPanic(logger, panicPathReap, r)
				resp = &abci.ResponseReapTxs{}
			}
		}()

		snapshot := PoolSnapshot(context.Background(), mpool)

		now := time.Now().UnixNano()
		var (
			totalBytes uint64
			totalGas   uint64
			cacheHits  float32
			cacheMiss  float32
			deduped    float32
		)
		txs := make([][]byte, 0, len(snapshot))
		for _, tx := range snapshot {
			bz, hash, gas, hit, err := prepareForReap(encCache, txEncoder, tx, logger)
			if hit {
				cacheHits++
			} else {
				cacheMiss++
			}
			if err != nil {
				continue
			}
			size := uint64(ProtoSizeForTx(bz))
			if req.MaxBytes > 0 && totalBytes+size > req.MaxBytes {
				break
			}
			if req.MaxGas > 0 && gas > req.MaxGas-totalGas {
				break
			}
			if !tracker.gossiped(hash, now) {
				deduped++
				continue
			}
			txs = append(txs, bz)
			totalBytes += size
			totalGas += gas
			if maxPerReap > 0 && len(txs) >= maxPerReap {
				break
			}
		}
		tracker.prune(now)
		if cacheHits > 0 {
			telemetry.IncrCounter(cacheHits, "cronos", "mempool", "reap", "encode_cache", "hit")
		}
		if cacheMiss > 0 {
			telemetry.IncrCounter(cacheMiss, "cronos", "mempool", "reap", "encode_cache", "miss")
		}
		if len(txs) > 0 {
			telemetry.IncrCounter(float32(len(txs)), "cronos", "mempool", "reap", "gossip", "sent")
		}
		if deduped > 0 {
			telemetry.IncrCounter(deduped, "cronos", "mempool", "reap", "gossip", "deduped")
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

// prepareForReap recovers per tx so one bad tx can't stall gossip for the pool.
func prepareForReap(encCache *EncoderCache, txEncoder sdk.TxEncoder, tx sdk.Tx, logger log.Logger) (bz []byte, hash [32]byte, gas uint64, hit bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			bz, err = nil, recoveredPanic(logger, panicPathReap, r)
		}
	}()
	bz, hit, err = EncodeTx(encCache, txEncoder, tx)
	if err != nil {
		logger.Error("reap encode failed; skipping tx", "err", err)
		return nil, hash, 0, hit, err
	}
	if feeTx, ok := tx.(sdk.FeeTx); ok {
		gas = feeTx.GetGas()
	}
	return bz, encCache.HashTx(tx, bz), gas, hit, nil
}
