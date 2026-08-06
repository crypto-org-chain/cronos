package mempool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	antecache "github.com/evmos/ethermint/ante/cache"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

// ethTx wraps a real *evmtypes.MsgEthereumTx so evictAnteCache's type assertion
// succeeds, unlike the plain ptrTx fixture used elsewhere in this package.
type ethTx struct {
	msg *evmtypes.MsgEthereumTx
}

func (t *ethTx) GetMsgs() []sdk.Msg                    { return []sdk.Msg{t.msg} }
func (t *ethTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

// newEthTx builds a legacy MsgEthereumTx for `from` at `nonce`, unsigned: evict
// only reads GetFrom/AsTransaction, neither of which requires a valid signature.
func newEthTx(from common.Address, nonce uint64) *ethTx {
	msg := evmtypes.NewTx(nil, nonce, &common.Address{0xab}, big.NewInt(0), 21000, big.NewInt(1), nil, nil, nil, nil)
	msg.From = from.Bytes()
	return &ethTx{msg: msg}
}

func anteKey(tx *ethTx) (string, uint64) {
	return tx.msg.GetFrom().String(), tx.msg.AsTransaction().Nonce()
}

// TestEvict_ClearsAnteCacheEntry proves the TTL/timeout eviction path (evict,
// shared by RecheckTxs' expiry/TTL sweep and the post-recheck-failure path)
// removes the tx's ante-cache nonce entry instead of leaking it.
func TestEvict_ClearsAnteCacheEntry(t *testing.T) {
	tx := newEthTx(common.Address{0x1}, 5)
	addr, nonce := anteKey(tx)

	ac := antecache.NewAnteCache(0)
	ac.Set(addr, nonce)
	if !ac.Exists(addr, nonce) {
		t.Fatal("precondition: entry must be cached before eviction")
	}

	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.mpool = &fakePool{txs: []sdk.Tx{tx}}
	a.SetAnteCache(ac)

	a.evict(tx)

	if ac.Exists(addr, nonce) {
		t.Fatal("evict must clear the tx's ante-cache entry, not leak it")
	}
}

// TestEvict_NilAnteCacheNoPanic: Manager built without SetAnteCache (e.g. tests,
// or a future caller that doesn't wire one) must not panic on eviction.
func TestEvict_NilAnteCacheNoPanic(t *testing.T) {
	tx := newEthTx(common.Address{0x2}, 1)
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.mpool = &fakePool{txs: []sdk.Tx{tx}}

	a.evict(tx) // anteCache nil: must be a no-op, not a panic
}

// TestEvict_NonEthMsgSkipsAnteCache: a non-eth tx (e.g. ptrTx, standing in for a
// cosmos-native message) has no ante-cache entry to clear; evict must not panic
// walking its (empty) message list.
func TestEvict_NonEthMsgSkipsAnteCache(t *testing.T) {
	tx := &ptrTx{id: 1}
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.mpool = &fakePool{txs: []sdk.Tx{tx}}
	a.SetAnteCache(antecache.NewAnteCache(0))

	a.evict(tx)
}

// TestRecheckTxs_TTLEvictionClearsAnteCache drives the eviction through the real
// TTL sweep (selectTxs -> evictForRecheck -> evict) instead of calling evict
// directly, proving the recheck-triggered eviction path also clears the cache.
func TestRecheckTxs_TTLEvictionClearsAnteCache(t *testing.T) {
	f := newRecheckFixture()
	f.a.ttlNumBlocks = 5
	ac := antecache.NewAnteCache(0)
	f.a.SetAnteCache(ac)

	tx := newEthTx(common.Address{0x3}, 7)
	addr, nonce := anteKey(tx)
	ac.Set(addr, nonce)

	f.signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("carol"), 0)}
	if err := f.pool.Insert(sdk.Context{}, tx); err != nil {
		t.Fatal(err)
	}
	f.enc.Set(tx, []byte("carol-7"))

	f.a.lastCommittedHeight = 10 // first sighting: arrival=10
	f.a.RecheckTxs()
	if !ac.Exists(addr, nonce) {
		t.Fatal("tx must survive (and keep its ante-cache entry) before TTL expiry")
	}

	f.a.lastCommittedHeight = 15 // 15-10 == ttl → evicted
	f.a.RecheckTxs()

	if poolHas(f.pool, tx) {
		t.Fatal("TTL-expired tx must be evicted from the pool")
	}
	if ac.Exists(addr, nonce) {
		t.Fatal("TTL eviction must clear the ante-cache entry, not leak it")
	}
}

// TestRecheckTxs_RecheckFailureClearsAnteCache drives the runRecheck eviction
// path (RunTx(ReCheck) failure -> evict), the other call site sharing evict.
func TestRecheckTxs_RecheckFailureClearsAnteCache(t *testing.T) {
	tx := newEthTx(common.Address{0x4}, 2)
	addr, nonce := anteKey(tx)

	f := newRecheckFixture("dave-2")
	ac := antecache.NewAnteCache(0)
	f.a.SetAnteCache(ac)
	ac.Set(addr, nonce)

	f.signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("dave"), 0)}
	if err := f.pool.Insert(sdk.Context{}, tx); err != nil {
		t.Fatal(err)
	}
	f.enc.Set(tx, []byte("dave-2"))

	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("dave").String(): {}}
	f.a.RecheckTxs()

	if poolHas(f.pool, tx) {
		t.Fatal("tx failing recheck must be removed from the pool")
	}
	if ac.Exists(addr, nonce) {
		t.Fatal("recheck-failure eviction must clear the ante-cache entry, not leak it")
	}
}

// TestEvictAnteCache_MultipleMsgsClearsAll: a tx with more than one eth message
// (batched) must clear every message's nonce entry, not just the first.
func TestEvictAnteCache_MultipleMsgsClearsAll(t *testing.T) {
	msg1 := newEthTx(common.Address{0x5}, 1)
	msg2 := newEthTx(common.Address{0x6}, 9)
	ac := antecache.NewAnteCache(0)
	addr1, nonce1 := anteKey(msg1)
	addr2, nonce2 := anteKey(msg2)
	ac.Set(addr1, nonce1)
	ac.Set(addr2, nonce2)

	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.SetAnteCache(ac)

	multi := &multiMsgTx{msgs: []sdk.Msg{msg1.msg, msg2.msg}}
	a.mpool = &fakePool{txs: []sdk.Tx{multi}}
	a.evict(multi)

	if ac.Exists(addr1, nonce1) || ac.Exists(addr2, nonce2) {
		t.Fatal("evict must clear every eth message's ante-cache entry")
	}
}

type multiMsgTx struct{ msgs []sdk.Msg }

func (t *multiMsgTx) GetMsgs() []sdk.Msg                    { return t.msgs }
func (t *multiMsgTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
