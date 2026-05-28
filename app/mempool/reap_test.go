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
		pool.txs = append(pool.txs, &stubFeeTx{gas: gasPerTx, wire: make([]byte, sizePerTx)})
	}
	return pool
}

func TestReapTxs_GasCap(t *testing.T) {
	pool := newPool(10_000, 50_000, 200) // 10K txs, 50K gas each
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire, log.NewNopLogger())

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
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire, log.NewNopLogger())

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
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire, log.NewNopLogger())

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
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire, log.NewNopLogger())

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
	h := cronosmempool.NewReapTxsHandler(pool, encoderFixedWire, log.NewNopLogger())

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

// TestInsertReap_NonceOrderingPerSender verifies that for a single sender,
// txs inserted via NewInsertTxHandler in shuffled nonce order are returned
// by NewReapTxsHandler in ascending nonce order — the per-sender invariant
// that downstream block proposers rely on.
func TestInsertReap_NonceOrderingPerSender(t *testing.T) {
	mp := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: fixedSignerExtractor{},
		MaxTx:           100,
	})
	sender := sdk.AccAddress("sender00-padding-bytes")

	decoder := func(b []byte) (sdk.Tx, error) {
		return &signerTx{
			stubFeeTx: &stubFeeTx{gas: 21_000, wire: b},
			sender:    sender,
			nonce:     uint64(b[0]),
		}, nil
	}
	insert := cronosmempool.NewInsertTxHandler(mp, decoder)
	reap := cronosmempool.NewReapTxsHandler(mp, encoderFixedWire, log.NewNopLogger())

	for _, n := range []byte{3, 0, 4, 1, 2} {
		resp, err := insert(&abci.RequestInsertTx{Tx: []byte{n}})
		if err != nil || resp.Code != abci.CodeTypeOK {
			t.Fatalf("insert nonce=%d: code=%d err=%v", n, resp.Code, err)
		}
	}

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

// TestReapTxs_PriorityDescending verifies that txs inserted with varying
// ctx.Priority() values are reaped highest-priority-first. Insertion bypasses
// NewInsertTxHandler (which always sets priority=0); this exercises the
// CheckTx path where AnteHandler stamps priority on the SDK ctx.
func TestReapTxs_PriorityDescending(t *testing.T) {
	mp := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: fixedSignerExtractor{},
		MaxTx:           100,
	})
	reap := cronosmempool.NewReapTxsHandler(mp, encoderFixedWire, log.NewNopLogger())

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

// priorityCapturingMempool records ctx.Priority() on each Insert call so
// tests can assert what priority value the handler stamped on the SDK ctx.
// Other Mempool methods are stubbed and not exercised here.
type priorityCapturingMempool struct {
	priorities []int64
}

func (m *priorityCapturingMempool) Insert(ctx context.Context, _ sdk.Tx) error {
	m.priorities = append(m.priorities, sdk.UnwrapSDKContext(ctx).Priority())
	return nil
}

func (*priorityCapturingMempool) Select(context.Context, [][]byte) sdkmempool.Iterator {
	return nil
}
func (m *priorityCapturingMempool) CountTx() int      { return len(m.priorities) }
func (*priorityCapturingMempool) Remove(sdk.Tx) error { return nil }

// TestInsertTxHandler_StampsPriorityZero locks in the contract that
// NewInsertTxHandler hands the underlying mempool an SDK ctx with
// priority=0, regardless of any "gas tip" the decoded tx might carry.
// The ABCI InsertTx hook does not run AnteHandler, so there is no
// authoritative priority to copy. If this contract changes (e.g. we
// start computing a deterministic priority from the decoded tx),
// update both NewInsertTxHandler and this test.
func TestInsertTxHandler_StampsPriorityZero(t *testing.T) {
	mp := &priorityCapturingMempool{}
	insert := cronosmempool.NewInsertTxHandler(mp, func(b []byte) (sdk.Tx, error) {
		return &stubFeeTx{gas: uint64(b[0]) * 1000, wire: b}, nil
	})

	for _, g := range []byte{50, 5, 200, 10, 100} {
		resp, err := insert(&abci.RequestInsertTx{Tx: []byte{g}})
		if err != nil || resp.Code != abci.CodeTypeOK {
			t.Fatalf("insert gas=%d: code=%d err=%v", g, resp.Code, err)
		}
	}

	if got, want := len(mp.priorities), 5; got != want {
		t.Fatalf("captured %d priorities, want %d", got, want)
	}
	for i, p := range mp.priorities {
		if p != 0 {
			t.Fatalf("priorities[%d] = %d, want 0", i, p)
		}
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
	handler := cronosmempool.NewReapTxsHandler(mp, encoderFixedWire, log.NewNopLogger())
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
