package mempool_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// stubFeeTx implements sdk.FeeTx with a configurable gas value and a
// distinct on-the-wire payload of `size` bytes.
type stubFeeTx struct {
	gas  uint64
	wire []byte
}

func (s *stubFeeTx) GetMsgs() []sdk.Msg                    { return nil }
func (s *stubFeeTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (s *stubFeeTx) GetGas() uint64                        { return s.gas }
func (s *stubFeeTx) GetFee() sdk.Coins                     { return nil }
func (s *stubFeeTx) FeePayer() []byte                      { return nil }
func (s *stubFeeTx) FeeGranter() []byte                    { return nil }

type stubIterator struct {
	txs []sdk.Tx
	i   int
}

func (it *stubIterator) Next() sdkmempool.Iterator {
	it.i++
	if it.i >= len(it.txs) {
		return nil
	}
	return it
}

func (it *stubIterator) Tx() sdk.Tx {
	if it.i >= len(it.txs) {
		return nil
	}
	return it.txs[it.i]
}

type stubMempool struct {
	txs []sdk.Tx
}

func (m *stubMempool) Insert(_ context.Context, tx sdk.Tx) error {
	m.txs = append(m.txs, tx)
	return nil
}

func (m *stubMempool) Select(_ context.Context, _ [][]byte) sdkmempool.Iterator {
	if len(m.txs) == 0 {
		return nil
	}
	return &stubIterator{txs: m.txs}
}
func (m *stubMempool) CountTx() int          { return len(m.txs) }
func (m *stubMempool) Remove(_ sdk.Tx) error { return nil }

// minimal pool helpers below

// newReapHandler builds a reap handler with the gossip throttle effectively
// disabled (ttl far exceeds a test's duration so a fresh handler's first reap
// returns everything; no count cap), isolating the pre-existing cap/ordering
// tests from gossip dedup. Throttle behavior is covered separately.
func newReapHandler(mp sdkmempool.Mempool, enc sdk.TxEncoder, cache *cronosmempool.EncoderCache) sdk.ReapTxsHandler {
	return cronosmempool.NewReapTxsHandler(mp, enc, cache, time.Hour, 0, log.NewNopLogger())
}

func encoderFixedWire(tx sdk.Tx) ([]byte, error) {
	switch s := tx.(type) {
	case *stubFeeTx:
		return s.wire, nil
	case *signerTx:
		return s.wire, nil
	}
	return nil, errors.New("unsupported tx type")
}

func newPool(n int, gasPerTx uint64, sizePerTx int) *stubMempool {
	pool := &stubMempool{}
	for i := 0; i < n; i++ {
		wire := make([]byte, sizePerTx)
		// Unique payload (length unchanged) so the gossip dedup, which hashes
		// wire bytes, doesn't collapse otherwise-identical stub txs.
		binary.PutUvarint(wire, uint64(i+1))
		pool.txs = append(pool.txs, &stubFeeTx{gas: gasPerTx, wire: wire})
	}
	return pool
}

func TestReapTxs_GasCap(t *testing.T) {
	pool := newPool(10_000, 50_000, 200) // 10K txs, 50K gas each
	h := newReapHandler(pool, encoderFixedWire, nil)

	resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 80_000_000})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// 80M / 50K = 1600 txs
	if got, want := len(resp.Txs), 1600; got != want {
		t.Fatalf("len(resp.Txs) = %d, want %d", got, want)
	}
}

func TestReapTxs_BytesCap(t *testing.T) {
	const txPayload = 1_024
	pool := newPool(1_000, 50_000, txPayload)
	h := newReapHandler(pool, encoderFixedWire, nil)

	// Use proto-framed size so MaxBytes is an exact multiple of the per-tx
	// wire cost — identical to how PrepareProposal accounts bytes.
	protoSize := uint64(cronosmempool.ProtoSizeForTx(make([]byte, txPayload)))
	const wantTxs = 100
	resp, err := h(&abci.RequestReapTxs{MaxBytes: wantTxs * protoSize, MaxGas: 0})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(resp.Txs); got != wantTxs {
		t.Fatalf("len(resp.Txs) = %d, want %d", got, wantTxs)
	}
}

func TestReapTxs_NoCapReturnsAll(t *testing.T) {
	pool := newPool(50, 1, 8)
	h := newReapHandler(pool, encoderFixedWire, nil)

	resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got, want := len(resp.Txs), 50; got != want {
		t.Fatalf("len(resp.Txs) = %d, want %d", got, want)
	}
}

func TestReapTxs_EmptyPool(t *testing.T) {
	pool := &stubMempool{}
	h := newReapHandler(pool, encoderFixedWire, nil)

	resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resp.Txs) != 0 {
		t.Fatalf("expected empty txs, got %d", len(resp.Txs))
	}
}

