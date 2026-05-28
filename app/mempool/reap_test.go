package mempool_test

import (
	"context"
	"errors"
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
