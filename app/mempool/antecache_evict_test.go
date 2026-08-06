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

type ethTx struct {
	msg *evmtypes.MsgEthereumTx
}

func (t *ethTx) GetMsgs() []sdk.Msg                    { return []sdk.Msg{t.msg} }
func (t *ethTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func newEthTx(from common.Address, nonce uint64) *ethTx {
	msg := evmtypes.NewTx(nil, nonce, &common.Address{0xab}, big.NewInt(0), 21000, big.NewInt(1), nil, nil, nil, nil)
	msg.From = from.Bytes()
	return &ethTx{msg: msg}
}

func anteKey(tx *ethTx) (string, uint64) {
	return tx.msg.GetFrom().String(), tx.msg.AsTransaction().Nonce()
}

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

func TestEvict_NilAnteCacheNoPanic(t *testing.T) {
	tx := newEthTx(common.Address{0x2}, 1)
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.mpool = &fakePool{txs: []sdk.Tx{tx}}

	a.evict(tx) // anteCache nil: must be a no-op, not a panic
}

func TestEvict_NonEthMsgSkipsAnteCache(t *testing.T) {
	tx := &ptrTx{id: 1}
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.mpool = &fakePool{txs: []sdk.Tx{tx}}
	a.SetAnteCache(antecache.NewAnteCache(0))

	a.evict(tx)
}

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