func TestReapTxs_SingleTxExceedsGasCap(t *testing.T) {
	// one tx requiring more gas than the cap -> cap wins, return empty
	pool := newPool(1, 100_000, 8)
	h := newReapHandler(pool, encoderFixedWire, nil)

	resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 50_000})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(resp.Txs); got != 0 {
		t.Fatalf("expected 0 txs, got %d", got)
	}
}

// signerTx wraps stubFeeTx with explicit sender/nonce so a fixed
// SignerExtractor can route it into PriorityNonceMempool without a full
// SigVerifiableTx implementation.
type signerTx struct {
	*stubFeeTx
	sender sdk.AccAddress
	nonce  uint64
}

type fixedSignerExtractor struct{}

func (fixedSignerExtractor) GetSigners(tx sdk.Tx) ([]sdkmempool.SignerData, error) {
	s := tx.(*signerTx)
	return []sdkmempool.SignerData{{Signer: s.sender, Sequence: s.nonce}}, nil
}

func TestReapTxs_NonceOrderingPerSender(t *testing.T) {
	mp := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: fixedSignerExtractor{},
		MaxTx:           100,
	})
	sender := sdk.AccAddress("sender00-padding-bytes")
	ctx := sdk.Context{}.WithContext(context.Background()).WithPriority(0)

	for _, n := range []byte{3, 0, 4, 1, 2} {
		tx := &signerTx{
			stubFeeTx: &stubFeeTx{gas: 21_000, wire: []byte{n}},
			sender:    sender,
			nonce:     uint64(n),
		}
		if err := mp.Insert(ctx, tx); err != nil {
			t.Fatalf("insert nonce=%d: %v", n, err)
		}
	}

	reap := newReapHandler(mp, encoderFixedWire, nil)
	resp, err := reap(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if got, want := len(resp.Txs), 5; got != want {
		t.Fatalf("len(resp.Txs) = %d, want %d", got, want)
	}
	for i, tx := range resp.Txs {
		if got, want := tx[0], byte(i); got != want {
			t.Fatalf("resp.Txs[%d][0] = %d, want %d (nonce ascending)", i, got, want)
		}
	}
}

func TestReapTxs_PriorityDescending(t *testing.T) {
	mp := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: fixedSignerExtractor{},
		MaxTx:           100,
	})
	reap := newReapHandler(mp, encoderFixedWire, nil)

	priorities := []int64{10, 100, 50, 200, 5}
	for i, p := range priorities {
		sender := sdk.AccAddress(fmt.Sprintf("sender%02d-padding-bytes", i))
		tx := &signerTx{
			stubFeeTx: &stubFeeTx{gas: 21_000, wire: []byte{byte(p)}},
			sender:    sender,
			nonce:     0,
		}
		ctx := sdk.Context{}.WithContext(context.Background()).WithPriority(p)
		if err := mp.Insert(ctx, tx); err != nil {
			t.Fatalf("insert priority=%d: %v", p, err)
		}
	}

	resp, err := reap(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if got, want := len(resp.Txs), len(priorities); got != want {
		t.Fatalf("len(resp.Txs) = %d, want %d", got, want)
	}
	want := []byte{200, 100, 50, 10, 5}
	for i, tx := range resp.Txs {
		if got := tx[0]; got != want[i] {
			t.Fatalf("resp.Txs[%d][0] = %d, want %d (priority desc)", i, got, want[i])
		}
	}
}

func TestReapTxs_UsesEncoderCacheForRegisteredTx(t *testing.T) {
	cached := &stubFeeTx{gas: 21_000, wire: []byte("ENCODER-FALLBACK-cached")}
	uncached := &stubFeeTx{gas: 21_000, wire: []byte("ENCODER-FALLBACK-uncached")}
	pool := &stubMempool{txs: []sdk.Tx{cached, uncached}}

	enc := cronosmempool.NewEncoderCache(0)
	canonical := []byte("CANONICAL-from-cache")
	enc.Register(cached, canonical)

	var encoderCalls []sdk.Tx
	countingEncoder := func(tx sdk.Tx) ([]byte, error) {
		encoderCalls = append(encoderCalls, tx)
		return encoderFixedWire(tx)
	}

	h := newReapHandler(pool, countingEncoder, enc)
	resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(resp.Txs); got != 2 {
		t.Fatalf("len(resp.Txs) = %d, want 2", got)
	}
	// Cached tx ships canonical cache bytes, not its raw wire.
	if string(resp.Txs[0]) != string(canonical) {
		t.Fatalf("cached tx reaped %q, want canonical %q", resp.Txs[0], canonical)
	}
	// Uncached tx falls back to the encoder.
	if string(resp.Txs[1]) != string(uncached.wire) {
		t.Fatalf("uncached tx reaped %q, want encoder wire %q", resp.Txs[1], uncached.wire)
	}
	// Fast path: encoder called exactly once, only for the uncached tx.
	if len(encoderCalls) != 1 || encoderCalls[0] != sdk.Tx(uncached) {
		t.Fatalf("encoder calls = %v, want exactly [uncached] (cached tx must skip proto.Marshal)", encoderCalls)
	}
}

