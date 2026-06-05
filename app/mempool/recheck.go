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
// RecheckLocked can re-validate only their remaining pending txs. CometBFT's
// app-mempool Update() is a no-op, so the app drives recheck itself.
//
// Called from App.FinalizeBlock after BaseApp.FinalizeBlock. Decoding hits the
// tx-decode cache (these txs were just executed). FinalizeBlock and Commit are
// serialized by ABCI; pendingMu only guards against stray RPC concurrency.
func (a *Admitter) StageRecheckSenders(txs [][]byte) {
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

// RecheckLocked re-runs the AnteHandler in ReCheck mode against pending txs from
// senders touched by the last block, evicting any now-invalid (stale sequence,
// drained balance). The caller MUST hold a.mu (App.Commit does): recheck mutates
// checkState, which is reset to the committed state post-Commit.
//
// ExecModeReCheck skips signature verification (the dominant CheckTx cost) and
// validate-basic, and BaseApp.RunTx auto-removes a tx from the mempool when its
// ante fails — so this only runs ante and evicts our encCache for the casualties.
// The ante (recheck) work scales with touched senders, but the candidate scan
// below is O(pool depth): SelectBy holds the pool lock for the full iteration.
func (a *Admitter) RecheckLocked() {
	if a.mpool == nil || a.signer == nil {
		return
	}
	a.pendingMu.Lock()
	pending := a.pending
	a.pending = nil
	a.pendingMu.Unlock()
	if len(pending) == 0 {
		return
	}

	// Collect candidates under the pool lock, then recheck after iteration:
	// RunTx removes failures via mpool.Remove, which must not run inside SelectBy.
	var candidates []sdk.Tx
	sdkmempool.SelectBy(context.Background(), a.mpool, nil, func(tx sdk.Tx) bool {
		for _, s := range a.signerKeys(tx) {
			if _, ok := pending[s]; ok {
				candidates = append(candidates, tx)
				break
			}
		}
		return true
	})

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
