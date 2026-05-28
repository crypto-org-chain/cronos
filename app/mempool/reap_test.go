package mempool_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	protov2 "google.golang.org/protobuf/proto"

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

func encoderFixedWire(tx sdk.Tx) ([]byte, error) {
	if s, ok := tx.(*stubFeeTx); ok {
		return s.wire, nil
	}
	return nil, errors.New("unsupported tx type")
}

func newPool(n int, gasPerTx uint64, sizePerTx int) *stubMempool {
	pool := &stubMempool{}
	for i := 0; i < n; i++ {
		pool.txs = append(pool.txs, &stubFeeTx{gas: gasPerTx, wire: make([]byte, sizePerTx)})
	}
	return pool
}

func TestReapTxs_GasCap(t *testing.T) {
	pool := newPool(10_000, 50_000, 200) // 10K txs, 50K gas each
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire)

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
	pool := newPool(1_000, 50_000, 1_024) // 1024B per tx
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire)

	resp, err := h(&abci.RequestReapTxs{MaxBytes: 100 * 1_024, MaxGas: 0})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got, want := len(resp.Txs), 100; got != want {
		t.Fatalf("len(resp.Txs) = %d, want %d", got, want)
	}
}

func TestReapTxs_NoCapReturnsAll(t *testing.T) {
	pool := newPool(50, 1, 8)
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire)

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
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire)

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
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire)

	resp, err := h(&abci.RequestReapTxs{MaxBytes: 0, MaxGas: 50_000})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(resp.Txs); got != 0 {
		t.Fatalf("expected 0 txs, got %d", got)
	}
}

func TestInsertTx_OK(t *testing.T) {
	pool := &stubMempool{}
	h := cronosmempool.NewInsertTxHandler(pool, func(b []byte) (sdk.Tx, error) {
		return &stubFeeTx{gas: 1, wire: b}, nil
	})

	resp, err := h(&abci.RequestInsertTx{Tx: []byte("hello")})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Code != abci.CodeTypeOK {
		t.Fatalf("Code = %d, want %d", resp.Code, abci.CodeTypeOK)
	}
	if pool.CountTx() != 1 {
		t.Fatalf("pool size = %d, want 1", pool.CountTx())
	}
}

func TestInsertTx_DecodeFails(t *testing.T) {
	pool := &stubMempool{}
	h := cronosmempool.NewInsertTxHandler(pool, func(_ []byte) (sdk.Tx, error) {
		return nil, errors.New("bad tx")
	})

	resp, err := h(&abci.RequestInsertTx{Tx: []byte("garbage")})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Code != 1 {
		t.Fatalf("Code = %d, want 1 (permanent reject)", resp.Code)
	}
}

// signerTx wraps stubFeeTx with explicit sender/nonce so a fixed
// SignerExtractor can route it into PriorityNonceMempool without needing
// a full SigVerifiableTx implementation.
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

// TestReapTxs_ConcurrentInsertRace exercises the SelectBy lock semantics
// of the production reap path against a real PriorityNonceMempool. Run
// with `go test -race`. Pre-60ed80ad the handler used Select() and iterated
// without holding mp.mtx; concurrent Insert mutated the priority skiplist
// mid-iteration. Under -race that path produces "concurrent map read and
// map write" or skiplist data-race reports. After the fix the snapshot
// runs entirely under SelectBy's lock, so this test must not race.
func TestReapTxs_ConcurrentInsertRace(t *testing.T) {
	mp := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: fixedSignerExtractor{},
		MaxTx:           10_000,
	})

	const writers = 4
	const txsPerWriter = 500
	handler := cronosmempool.NewReapTxsHandler(mp, encoderFixedWire)
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
					stubFeeTx: &stubFeeTx{gas: 1, wire: []byte{byte(id), byte(n)}},
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
}