func TestReapTxs_ConcurrentInsertRace(t *testing.T) {
	mp := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: fixedSignerExtractor{},
		MaxTx:           10_000,
	})

	const writers = 4
	const txsPerWriter = 500
	handler := newReapHandler(mp, encoderFixedWire, nil)
	insertCtx := sdk.Context{}.WithPriority(0)

	var wg sync.WaitGroup
	done := make(chan struct{})

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sender := sdk.AccAddress(fmt.Sprintf("sender%02d-padding-bytes", id))
			for n := uint64(0); n < txsPerWriter; n++ {
				tx := &signerTx{
					// 3-byte wire keeps every (id, nonce) distinct so the gossip
					// dedup (hashes wire bytes) doesn't collapse nonces that alias
					// mod 256; nonce reaches 499 > 255.
					stubFeeTx: &stubFeeTx{gas: 1, wire: []byte{byte(id), byte(n), byte(n >> 8)}},
					sender:    sender,
					nonce:     n,
				}
				if err := mp.Insert(insertCtx, tx); err != nil {
					t.Errorf("insert err: %v", err)
					return
				}
			}
		}(w)
	}

	var reapErr atomic.Value
	reaperDone := make(chan struct{})
	go func() {
		defer close(reaperDone)
		for {
			select {
			case <-done:
				return
			default:
			}
			if _, err := handler(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0}); err != nil {
				reapErr.Store(err)
				return
			}
		}
	}()

	wg.Wait()
	close(done)
	<-reaperDone

	if v := reapErr.Load(); v != nil {
		t.Fatalf("reap err: %v", v)
	}
	if got, want := mp.CountTx(), writers*txsPerWriter; got != want {
		t.Fatalf("CountTx = %d, want %d", got, want)
	}

	// Snapshot completeness on a stable pool, via a fresh handler so gossip
	// dedup state is empty. Every distinct tx (unique wire bytes) must be
	// reaped exactly once: dedup also collapses the upstream iterator's
	// same-priority double-emit (cosmos/cosmos-sdk#1751), so the unique-tx
	// count equals CountTx. A second reap on the same handler is fully deduped.
	fresh := newReapHandler(mp, encoderFixedWire, nil)
	resp1, err := fresh(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("final reap 1: %v", err)
	}
	if got, want := len(resp1.Txs), writers*txsPerWriter; got != want {
		t.Fatalf("fresh reap returned %d txs, want %d (all distinct txs once)", got, want)
	}
	resp2, err := fresh(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("final reap 2: %v", err)
	}
	if got := len(resp2.Txs); got != 0 {
		t.Fatalf("second reap returned %d txs, want 0 (all deduped within ttl)", got)
	}
}

func TestReapTxs_GossipDedupAcrossReaps(t *testing.T) {
	pool := newPool(50, 1, 8)
	// ttl far exceeds the test; second reap of the same resident pool must be
	// fully suppressed (this is the steady-state flood the throttle kills).
	h := newReapHandler(pool, encoderFixedWire, nil)

	resp1, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("reap 1: %v", err)
	}
	if got := len(resp1.Txs); got != 50 {
		t.Fatalf("reap 1 returned %d, want 50", got)
	}
	resp2, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("reap 2: %v", err)
	}
	if got := len(resp2.Txs); got != 0 {
		t.Fatalf("reap 2 returned %d, want 0 (deduped)", got)
	}
}

func TestReapTxs_GossipCountCap(t *testing.T) {
	pool := newPool(50, 1, 8)
	// cap 20/reap, ttl huge: the pool drains over successive reaps (20, 20, 10),
	// spreading a burst across ticks instead of one libp2p batch.
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire, nil, time.Hour, 20, log.NewNopLogger())

	seen := map[byte]struct{}{}
	wantPerReap := []int{20, 20, 10}
	for i, want := range wantPerReap {
		resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
		if err != nil {
			t.Fatalf("reap %d: %v", i, err)
		}
		if got := len(resp.Txs); got != want {
			t.Fatalf("reap %d returned %d txs, want %d", i, got, want)
		}
		for _, tx := range resp.Txs {
			if _, dup := seen[tx[0]]; dup {
				t.Fatalf("tx id %d reaped twice across capped reaps", tx[0])
			}
			seen[tx[0]] = struct{}{}
		}
	}
	if len(seen) != 50 {
		t.Fatalf("union of capped reaps = %d distinct txs, want 50", len(seen))
	}
	// Pool fully gossiped; next reap is empty until ttl elapses.
	resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 0})
	if err != nil {
		t.Fatalf("final reap: %v", err)
	}
	if got := len(resp.Txs); got != 0 {
		t.Fatalf("post-drain reap returned %d, want 0", got)
	}
}
