package mempool

import (
	"crypto/sha256"
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// DefaultInsertTxCacheSize is the default FIFO seen-cache size for NewInsertTxHandler.
const DefaultInsertTxCacheSize = 16384

// txRunner is the subset of baseapp.BaseApp used by NewInsertTxHandler.
// *baseapp.BaseApp satisfies this interface; tests may inject stubs.
type txRunner interface {
	RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, txMultiStore storetypes.MultiStore, incarnationCache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
	LastBlockHeight() int64
}

// Compile-time check: *baseapp.BaseApp implements txRunner.
var _ txRunner = (*baseapp.BaseApp)(nil)

// NewInsertTxHandler returns an sdk.InsertTxHandler that validates peer-relayed
// txs via RunTx(execModeCheck) before admitting them to the mempool.
//
// A FIFO seen-cache of size cacheSize (0 = disabled) deduplicates AnteHandler
// invocations when the same tx is gossiped by multiple peers. The cache is
// scoped to a single block height: it is fully cleared whenever the committed
// block height advances, so stale entries from a tx whose nonce was consumed,
// account drained, or signing key rotated cannot survive across a block
// commit. Within a height, the cache assumes a short gossip window and does
// not interact with mempool eviction signals; the mempool itself still
// rejects duplicates on Insert.
//
// The cache keys on raw wire bytes (SHA256(req.Tx)) by design: it only skips
// byte-identical re-delivery. A re-encoded (non-minimal) copy of the same tx is
// a distinct key, validated through the AnteHandler once — not a bypass, just a
// forfeited cache hit. Canonical keying would force a decode before the lookup,
// defeating the purpose for no security gain.
//
// DoS note: every gossiped tx not in the seen-cache costs one
// RunTx(ExecModeCheck) (a secp256k1 signature verification). A flood of distinct
// well-formed txs is bounded only by the p2p layer, so rely on CometBFT peer
// limits / rate limiting — not this handler — for gossip-flood protection.
//
// If encCache is non-nil, InsertTxHandler registers the canonical bytes for
// each successfully-admitted tx so that ReapTxsHandler can skip proto.Marshal
// on the reap hot path. txGet and txEncoder must both be non-nil when encCache
// is non-nil; newInsertTxHandler panics at construction otherwise.
//
// Must be registered with BaseApp.SetInsertTxHandler before Seal.
func NewInsertTxHandler(app *baseapp.BaseApp, cacheSize int, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder) sdk.InsertTxHandler {
	return newInsertTxHandler(app, cacheSize, txGet, encCache, txEncoder)
}

func newInsertTxHandler(runner txRunner, cacheSize int, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder) sdk.InsertTxHandler {
	if encCache != nil {
		if txGet == nil {
			panic("mempool: encCache requires txGet != nil")
		}
		if txEncoder == nil {
			panic("mempool: encCache requires txEncoder != nil: nil txEncoder risks non-canonical bytes in proposals")
		}
	}
	var cache *insertSeenCache
	if cacheSize > 0 {
		cache = newInsertSeenCache(cacheSize)
	}
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		var hash [32]byte
		if cache != nil {
			hash = sha256.Sum256(req.Tx)
			if cache.HasAtHeight(hash, runner.LastBlockHeight()) {
				return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
			}
		}

		_, _, _, err := runner.RunTx(sdk.ExecModeCheck, req.Tx, nil, -1, nil, nil)
		if err != nil {
			if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
				return &abci.ResponseInsertTx{Code: abci.CodeTypeRetry}, nil
			}
			_, code, _ := errorsmod.ABCIInfo(err, false)
			return &abci.ResponseInsertTx{Code: code}, nil
		}

		if cache != nil {
			cache.Add(hash)
		}

		// Register canonical bytes in the encoder cache so ReapTxsHandler can
		// skip proto.Marshal. Re-encoding from the decoded tx ensures canonical
		// proto bytes are stored even if req.Tx arrived with non-minimal encoding.
		if encCache != nil {
			if tx, ok := txGet(req.Tx); ok {
				bz := req.Tx
				if canonical, err := txEncoder(tx); err == nil {
					bz = canonical
				}
				encCache.Register(tx, bz)
			}
		}

		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

// insertSeenCache is a fixed-size FIFO ring buffer of tx-hashes used to skip
// redundant AnteHandler runs when multiple peers gossip the same tx. Entries
// are scoped to a single block height: HasAtHeight clears the cache the
// first time it observes a height greater than the last observed one.
type insertSeenCache struct {
	mu         sync.Mutex
	ring       [][32]byte
	set        map[[32]byte]struct{}
	pos        int
	n          int
	max        int
	lastHeight int64
}

func newInsertSeenCache(max int) *insertSeenCache {
	return &insertSeenCache{
		ring: make([][32]byte, max),
		set:  make(map[[32]byte]struct{}, max),
		max:  max,
	}
}

// HasAtHeight reports whether h is in the cache, after first evicting all
// entries if height advanced since the last call. Eviction is required so a
// tx whose nonce was consumed, account drained, or signing key rotated in a
// committed block is re-validated through the AnteHandler instead of being
// admitted as a stale cache hit.
func (c *insertSeenCache) HasAtHeight(h [32]byte, height int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if height > c.lastHeight {
		clear(c.set)
		c.pos = 0
		c.n = 0
		c.lastHeight = height
	}
	_, ok := c.set[h]
	return ok
}

func (c *insertSeenCache) Add(h [32]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n >= c.max {
		delete(c.set, c.ring[c.pos])
	} else {
		c.n++
	}
	c.ring[c.pos] = h
	c.set[h] = struct{}{}
	c.pos = (c.pos + 1) % c.max
}
