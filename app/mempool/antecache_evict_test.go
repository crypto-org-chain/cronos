package mempool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	antecache "github.com/evmos/ethermint/ante/cache"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/suite"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
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

type multiMsgTx struct{ msgs []sdk.Msg }

// GetMsgs implements sdk.Tx.
func (t *multiMsgTx) GetMsgs() []sdk.Msg { return t.msgs }

// GetMsgsV2 implements sdk.Tx.
func (t *multiMsgTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

type AnteCacheEvictTestSuite struct {
	suite.Suite
}

// TestAnteCacheEvictTestSuite runs the AnteCacheEvictTestSuite.
func TestAnteCacheEvictTestSuite(t *testing.T) {
	suite.Run(t, new(AnteCacheEvictTestSuite))
}

// TestEvictScenarios covers the ways a tx's ante-cache entry gets cleared on eviction.
func (s *AnteCacheEvictTestSuite) TestEvictScenarios() {
	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"direct evict clears the tx's ante-cache entry", evictClearsAnteCacheEntry},
		{"nil ante cache is a no-op", evictNilAnteCacheNoPanic},
		{"non-eth message is skipped", evictNonEthMsgSkipsAnteCache},
		{"TTL eviction clears the entry", evictTTLEvictionClearsAnteCache},
		{"recheck failure clears the entry", evictRecheckFailureClearsAnteCache},
		{"multiple eth messages clear all entries", evictMultipleMsgsClearsAll},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() { tc.run(s.T()) })
	}
}

func evictClearsAnteCacheEntry(t *testing.T) {
	t.Helper()
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

func evictNilAnteCacheNoPanic(t *testing.T) {
	t.Helper()
	tx := newEthTx(common.Address{0x2}, 1)
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.mpool = &fakePool{txs: []sdk.Tx{tx}}

	a.evict(tx) // anteCache nil: must be a no-op, not a panic
}

func evictNonEthMsgSkipsAnteCache(t *testing.T) {
	t.Helper()
	tx := &ptrTx{id: 1}
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.mpool = &fakePool{txs: []sdk.Tx{tx}}
	a.SetAnteCache(antecache.NewAnteCache(0))

	a.evict(tx)
}

func evictTTLEvictionClearsAnteCache(t *testing.T) {
	t.Helper()
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

func evictRecheckFailureClearsAnteCache(t *testing.T) {
	t.Helper()
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

func evictMultipleMsgsClearsAll(t *testing.T) {
	t.Helper()
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
