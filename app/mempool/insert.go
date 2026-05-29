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
}

// Compile-time check: *baseapp.BaseApp implements txRunner.
var _ txRunner = (*baseapp.BaseApp)(nil)

// NewInsertTxHandler returns an sdk.InsertTxHandler that validates peer-relayed
// txs via RunTx(execModeCheck) before admitting them to the mempool.
//
// A FIFO seen-cache of size cacheSize (0 = disabled) deduplicates AnteHandler
// invocations when the same tx is gossiped by multiple peers. The cache is a
// short-window dedup mechanism: a tx that was valid at first admission and
// recorded in the cache is unconditionally returned as CodeTypeOK on gossip
// re-delivery, even if it has since become invalid (nonce consumed, account
// drained, key rotated). The mempool itself still rejects duplicates on
// Insert, and stale entries are eventually evicted from the ring, so the
// practical risk is bounded by the gossip window. The cache does not
// interact with mempool eviction signals.
//
// Must be registered with BaseApp.SetInsertTxHandler before Seal.
func NewInsertTxHandler(app *baseapp.BaseApp, cacheSize int) sdk.InsertTxHandler {
	return newInsertTxHandler(app, cacheSize)
}

func newInsertTxHandler(runner txRunner, cacheSize int) sdk.InsertTxHandler {
	var cache *insertSeenCache
	if cacheSize > 0 {
		cache = newInsertSeenCache(cacheSize)
	}
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		var hash [32]byte
		if cache != nil {
			hash = sha256.Sum256(req.Tx)
			if cache.Has(hash) {
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
		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

// insertSeenCache is a fixed-size FIFO ring buffer of tx-hashes used to skip
// redundant AnteHandler runs when multiple peers gossip the same tx.
type insertSeenCache struct {
	mu   sync.Mutex
	ring [][32]byte
	set  map[[32]byte]struct{}
	pos  int
	n    int
	max  int
}

func newInsertSeenCache(max int) *insertSeenCache {
	return &insertSeenCache{
		ring: make([][32]byte, max),
		set:  make(map[[32]byte]struct{}, max),
		max:  max,
	}
}

func (c *insertSeenCache) Has(h [32]byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
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
