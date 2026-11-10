package mempool

import (
	"bytes"
	"container/list"
	cryptorand "crypto/rand"
	"encoding/binary"
	"math"
	"sync"

	"github.com/cespare/xxhash/v2"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
)

// xxhashSeed randomizes shard assignment so an attacker can't target one shard.
var xxhashSeed = func() uint64 {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(b[:])
}()

// shardCount is the number of independent LRU stripes in DecodeCache. Must be a
// power of two so shardMask can replace modulo in shard selection.
const shardCount = 16

// defaultCacheSize is the tx encode/decode cache capacity fallback for callers
// that construct a cache with size 0 directly.
const defaultCacheSize = cmdcfg.DefaultMaxTxPerBlock

// shardMask maps a hash to a shard via h&shardMask (== h%shardCount for a
// power-of-two shardCount).
const shardMask = shardCount - 1

// Compile-time assert shardCount is a power of two.
var _ = [1]struct{}{}[shardCount&shardMask]

type lruItem struct {
	h   uint64
	key []byte // raw tx bytes; resolves xxhash collisions
	tx  sdk.Tx
}

// cacheShard is one LRU stripe of DecodeCache with its own mutex, so access to
// different tx hashes is contention-free.
type cacheShard struct {
	mu    sync.Mutex
	cap   int
	items map[uint64]*list.Element
	lru   list.List // front = MRU, back = LRU; zero value is empty list
}

// get returns the cached tx for (h, bz). On hash collision returns (nil, false);
// the next set evicts the stale entry.
func (s *cacheShard) get(h uint64, bz []byte) (sdk.Tx, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[h]
	if !ok {
		return nil, false
	}
	item := el.Value.(*lruItem)
	if !bytes.Equal(item.key, bz) {
		return nil, false
	}
	s.lru.MoveToFront(el)
	return item.tx, true
}

func (s *cacheShard) set(h uint64, bz []byte, tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.items[h]; ok {
		item := el.Value.(*lruItem)
		if bytes.Equal(item.key, bz) {
			// Identical bytes already cached; promote to MRU.
			s.lru.MoveToFront(el)
			return
		}
		// Hash collision (rare): evict the old entry, insert the new one.
		delete(s.items, h)
		s.lru.Remove(el)
	}

	if s.lru.Len() >= s.cap {
		back := s.lru.Back()
		delete(s.items, back.Value.(*lruItem).h)
		s.lru.Remove(back)
	}
	s.items[h] = s.lru.PushFront(&lruItem{h: h, key: bytes.Clone(bz), tx: tx})
}

// DecodeCache is a sharded LRU cache of decoded txs. Returned sdk.Tx pointers
// are shared across consumers (PrepareProposal, runTx, CheckTx) — callers MUST
// NOT mutate them. Only EVM txs are cached, with lazy fields filled first (toBeCached); the EVM
// ante and the encoder only read them. A future EVM ante that writes would break sharing.
type DecodeCache struct {
	shards     [shardCount]cacheShard
	maxTxBytes int
}

// NewDecodeCache returns a cache with total capacity ~size and per-entry
// payload cap maxTxBytes. Pass 0 for either to use defaults.
func NewDecodeCache(size, maxTxBytes uint) *DecodeCache {
	if size == 0 {
		size = defaultCacheSize
	}
	if maxTxBytes == 0 {
		maxTxBytes = cmdcfg.DefaultTxCacheMaxTxBytes
	}
	shardCapUint := size / shardCount
	if size%shardCount != 0 {
		shardCapUint++
	}
	shardCap := math.MaxInt / shardCount
	if shardCapUint <= uint(shardCap) {
		shardCap = int(shardCapUint)
	}
	c := &DecodeCache{maxTxBytes: int(maxTxBytes)}
	for i := range c.shards {
		c.shards[i].cap = shardCap
		c.shards[i].items = make(map[uint64]*list.Element, preallocEntries(shardCap))
	}
	return c
}

func (c *DecodeCache) get(bz []byte) (sdk.Tx, bool) {
	h := xxhash.Sum64(bz) ^ xxhashSeed
	return c.shards[h&shardMask].get(h, bz)
}

func (c *DecodeCache) set(bz []byte, tx sdk.Tx) {
	h := xxhash.Sum64(bz) ^ xxhashSeed
	c.shards[h&shardMask].set(h, bz, tx)
}

// NewCachingDecoder wraps base with the shared cache, so PrepareProposal and
// BaseApp.runTx reuse each other's decodes.
func NewCachingDecoder(base sdk.TxDecoder, cache *DecodeCache, logger log.Logger) sdk.TxDecoder {
	if logger == nil { // a nil logger would re-panic inside toBeCached's recover
		logger = log.NewNopLogger()
	}
	return func(bz []byte) (sdk.Tx, error) {
		if tx, ok := cache.get(bz); ok {
			return tx, nil
		}
		tx, err := base(bz)
		if err != nil {
			return nil, err
		}
		if len(bz) <= cache.maxTxBytes && toBeCached(logger, bz, tx) {
			cache.set(bz, tx)
		}
		return tx, nil
	}
}

// ethTxTypeURL is the first extension option that routes a tx to ethermint's EVM ante.
var ethTxTypeURL = sdk.MsgTypeURL(&evmtypes.ExtensionOptionsEthereumTx{})

// toBeCached also fills tx's lazily written fields, so sharers only read it; false keeps tx private.
func toBeCached(logger log.Logger, bz []byte, tx sdk.Tx) (ok bool) {
	// A malformed tx must fail in runTx's recover, not in decode.
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(logger, panicPathDecodeCache, r, "tx", txHash(bz))
			ok = false
		}
	}()
	// Cosmos txs stay private: multisig Address() and legacy EIP-712 checks rewrite Any amino caches on every call,
	// and cosmos msg handlers aren't held to read-only.
	return isRawEthTx(tx) && hasFee(tx) && initSigners(tx)
}

// hasFee reports whether AuthInfo has a Fee: with it and msgs, the wire body and auth-info bytes
// the proto decoder kept are non-empty, so the encoder never fills them.
func hasFee(tx sdk.Tx) bool {
	protoTx, ok := tx.(interface{ GetProtoTx() *txtypes.Tx })
	return ok && protoTx.GetProtoTx().GetAuthInfo().GetFee() != nil
}

// initSigners fills the SDK wrapper's lazy signers and msgsV2, which it sets in one call and reruns while signers is nil.
func initSigners(tx sdk.Tx) bool {
	sigTx, isSigTx := tx.(interface{ GetSigners() ([][]byte, error) })
	if !isSigTx {
		return false
	}
	signers, err := sigTx.GetSigners()
	return err == nil && len(signers) > 0
}

// isRawEthTx reports whether tx takes the EVM ante with only Raw-format eth msgs.
func isRawEthTx(tx sdk.Tx) bool {
	extTx, hasExt := tx.(authante.HasExtensionOptionsTx)
	if !hasExt {
		return false
	}
	if opts := extTx.GetExtensionOptions(); len(opts) == 0 || opts[0].GetTypeUrl() != ethTxTypeURL {
		return false
	}
	msgs := tx.GetMsgs()
	if len(msgs) == 0 {
		return false
	}
	for _, msg := range msgs {
		// Legacy Data-format msgs get Raw written on first AsTransaction.
		if ethMsg, isEth := msg.(*evmtypes.MsgEthereumTx); !isEth || ethMsg.Raw.Transaction == nil {
			return false
		}
	}
	return true
}
