package mempool

import (
	"context"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// EnableRecheck wires the deps RecheckLocked/StageRecheckSenders need for
// mempool.type=app. decoder must hit the tx-decode cache so staging committed
// txs is cheap. Until called, both recheck methods no-op.
func (a *Admitter) EnableRecheck(mpool sdkmempool.Mempool, signer sdkmempool.SignerExtractionAdapter, decoder sdk.TxDecoder) {
	a.mpool = mpool
	a.signer = signer
	a.decoder = decoder
}

// StageRecheckSenders records the senders of the just-committed block's txs so
// RecheckLocked can re-validate only their remaining pending txs, and stages the
// committed height for TimeoutHeight eviction. CometBFT's app-mempool Update()
// is a no-op, so the app drives recheck itself.
//
// Called from App.FinalizeBlock after BaseApp.FinalizeBlock. Decoding hits the
// tx-decode cache (these txs were just executed). FinalizeBlock and Commit are
// serialized by ABCI; pendingMu only guards against stray RPC concurrency.
func (a *Admitter) StageRecheckSenders(height int64, txs [][]byte) {
	// Stage height before the dep guard so the timeout sweep runs even if the
	// recheck deps (signer/decoder) aren't wired.
	a.pendingMu.Lock()
	a.committedHeight = height
	a.pendingMu.Unlock()

	if a.signer == nil || a.decoder == nil {
		return
	}
	senders := make(map[string]struct{}, len(txs))
	for _, bz := range txs {
		tx, err := a.decoder(bz)
		if err != nil {
			continue // non-sdk txs (e.g. vote extensions) have no mempool entry
		}
		for _, s := range a.signerKeys(tx) {
			senders[s] = struct{}{}
		}
	}
	a.pendingMu.Lock()
	a.pending = senders
	a.pendingMu.Unlock()
}

// RecheckLocked evicts pool txs invalidated by the last block: those whose
// TimeoutHeight has passed (any sender), and those of senders touched by the
// block that now fail the AnteHandler in ReCheck mode (stale sequence, drained
// balance). The caller MUST hold a.mu (App.Commit does): recheck mutates
// checkState, which is reset to the committed state post-Commit.
//
// The timeout sweep runs every commit (it needs no touched sender); ante recheck
// runs only for pending senders. ExecModeReCheck skips signature verification
// (the dominant CheckTx cost) and validate-basic, and BaseApp.RunTx auto-removes
// a tx from the mempool when its ante fails — so this only runs ante and evicts
// our encCache for the casualties. The candidate scan is O(pool depth).
func (a *Admitter) RecheckLocked() {
	if a.mpool == nil {
		return
	}
	a.pendingMu.Lock()
	pending := a.pending
	height := a.committedHeight
	a.pending = nil
	a.pendingMu.Unlock()
	// Nothing to do before the first committed block (height 0) with no pending
	// senders. In steady state height > 0, so the sweep always scans.
	if len(pending) == 0 && height == 0 {
		return
	}

	// Snapshot pointers under the pool lock; match senders after release.
	// signerKeys allocs per tx and RunTx's Remove can't run inside SelectBy,
	// which holds mp.mtx and blocks admission/reap. Matches reap.go.
	snapshot := make([]sdk.Tx, 0, a.mpool.CountTx())
	sdkmempool.SelectBy(context.Background(), a.mpool, nil, func(tx sdk.Tx) bool {
		snapshot = append(snapshot, tx)
		return true
	})

	var (
		candidates     []sdk.Tx
		expiredEvicted float32
	)
	for _, tx := range snapshot {
		if txExpired(tx, height) {
			// Evict directly (no RunTx): the next block's height already exceeds
			// TimeoutHeight, so the ante would reject it forever. Safe here because
			// the snapshot is materialized — Remove takes the pool lock, which the
			// SelectBy above has released.
			_ = a.mpool.Remove(tx)
			a.encCache.Evict(tx)
			expiredEvicted++
			continue
		}
		if len(pending) == 0 {
			continue
		}
		for _, s := range a.signerKeys(tx) {
			if _, ok := pending[s]; ok {
				candidates = append(candidates, tx)
				break
			}
		}
	}
	if expiredEvicted > 0 {
		telemetry.IncrCounter(expiredEvicted, "cronos", "mempool", "recheck", "expired") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
	}

	var evicted float32
	for _, tx := range candidates {
		bz, ok := a.encCache.Bytes(tx)
		if !ok {
			var err error
			if bz, err = a.txEncoder(tx); err != nil {
				continue
			}
		}
		if _, _, _, err := a.runner.RunTx(sdk.ExecModeReCheck, bz, tx, -1, nil, nil); err != nil {
			// BaseApp.RunTx attempts mpool.Remove on ante failure; drop our cache
			// entry regardless (Evict is a no-op if tx was never/already gone).
			a.encCache.Evict(tx)
			evicted++
		}
	}
	if evicted > 0 {
		telemetry.IncrCounter(evicted, "cronos", "mempool", "recheck", "evicted") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
	}
}

// txExpired reports whether tx's TimeoutHeight has passed. A tx is valid only
// in blocks H <= TimeoutHeight, and the next block is committedHeight+1, so
// committedHeight >= TimeoutHeight means it can never be valid again. Zero
// TimeoutHeight (e.g. EVM txs) means no timeout.
func txExpired(tx sdk.Tx, committedHeight int64) bool {
	t, ok := tx.(sdk.TxWithTimeoutHeight)
	if !ok {
		return false
	}
	th := t.GetTimeoutHeight()
	return th > 0 && uint64(committedHeight) >= th
}

// signerKeys returns the sender address strings for tx, or nil if extraction
// fails. The same adapter keys both staging and the pool scan, so keys match.
func (a *Admitter) signerKeys(tx sdk.Tx) []string {
	sigs, err := a.signer.GetSigners(tx)
	if err != nil {
		return nil
	}
	keys := make([]string, len(sigs))
	for i, s := range sigs {
		keys[i] = s.Signer.String()
	}
	return keys
}
