package mempool

import (
	"errors"
	"strconv"
	"sync"
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
// so a pointer receiver is required to exercise registration correctly. The id
// field gives it non-zero size so distinct &ptrTx{} allocations have distinct
// addresses (zero-size structs all share runtime.zerobase).
type ptrTx struct{ id int }

func (*ptrTx) GetMsgs() []sdk.Msg                    { return nil }
func (*ptrTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

// noopEncoder is a non-nil txEncoder for tests that don't assert on bytes.
var noopEncoder sdk.TxEncoder = func(sdk.Tx) ([]byte, error) { return nil, nil }

// stubRunner is a test double for txRunner.
type stubRunner struct {
	runTx func([]byte) error
	calls atomic.Int64
}

func (s *stubRunner) RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, ms storetypes.MultiStore, cache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	s.calls.Add(1)
	if s.runTx != nil {
		return sdk.GasInfo{}, nil, nil, s.runTx(txBytes)
	}
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

// insertHandler builds an InsertTxHandler with no encCache for tests that only
// exercise the admission path.
func insertHandler(runner txRunner) sdk.InsertTxHandler {
	return newAdmitter(runner, nil, nil, noopEncoder).InsertTxHandler()
}

func TestInsertTxHandler_AcceptsValidTx(t *testing.T) {
	runner := &stubRunner{}
	h := insertHandler(runner)

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
	h := insertHandler(runner)

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
	h := insertHandler(runner)

	resp, err := h(&abci.RequestInsertTx{Tx: []byte("any-tx")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != abci.CodeTypeRetry {
		t.Fatalf("expected CodeTypeRetry, got %d", resp.Code)
	}
}

func TestInsertTxHandler_ExecModeIsCheck(t *testing.T) {
	var capturedMode sdk.ExecMode
	var captureRunner captureExecModeRunner
	captureRunner.mode = &capturedMode
	h := insertHandler(&captureRunner)

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

// Ensure the error wrapping works for wrapped sentinel errors.
func TestInsertTxHandler_RetryOnWrappedMempoolFull(t *testing.T) {
	runner := &stubRunner{runTx: func(_ []byte) error {
		return errors.Join(errors.New("outer"), sdkmempool.ErrMempoolTxMaxCapacity)
	}}
	h := insertHandler(runner)

	resp, _ := h(&abci.RequestInsertTx{Tx: []byte("tx")})
	if resp.Code != abci.CodeTypeRetry {
		t.Fatalf("expected CodeTypeRetry for wrapped ErrMempoolTxMaxCapacity, got %d", resp.Code)
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

// TestNewAdmitter_PanicsOnMissingDeps verifies construction fails loudly on
// misconfiguration: a non-nil encCache without a txGet or txEncoder would skip
// canonical-bytes registration on the reap fast path.
func TestNewAdmitter_PanicsOnMissingDeps(t *testing.T) {
	enc := NewEncoderCache(0)
	txGet := func([]byte) (sdk.Tx, bool) { return &ptrTx{}, true }

	assertPanics(t, "nil txEncoder with encCache", func() {
		newAdmitter(&stubRunner{}, txGet, enc, nil)
	})
	assertPanics(t, "nil txGet with encCache", func() {
		newAdmitter(&stubRunner{}, nil, enc, noopEncoder)
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
	enc := NewEncoderCache(0)
	h := newAdmitter(runner, txGet, enc, txEncoder).InsertTxHandler()

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
	enc := NewEncoderCache(0)
	h := newAdmitter(runner, txGet, enc, txEncoder).InsertTxHandler()

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
	enc := NewEncoderCache(0)
	h := newAdmitter(runner, txGet, enc, txEncoder).InsertTxHandler()

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

// raceRunner models the real txRunner: RunTx mutates shared, non-thread-safe
// state (here a plain Go map, standing in for baseapp's checkState multistore).
// It takes NO internal lock, so the Admitter MUST serialize admission for these
// writes to be safe. Run under -race to expose a missing mutex.
type raceRunner struct {
	state map[string]struct{} // intentionally lock-free
}

func (r *raceRunner) RunTx(_ sdk.ExecMode, txBytes []byte, _ sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	// Unsynchronized read+write, mirroring cacheTxContext + msCache.Write into
	// the shared checkState. Safe only because the Admitter holds its mutex.
	r.state[string(txBytes)] = struct{}{}
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

// TestInsertTxHandler_ConcurrentAdmissionIsSerialized hammers the handler from
// many goroutines, mirroring CometBFT's concurrent InsertTx delivery (per-peer
// P2P reactor + per-tx RPC BroadcastTx goroutines). RunTx writes a lock-free
// map; the Admitter's mutex is the ONLY thing making those writes safe. Under
// `go test -race` this fails if the mutex is removed.
func TestInsertTxHandler_ConcurrentAdmissionIsSerialized(t *testing.T) {
	runner := &raceRunner{state: make(map[string]struct{})}
	h := insertHandler(runner)

	const goroutines = 16
	const perG = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range perG {
				tx := []byte(strconv.Itoa(g) + ":" + strconv.Itoa(i))
				if _, err := h(&abci.RequestInsertTx{Tx: tx}); err != nil {
					t.Errorf("g%d i%d: unexpected error: %v", g, i, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if got := len(runner.state); got != goroutines*perG {
		t.Fatalf("expected %d distinct txs admitted, got %d", goroutines*perG, got)
	}
}

// TestCheckTxHandler_MapsSuccess verifies a passing tx maps gas/log/data into
// the ResponseCheckTx with CodeTypeOK.
func TestCheckTxHandler_MapsSuccess(t *testing.T) {
	a := newAdmitter(&stubRunner{}, nil, nil, noopEncoder)
	check := a.CheckTxHandler()

	runTx := func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return sdk.GasInfo{GasWanted: 100, GasUsed: 42}, &sdk.Result{Log: "ok", Data: []byte("d")}, nil, nil
	}
	resp, err := check(runTx, &abci.RequestCheckTx{Tx: []byte("tx")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != abci.CodeTypeOK {
		t.Fatalf("expected CodeTypeOK, got %d", resp.Code)
	}
	if resp.GasWanted != 100 || resp.GasUsed != 42 {
		t.Fatalf("gas mismatch: wanted=%d used=%d", resp.GasWanted, resp.GasUsed)
	}
	if resp.Log != "ok" || string(resp.Data) != "d" {
		t.Fatalf("log/data mismatch: log=%q data=%q", resp.Log, resp.Data)
	}
}

// TestCheckTxHandler_MapsError verifies an AnteHandler rejection maps to a
// non-OK code in the response (not a transport error).
func TestCheckTxHandler_MapsError(t *testing.T) {
	a := newAdmitter(&stubRunner{}, nil, nil, noopEncoder)
	check := a.CheckTxHandler()

	anteErr := errorsmod.Register("test-check", 1, "bad sig")
	runTx := func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return sdk.GasInfo{}, nil, nil, anteErr
	}
	resp, err := check(runTx, &abci.RequestCheckTx{Tx: []byte("bad")})
	if err != nil {
		t.Fatalf("handler must not surface a transport error, got %v", err)
	}
	if resp.Code == abci.CodeTypeOK {
		t.Fatal("expected non-OK code for rejected tx")
	}
}

// TestCheckTxHandler_RegistersCanonicalBytes verifies Q1 fix: an RPC-submitted
// tx (which never traverses InsertTx) still has its canonical bytes registered
// in the EncoderCache, so it hits the reap fast path.
func TestCheckTxHandler_RegistersCanonicalBytes(t *testing.T) {
	tx := &ptrTx{}
	raw := []byte("rpc-gossip-bytes")
	canonical := []byte("canonical")

	txGet := func(bz []byte) (sdk.Tx, bool) {
		if string(bz) != string(raw) {
			t.Fatalf("txGet got %q, want %q", bz, raw)
		}
		return tx, true
	}
	txEncoder := func(sdk.Tx) ([]byte, error) { return canonical, nil }
	enc := NewEncoderCache(0)
	a := newAdmitter(&stubRunner{}, txGet, enc, txEncoder)
	check := a.CheckTxHandler()

	runTx := func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return sdk.GasInfo{}, &sdk.Result{}, nil, nil
	}
	if _, err := check(runTx, &abci.RequestCheckTx{Tx: raw}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := enc.Bytes(tx)
	if !ok {
		t.Fatal("RPC-admitted tx was not registered in encCache")
	}
	if string(got) != string(canonical) {
		t.Fatalf("registered %q, want canonical %q", got, canonical)
	}
}

// TestCheckTxHandler_NoRegisterOnReject verifies a rejected RPC tx is never
// looked up or registered.
func TestCheckTxHandler_NoRegisterOnReject(t *testing.T) {
	tx := &ptrTx{}
	var txGetCalled bool
	txGet := func([]byte) (sdk.Tx, bool) { txGetCalled = true; return tx, true }
	enc := NewEncoderCache(0)
	a := newAdmitter(&stubRunner{}, txGet, enc, noopEncoder)
	check := a.CheckTxHandler()

	anteErr := errorsmod.Register("test-check-rej", 1, "bad")
	runTx := func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return sdk.GasInfo{}, nil, nil, anteErr
	}
	if _, err := check(runTx, &abci.RequestCheckTx{Tx: []byte("bad")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txGetCalled {
		t.Fatal("txGet must not run for a rejected tx")
	}
	if _, ok := enc.Bytes(tx); ok {
		t.Fatal("rejected tx must not be registered")
	}
}

// TestAdmitter_InsertAndCheckShareMutex is the regression test for the race the
// reviewer flagged: RPC CheckTx runs LOCK-FREE while p2p InsertTx is serialized.
// Both write the same lock-free state (standing in for checkState). They MUST
// share one mutex. Half the goroutines drive InsertTx, half drive CheckTx
// against the SAME Admitter and the SAME backing map. Under `go test -race`
// this fails if CheckTxHandler omits a.mu or uses a different lock.
func TestAdmitter_InsertAndCheckShareMutex(t *testing.T) {
	runner := &raceRunner{state: make(map[string]struct{})}
	a := newAdmitter(runner, nil, nil, noopEncoder)
	insert := a.InsertTxHandler()
	check := a.CheckTxHandler()

	// CheckTx's runTx closure mirrors BaseApp: it drives the SAME lock-free
	// runner/state that InsertTx writes through a.runner.
	runTx := func(txBytes []byte, _ sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return runner.RunTx(sdk.ExecModeCheck, txBytes, nil, -1, nil, nil)
	}

	const goroutines = 16
	const perG = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range perG {
				tx := []byte(strconv.Itoa(g) + ":" + strconv.Itoa(i))
				var err error
				if g%2 == 0 {
					_, err = insert(&abci.RequestInsertTx{Tx: tx})
				} else {
					_, err = check(runTx, &abci.RequestCheckTx{Tx: tx})
				}
				if err != nil {
					t.Errorf("g%d i%d: unexpected error: %v", g, i, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if got := len(runner.state); got != goroutines*perG {
		t.Fatalf("expected %d distinct txs, got %d", goroutines*perG, got)
	}
}

// BenchmarkInsertTxHandler_Admit measures the admission path: SHA256-free now,
// one RunTx per call (stubbed to ~0ns to isolate handler + mutex overhead).
func BenchmarkInsertTxHandler_Admit(b *testing.B) {
	runner := &stubRunner{}
	h := insertHandler(runner)

	b.ResetTimer()
	b.ReportAllocs()
	for i := range b.N {
		tx := make([]byte, 32)
		tx[0] = byte(i)
		tx[1] = byte(i >> 8)
		h(&abci.RequestInsertTx{Tx: tx}) //nolint:errcheck
	}
}
