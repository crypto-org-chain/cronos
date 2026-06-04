package mempool

import (
	"errors"
	"sync/atomic"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	protov2 "google.golang.org/protobuf/proto"

	errorsmod "cosmossdk.io/errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// ptrTx is a minimal pointer-typed sdk.Tx. EncoderCache keys on the sdk.Tx
// interface value; for pointer types interface equality is pointer equality,
// so a pointer receiver is required to exercise registration correctly.
type ptrTx struct{}

func (*ptrTx) GetMsgs() []sdk.Msg                    { return nil }
func (*ptrTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

// stubRunner is a test double for txRunner.
type stubRunner struct {
	runTx  func([]byte) error
	calls  atomic.Int64
	height atomic.Int64
}

func (s *stubRunner) RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, ms storetypes.MultiStore, cache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	s.calls.Add(1)
	if s.runTx != nil {
		return sdk.GasInfo{}, nil, nil, s.runTx(txBytes)
	}
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

func (s *stubRunner) LastBlockHeight() int64 {
	return s.height.Load()
}

func TestInsertTxHandler_AcceptsValidTx(t *testing.T) {
	runner := &stubRunner{}
	h := newInsertTxHandler(runner, 0, nil, nil, nil)

	resp, err := h(&abci.RequestInsertTx{Tx: []byte("good-tx")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != abci.CodeTypeOK {
		t.Fatalf("expected CodeTypeOK, got %d", resp.Code)
	}
	if runner.calls.Load() != 1 {
		t.Fatalf("expected 1 RunTx call, got %d", runner.calls.Load())
	}
}

func TestInsertTxHandler_RejectsInvalidTx(t *testing.T) {
	anteErr := errorsmod.Register("test", 1, "bad sig")
	runner := &stubRunner{runTx: func(_ []byte) error { return anteErr }}
	h := newInsertTxHandler(runner, 0, nil, nil, nil)

	resp, err := h(&abci.RequestInsertTx{Tx: []byte("bad-tx")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code == abci.CodeTypeOK {
		t.Fatal("expected non-OK code for rejected tx")
	}
}

func TestInsertTxHandler_RetryOnMempoolFull(t *testing.T) {
	runner := &stubRunner{runTx: func(_ []byte) error {
		return sdkmempool.ErrMempoolTxMaxCapacity
	}}
	h := newInsertTxHandler(runner, 0, nil, nil, nil)

	resp, err := h(&abci.RequestInsertTx{Tx: []byte("any-tx")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != abci.CodeTypeRetry {
		t.Fatalf("expected CodeTypeRetry, got %d", resp.Code)
	}
}

func TestInsertTxHandler_SeenCacheDeduplicates(t *testing.T) {
	runner := &stubRunner{}
	h := newInsertTxHandler(runner, 16, nil, nil, nil)

	tx := []byte("dup-tx")
	for i := range 3 {
		resp, err := h(&abci.RequestInsertTx{Tx: tx})
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if resp.Code != abci.CodeTypeOK {
			t.Fatalf("call %d: expected OK, got %d", i, resp.Code)
		}
	}
	// RunTx must be called exactly once; subsequent calls hit cache.
	if runner.calls.Load() != 1 {
		t.Fatalf("expected 1 RunTx call, got %d (cache not deduplicating)", runner.calls.Load())
	}
}

func TestInsertTxHandler_SeenCacheDisabledWhenZero(t *testing.T) {
	runner := &stubRunner{}
	h := newInsertTxHandler(runner, 0, nil, nil, nil)

	tx := []byte("dup-tx")
	for range 3 {
		h(&abci.RequestInsertTx{Tx: tx}) //nolint:errcheck
	}
	if runner.calls.Load() != 3 {
		t.Fatalf("expected 3 RunTx calls with cache disabled, got %d", runner.calls.Load())
	}
}

func TestInsertTxHandler_ExecModeIsCheck(t *testing.T) {
	var capturedMode sdk.ExecMode
	var captureRunner captureExecModeRunner
	captureRunner.mode = &capturedMode
	h := newInsertTxHandler(&captureRunner, 0, nil, nil, nil)

	h(&abci.RequestInsertTx{Tx: []byte("tx")}) //nolint:errcheck

	if capturedMode != sdk.ExecModeCheck {
		t.Fatalf("expected ExecModeCheck, got %v", capturedMode)
	}
}

type captureExecModeRunner struct {
	mode *sdk.ExecMode
}

func (r *captureExecModeRunner) RunTx(mode sdk.ExecMode, _ []byte, _ sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	*r.mode = mode
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

func (r *captureExecModeRunner) LastBlockHeight() int64 { return 0 }

func TestInsertTxHandler_SeenCacheRingWrap(t *testing.T) {
	const size = 4
	runner := &stubRunner{}
	h := newInsertTxHandler(runner, size, nil, nil, nil)

	// Fill the ring with 4 distinct txs.
	for i := range size {
		tx := []byte{byte(i)}
		h(&abci.RequestInsertTx{Tx: tx}) //nolint:errcheck
	}
	if runner.calls.Load() != int64(size) {
		t.Fatalf("expected %d RunTx calls, got %d", size, runner.calls.Load())
	}

	// Insert a 5th tx. This wraps pos back to 0, evicting tx[0].
	h(&abci.RequestInsertTx{Tx: []byte{byte(size)}}) //nolint:errcheck

	// tx[0] was evicted — must trigger a new RunTx (not a cache hit).
	runner.calls.Store(0)
	h(&abci.RequestInsertTx{Tx: []byte{0}}) //nolint:errcheck
	if runner.calls.Load() != 1 {
		t.Fatalf("evicted tx should re-run AnteHandler; got %d RunTx calls", runner.calls.Load())
	}
}

// Ensure the error wrapping works for wrapped sentinel errors.
func TestInsertTxHandler_RetryOnWrappedMempoolFull(t *testing.T) {
	runner := &stubRunner{runTx: func(_ []byte) error {
		return errors.Join(errors.New("outer"), sdkmempool.ErrMempoolTxMaxCapacity)
	}}
	h := newInsertTxHandler(runner, 0, nil, nil, nil)

	resp, _ := h(&abci.RequestInsertTx{Tx: []byte("tx")})
	if resp.Code != abci.CodeTypeRetry {
		t.Fatalf("expected CodeTypeRetry for wrapped ErrMempoolTxMaxCapacity, got %d", resp.Code)
	}
}

// TestInsertTxHandler_SeenCacheClearsOnHeightAdvance verifies that a tx
// admitted at height N is re-validated through the AnteHandler the first
// time the handler is called at height > N. This is the guard against stale
// cache hits when the underlying account state changes across a block
// commit (nonce consumed, balance drained, signing key rotated).
func TestInsertTxHandler_SeenCacheClearsOnHeightAdvance(t *testing.T) {
	runner := &stubRunner{}
	runner.height.Store(10)
	h := newInsertTxHandler(runner, 16, nil, nil, nil)

	tx := []byte("repeat-across-blocks")

	// First admission at height 10: triggers RunTx, caches the hash.
	if _, err := h(&abci.RequestInsertTx{Tx: tx}); err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if got := runner.calls.Load(); got != 1 {
		t.Fatalf("expected 1 RunTx call after first admission, got %d", got)
	}

	// Same height: cache hit, no RunTx.
	if _, err := h(&abci.RequestInsertTx{Tx: tx}); err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if got := runner.calls.Load(); got != 1 {
		t.Fatalf("same-height re-delivery should hit cache; got %d RunTx calls", got)
	}

	// Block commits; height advances. Next handler call must clear the
	// cache and re-run the AnteHandler so a tx that became invalid in the
	// committed block (nonce consumed / balance drained / key rotated)
	// cannot be admitted as a stale cache hit.
	runner.height.Store(11)
	if _, err := h(&abci.RequestInsertTx{Tx: tx}); err != nil {
		t.Fatalf("post-commit call: unexpected error: %v", err)
	}
	if got := runner.calls.Load(); got != 2 {
		t.Fatalf("height advance must force AnteHandler re-run; got %d RunTx calls", got)
	}
}

// assertPanics fails the test unless fn panics.
func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

// TestInsertTxHandler_EncCachePanicsWithoutDeps verifies construction panics
// when encCache is wired without its txGet/txEncoder dependencies. A nil
// txEncoder would silently store non-canonical proposal bytes, so it must fail
// loudly at startup rather than degrade the canonical-bytes guarantee.
func TestInsertTxHandler_EncCachePanicsWithoutDeps(t *testing.T) {
	enc := new(EncoderCache)
	txGet := func([]byte) (sdk.Tx, bool) { return &ptrTx{}, true }
	txEncoder := func(sdk.Tx) ([]byte, error) { return nil, nil }

	assertPanics(t, "nil txGet", func() {
		newInsertTxHandler(&stubRunner{}, 0, nil, enc, txEncoder)
	})
	assertPanics(t, "nil txEncoder", func() {
		newInsertTxHandler(&stubRunner{}, 0, txGet, enc, nil)
	})
}

// TestInsertTxHandler_RegistersCanonicalBytes verifies that an admitted tx has
// its CANONICAL re-encoded bytes registered in the EncoderCache — not the raw
// gossip bytes. This is the invariant that stops a peer's non-minimal proto
// encoding from landing verbatim in a proposal.
func TestInsertTxHandler_RegistersCanonicalBytes(t *testing.T) {
	runner := &stubRunner{}
	tx := &ptrTx{}
	raw := []byte("non-canonical-gossip-bytes")
	canonical := []byte("canonical")

	txGet := func(bz []byte) (sdk.Tx, bool) {
		if string(bz) != string(raw) {
			t.Fatalf("txGet got %q, want raw req.Tx %q", bz, raw)
		}
		return tx, true
	}
	txEncoder := func(got sdk.Tx) ([]byte, error) {
		if got != sdk.Tx(tx) {
			t.Fatal("txEncoder called with a tx other than the one txGet returned")
		}
		return canonical, nil
	}
	enc := new(EncoderCache)
	h := newInsertTxHandler(runner, 0, txGet, enc, txEncoder)

	resp, err := h(&abci.RequestInsertTx{Tx: raw})
	if err != nil || resp.Code != abci.CodeTypeOK {
		t.Fatalf("admit failed: code=%d err=%v", resp.Code, err)
	}
	got, ok := enc.Bytes(tx)
	if !ok {
		t.Fatal("admitted tx was not registered in encCache")
	}
	if string(got) != string(canonical) {
		t.Fatalf("registered %q, want canonical bytes %q (raw must not be stored)", got, canonical)
	}
}

// TestInsertTxHandler_RegistersRawBytesOnEncoderError verifies the fallback:
// when re-encoding errors, the raw req.Tx bytes are registered so reap can
// still ship the tx (correctness wins over the canonical-bytes optimization).
func TestInsertTxHandler_RegistersRawBytesOnEncoderError(t *testing.T) {
	runner := &stubRunner{}
	tx := &ptrTx{}
	raw := []byte("raw-bytes")

	txGet := func([]byte) (sdk.Tx, bool) { return tx, true }
	txEncoder := func(sdk.Tx) ([]byte, error) { return nil, errors.New("encode fail") }
	enc := new(EncoderCache)
	h := newInsertTxHandler(runner, 0, txGet, enc, txEncoder)

	if _, err := h(&abci.RequestInsertTx{Tx: raw}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := enc.Bytes(tx)
	if !ok || string(got) != string(raw) {
		t.Fatalf("expected raw fallback %q, got %q (ok=%v)", raw, got, ok)
	}
}

// TestInsertTxHandler_NoRegisterOnReject verifies a tx that fails the
// AnteHandler is never looked up or registered in the encCache.
func TestInsertTxHandler_NoRegisterOnReject(t *testing.T) {
	anteErr := errorsmod.Register("test-encreg", 1, "bad sig")
	runner := &stubRunner{runTx: func([]byte) error { return anteErr }}
	tx := &ptrTx{}

	var txGetCalled bool
	txGet := func([]byte) (sdk.Tx, bool) { txGetCalled = true; return tx, true }
	txEncoder := func(sdk.Tx) ([]byte, error) { return []byte("x"), nil }
	enc := new(EncoderCache)
	h := newInsertTxHandler(runner, 0, txGet, enc, txEncoder)

	if _, err := h(&abci.RequestInsertTx{Tx: []byte("bad")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txGetCalled {
		t.Fatal("txGet must not run for a rejected tx")
	}
	if _, ok := enc.Bytes(tx); ok {
		t.Fatal("rejected tx must not be registered in encCache")
	}
}

// BenchmarkInsertTxHandler_CacheHit measures the hot path: tx seen before,
// returns immediately after SHA256 + mutex check.
func BenchmarkInsertTxHandler_CacheHit(b *testing.B) {
	runner := &stubRunner{}
	h := newInsertTxHandler(runner, DefaultInsertTxCacheSize, nil, nil, nil)
	tx := []byte("repeated-tx-bytes-for-benchmark")
	// prime cache
	h(&abci.RequestInsertTx{Tx: tx}) //nolint:errcheck
	req := &abci.RequestInsertTx{Tx: tx}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		h(req) //nolint:errcheck
	}
}

// BenchmarkInsertTxHandler_CacheMiss measures the cold path: distinct tx each
// iteration, triggers RunTx every call (stubbed to ~0ns to isolate handler overhead).
func BenchmarkInsertTxHandler_CacheMiss(b *testing.B) {
	runner := &stubRunner{}
	h := newInsertTxHandler(runner, DefaultInsertTxCacheSize, nil, nil, nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := range b.N {
		tx := make([]byte, 32)
		tx[0] = byte(i)
		tx[1] = byte(i >> 8)
		h(&abci.RequestInsertTx{Tx: tx}) //nolint:errcheck
	}
}

// BenchmarkInsertSeenCache_Has measures raw cache lookup under mutex.
func BenchmarkInsertSeenCache_Has(b *testing.B) {
	c := newInsertSeenCache(DefaultInsertTxCacheSize)
	var h [32]byte
	h[0] = 1
	c.Add(h)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		c.HasAtHeight(h, 0)
	}
}
