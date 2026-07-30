package mempool

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	protov2 "google.golang.org/protobuf/proto"

	errorsmod "cosmossdk.io/errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// ptrTx is a minimal pointer-typed sdk.Tx. EncoderCache keys on the sdk.Tx
// interface value, which for pointer types is pointer equality, so a pointer
// receiver is needed. The id field gives it non-zero size so distinct
// allocations get distinct addresses (zero-size structs share runtime.zerobase).
type ptrTx struct {
	id      int
	timeout uint64 // GetTimeoutHeight; 0 = no timeout
}

func (*ptrTx) GetMsgs() []sdk.Msg                    { return nil }
func (*ptrTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (t *ptrTx) GetTimeoutHeight() uint64            { return t.timeout }

// noopEncoder is a non-nil txEncoder for tests that don't assert on bytes.
var noopEncoder sdk.TxEncoder = func(sdk.Tx) ([]byte, error) { return nil, nil }

// stubRunner is a test double for txRunner. resp, if set, takes precedence
// over runTx and gives full control over the returned GasInfo/Result/events
// (needed by CheckTxHandler tests, which no longer drive a caller-supplied
// runTx closure).
type stubRunner struct {
	runTx func([]byte) error
	resp  func(mode sdk.ExecMode, txBytes []byte) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
	calls atomic.Int64
	// ms, if non-nil, records the txMultiStore arg of the most recent RunTx call.
	ms *storetypes.MultiStore
}

func (s *stubRunner) RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, ms storetypes.MultiStore, cache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	s.calls.Add(1)
	if s.ms != nil {
		*s.ms = ms
	}
	if s.resp != nil {
		return s.resp(mode, txBytes)
	}
	if s.runTx != nil {
		return sdk.GasInfo{}, nil, nil, s.runTx(txBytes)
	}
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

func insertHandler(runner txRunner) sdk.InsertTxHandler {
	return newManager(runner, nil, noopEncoder, nil).InsertTxHandler()
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

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

func TestNewManager_PanicsOnMissingDeps(t *testing.T) {
	enc := NewEncoderCache(0, 0)
	noopDecoder := func([]byte) (sdk.Tx, error) { return &ptrTx{}, nil }

	assertPanics(t, "nil txEncoder with encCache", func() {
		newManager(&stubRunner{}, enc, nil, noopDecoder)
	})
	assertPanics(t, "nil decoder with encCache", func() {
		newManager(&stubRunner{}, enc, noopEncoder, nil)
	})
}

func TestInsertTxHandler_RegistersCanonicalBytes(t *testing.T) {
	runner := &stubRunner{}
	tx := &ptrTx{}
	raw := []byte("non-canonical-gossip-bytes")
	canonical := []byte("canonical")

	decoder := func(bz []byte) (sdk.Tx, error) {
		if string(bz) != string(raw) {
			t.Fatalf("decoder got %q, want raw req.Tx %q", bz, raw)
		}
		return tx, nil
	}
	txEncoder := func(got sdk.Tx) ([]byte, error) {
		if got != sdk.Tx(tx) {
			t.Fatal("txEncoder called with a tx other than the one decoder returned")
		}
		return canonical, nil
	}
	enc := NewEncoderCache(0, 0)
	h := newManager(runner, enc, txEncoder, decoder).InsertTxHandler()

	resp, err := h(&abci.RequestInsertTx{Tx: raw})
	if err != nil || resp.Code != abci.CodeTypeOK {
		t.Fatalf("admit failed: code=%d err=%v", resp.Code, err)
	}
	got, ok := enc.Get(tx)
	if !ok {
		t.Fatal("admitted tx was not registered in encCache")
	}
	if string(got) != string(canonical) {
		t.Fatalf("registered %q, want canonical bytes %q (raw must not be stored)", got, canonical)
	}
}

func TestInsertTxHandler_RegistersRawBytesOnEncoderError(t *testing.T) {
	runner := &stubRunner{}
	tx := &ptrTx{}
	raw := []byte("raw-bytes")

	decoder := func([]byte) (sdk.Tx, error) { return tx, nil }
	txEncoder := func(sdk.Tx) ([]byte, error) { return nil, errors.New("encode fail") }
	enc := NewEncoderCache(0, 0)
	h := newManager(runner, enc, txEncoder, decoder).InsertTxHandler()

	if _, err := h(&abci.RequestInsertTx{Tx: raw}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := enc.Get(tx)
	if !ok || string(got) != string(raw) {
		t.Fatalf("expected raw fallback %q, got %q (ok=%v)", raw, got, ok)
	}
}

func TestInsertTxHandler_NoRegisterOnReject(t *testing.T) {
	anteErr := errorsmod.Register("test-encreg", 1, "bad sig")
	runner := &stubRunner{runTx: func([]byte) error { return anteErr }}
	tx := &ptrTx{}

	decoder := func([]byte) (sdk.Tx, error) { return tx, nil }
	txEncoder := func(sdk.Tx) ([]byte, error) { return []byte("x"), nil }
	enc := NewEncoderCache(0, 0)
	h := newManager(runner, enc, txEncoder, decoder).InsertTxHandler()

	if _, err := h(&abci.RequestInsertTx{Tx: []byte("bad")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := enc.Get(tx); ok {
		t.Fatal("rejected tx must not be registered in encCache")
	}
}

// raceRunner models the real txRunner: RunTx mutates shared, non-thread-safe
// state (a plain Go map, standing in for baseapp's checkState multistore) with
// NO internal lock, so the Manager MUST serialize admission. Run under -race
// to expose a missing mutex.
type raceRunner struct {
	state map[string]struct{} // intentionally lock-free
}

func (r *raceRunner) RunTx(_ sdk.ExecMode, txBytes []byte, _ sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	// Unsynchronized read+write, mirroring cacheTxContext + msCache.Write into
	// the shared checkState. Safe only because the Manager holds its mutex.
	r.state[string(txBytes)] = struct{}{}
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

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

func TestCheckTxHandler_MapsSuccess(t *testing.T) {
	runner := &stubRunner{resp: func(sdk.ExecMode, []byte) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return sdk.GasInfo{GasWanted: 100, GasUsed: 42}, &sdk.Result{Log: "ok", Data: []byte("d")}, nil, nil
	}}
	a := newManager(runner, nil, noopEncoder, nil)
	check := a.CheckTxHandler()

	resp, err := check(nil, &abci.RequestCheckTx{Tx: []byte("tx")})
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

func TestCheckTxHandler_MapsError(t *testing.T) {
	anteErr := errorsmod.Register("test-check", 1, "bad sig")
	runner := &stubRunner{runTx: func([]byte) error { return anteErr }}
	a := newManager(runner, nil, noopEncoder, nil)
	check := a.CheckTxHandler()

	resp, err := check(nil, &abci.RequestCheckTx{Tx: []byte("bad")})
	if err != nil {
		t.Fatalf("handler must not surface a transport error, got %v", err)
	}
	if resp.Code == abci.CodeTypeOK {
		t.Fatal("expected non-OK code for rejected tx")
	}
}

func TestCheckTxHandler_RecheckTypeMapsToExecModeReCheck(t *testing.T) {
	var capturedMode sdk.ExecMode
	a := newManager(&captureExecModeRunner{mode: &capturedMode}, nil, noopEncoder, nil)
	check := a.CheckTxHandler()

	if _, err := check(nil, &abci.RequestCheckTx{Tx: []byte("tx"), Type: abci.CheckTxType_Recheck}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMode != sdk.ExecModeReCheck {
		t.Fatalf("expected ExecModeReCheck, got %v", capturedMode)
	}
}

func TestCheckTxHandler_NewTypeMapsToExecModeCheck(t *testing.T) {
	var capturedMode sdk.ExecMode
	a := newManager(&captureExecModeRunner{mode: &capturedMode}, nil, noopEncoder, nil)
	check := a.CheckTxHandler()

	if _, err := check(nil, &abci.RequestCheckTx{Tx: []byte("tx"), Type: abci.CheckTxType_New}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMode != sdk.ExecModeCheck {
		t.Fatalf("expected ExecModeCheck, got %v", capturedMode)
	}
}

func TestCheckTxHandler_UnknownTypeReturnsError(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	check := a.CheckTxHandler()

	if _, err := check(nil, &abci.RequestCheckTx{Tx: []byte("tx"), Type: abci.CheckTxType(99)}); err == nil {
		t.Fatal("expected error for unknown CheckTxType")
	}
}

func TestCheckTxHandler_RegistersCanonicalBytes(t *testing.T) {
	tx := &ptrTx{}
	raw := []byte("rpc-gossip-bytes")
	canonical := []byte("canonical")

	decoder := func(bz []byte) (sdk.Tx, error) {
		if string(bz) != string(raw) {
			t.Fatalf("decoder got %q, want %q", bz, raw)
		}
		return tx, nil
	}
	txEncoder := func(sdk.Tx) ([]byte, error) { return canonical, nil }
	enc := NewEncoderCache(0, 0)
	a := newManager(&stubRunner{}, enc, txEncoder, decoder)
	check := a.CheckTxHandler()

	if _, err := check(nil, &abci.RequestCheckTx{Tx: raw}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := enc.Get(tx)
	if !ok {
		t.Fatal("RPC-admitted tx was not registered in encCache")
	}
	if string(got) != string(canonical) {
		t.Fatalf("registered %q, want canonical %q", got, canonical)
	}
}

func TestCheckTxHandler_NoRegisterOnReject(t *testing.T) {
	tx := &ptrTx{}
	decoder := func([]byte) (sdk.Tx, error) { return tx, nil }
	enc := NewEncoderCache(0, 0)
	anteErr := errorsmod.Register("test-check-rej", 1, "bad")
	runner := &stubRunner{runTx: func([]byte) error { return anteErr }}
	a := newManager(runner, enc, noopEncoder, decoder)
	check := a.CheckTxHandler()

	if _, err := check(nil, &abci.RequestCheckTx{Tx: []byte("bad")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := enc.Get(tx); ok {
		t.Fatal("rejected tx must not be registered")
	}
}

func TestManager_InsertAndCheckShareMutex(t *testing.T) {
	runner := &raceRunner{state: make(map[string]struct{})}
	a := newManager(runner, nil, noopEncoder, nil)
	insert := a.InsertTxHandler()
	check := a.CheckTxHandler()

	// CheckTxHandler drives a.runner directly (the same lock-free raceRunner
	// InsertTx writes through), so -race flags either path if it skips stateMu.
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
					_, err = check(nil, &abci.RequestCheckTx{Tx: tx})
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

func TestManager_AdmissionMutexGatesAdmission(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	insert := a.InsertTxHandler()
	mu := a.AdmissionMutex()

	mu.Lock() // simulate App.Commit holding the admission mutex
	admitted := make(chan struct{})
	go func() {
		insert(&abci.RequestInsertTx{Tx: []byte("tx")}) //nolint:errcheck
		close(admitted)
	}()

	select {
	case <-admitted:
		t.Fatal("admission ran while AdmissionMutex held; Commit would race checkState")
	case <-time.After(50 * time.Millisecond):
		// expected: admission blocked behind the mutex
	}

	mu.Unlock()
	select {
	case <-admitted:
		// admission proceeds once Commit releases the mutex
	case <-time.After(time.Second):
		t.Fatal("admission did not proceed after AdmissionMutex released")
	}
}

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

func TestManagerInsertTx_AcceptsValidTx(t *testing.T) {
	runner := &stubRunner{}
	a := newManager(runner, nil, noopEncoder, nil)

	resp, _ := a.InsertTx([]byte("good-tx"))
	if resp.Code != abci.CodeTypeOK {
		t.Fatalf("expected CodeTypeOK, got %d (codespace=%q log=%q)", resp.Code, resp.Codespace, resp.RawLog)
	}
	if resp.Codespace != "" || resp.RawLog != "" {
		t.Fatalf("expected empty codespace/log on success, got %q/%q", resp.Codespace, resp.RawLog)
	}
	if runner.calls.Load() != 1 {
		t.Fatalf("expected 1 RunTx call, got %d", runner.calls.Load())
	}
}

func TestManagerInsertTx_RejectsInvalidTx(t *testing.T) {
	anteErr := errorsmod.Register("test-inserttx-rej", 1, "bad sig")
	runner := &stubRunner{runTx: func([]byte) error { return anteErr }}
	a := newManager(runner, nil, noopEncoder, nil)

	resp, _ := a.InsertTx([]byte("bad-tx"))
	if resp.Code == abci.CodeTypeOK {
		t.Fatal("expected non-OK code for rejected tx")
	}
	if resp.Codespace == "" || resp.RawLog == "" {
		t.Fatalf("expected codespace+log on reject, got %q/%q", resp.Codespace, resp.RawLog)
	}
}

func TestManagerInsertTx_RetryOnMempoolFull(t *testing.T) {
	runner := &stubRunner{runTx: func([]byte) error { return sdkmempool.ErrMempoolTxMaxCapacity }}
	a := newManager(runner, nil, noopEncoder, nil)

	resp, _ := a.InsertTx([]byte("any-tx"))
	if resp.Code != abci.CodeTypeRetry {
		t.Fatalf("expected CodeTypeRetry, got %d", resp.Code)
	}
	if resp.RawLog != "mempool is full" {
		t.Fatalf("expected back-pressure log, got %q", resp.RawLog)
	}
}

func TestManagerInsertTx_RetryOnWrappedMempoolFull(t *testing.T) {
	runner := &stubRunner{runTx: func([]byte) error {
		return errors.Join(errors.New("outer"), sdkmempool.ErrMempoolTxMaxCapacity)
	}}
	a := newManager(runner, nil, noopEncoder, nil)

	if resp, _ := a.InsertTx([]byte("tx")); resp.Code != abci.CodeTypeRetry {
		t.Fatalf("expected CodeTypeRetry for wrapped ErrMempoolTxMaxCapacity, got %d", resp.Code)
	}
}

func TestManagerInsertTx_RegistersCanonicalBytes(t *testing.T) {
	runner := &stubRunner{}
	tx := &ptrTx{}
	raw := []byte("non-canonical-rpc-bytes")
	canonical := []byte("canonical")

	decoder := func([]byte) (sdk.Tx, error) { return tx, nil }
	txEncoder := func(sdk.Tx) ([]byte, error) { return canonical, nil }
	enc := NewEncoderCache(0, 0)
	a := newManager(runner, enc, txEncoder, decoder)

	if resp, _ := a.InsertTx(raw); resp.Code != abci.CodeTypeOK {
		t.Fatalf("admit failed: code=%d", resp.Code)
	}
	got, ok := enc.Get(tx)
	if !ok || string(got) != string(canonical) {
		t.Fatalf("expected canonical %q registered, got %q (ok=%v)", canonical, got, ok)
	}
}

func TestManagerInsertTx_NoRegisterOnReject(t *testing.T) {
	anteErr := errorsmod.Register("test-inserttx-encreg", 1, "bad sig")
	runner := &stubRunner{runTx: func([]byte) error { return anteErr }}
	tx := &ptrTx{}
	decoder := func([]byte) (sdk.Tx, error) { return tx, nil }
	txEncoder := func(sdk.Tx) ([]byte, error) { return []byte("x"), nil }
	enc := NewEncoderCache(0, 0)
	a := newManager(runner, enc, txEncoder, decoder)

	if resp, _ := a.InsertTx([]byte("bad")); resp.Code == abci.CodeTypeOK {
		t.Fatal("expected non-OK code")
	}
	if _, ok := enc.Get(tx); ok {
		t.Fatal("rejected tx must not be registered in encCache")
	}
}

// TestManagerInsertTx_SharesAdmitWithHandler proves the RPC InsertTx and the
// gossip InsertTxHandler run the same admission body under one mutex: both drive
// the lock-free raceRunner concurrently, which -race flags if a path skips a.mu.
func TestManagerInsertTx_SharesAdmitWithHandler(t *testing.T) {
	runner := &raceRunner{state: make(map[string]struct{})}
	a := newManager(runner, nil, noopEncoder, nil)
	insert := a.InsertTxHandler()

	const goroutines = 16
	const perG = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range perG {
				tx := []byte(strconv.Itoa(g) + ":" + strconv.Itoa(i))
				if g%2 == 0 {
					if _, err := insert(&abci.RequestInsertTx{Tx: tx}); err != nil {
						t.Errorf("g%d i%d: gossip insert error: %v", g, i, err)
						return
					}
				} else if resp, _ := a.InsertTx(tx); resp.Code != abci.CodeTypeOK {
					t.Errorf("g%d i%d: rpc insert code=%d", g, i, resp.Code)
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

// fakePool is a minimal ExtMempool: PoolSnapshot iterates it via SelectBy.
type fakePool struct{ txs []sdk.Tx }

func (p *fakePool) Insert(context.Context, sdk.Tx) error                 { return nil }
func (p *fakePool) Select(context.Context, [][]byte) sdkmempool.Iterator { return nil }
func (p *fakePool) CountTx() int                                         { return len(p.txs) }
func (p *fakePool) Remove(sdk.Tx) error                                  { return nil }
func (p *fakePool) RemoveWithReason(context.Context, sdk.Tx, sdkmempool.RemoveReason) error {
	return nil
}

func (p *fakePool) SelectBy(_ context.Context, _ [][]byte, cb func(sdk.Tx) bool) {
	for _, tx := range p.txs {
		if !cb(tx) {
			return
		}
	}
}

func TestManagerPendingTxs(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	if got := a.PendingTxs(); got != nil {
		t.Fatalf("nil mpool must report no pending txs, got %d", len(got))
	}

	tx1, tx2 := &ptrTx{}, &ptrTx{}
	a.mpool = &fakePool{txs: []sdk.Tx{tx1, tx2}}

	got := a.PendingTxs()
	if len(got) != 2 || got[0] != tx1 || got[1] != tx2 {
		t.Fatalf("want both pool txs, got %d", len(got))
	}
}

func TestManagerCountTx(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	if got := a.CountTx(); got != 0 {
		t.Fatalf("nil mpool must report 0, got %d", got)
	}

	a.mpool = &fakePool{txs: []sdk.Tx{&ptrTx{}, &ptrTx{}, &ptrTx{}}}
	if got := a.CountTx(); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
}

// msCaptureRunner is a txRunner double that records each call's txMultiStore
// arg, so a test can assert the three RunTx call sites (admit, CheckTxHandler,
// runRecheck) all receive the same mempoolState.base instance.
type msCaptureRunner struct {
	mu sync.Mutex
	ms []storetypes.MultiStore
}

func (r *msCaptureRunner) RunTx(_ sdk.ExecMode, _ []byte, _ sdk.Tx, _ int, ms storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	r.mu.Lock()
	r.ms = append(r.ms, ms)
	r.mu.Unlock()
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

func TestManager_AllThreeRunTxSitesShareBaseInstance(t *testing.T) {
	runner := &msCaptureRunner{}
	a := newManager(runner, nil, noopEncoder, nil)
	base := newFakeCacheStore()
	a.state = &mempoolState{base: base}
	a.mpool = &fakePool{}
	a.signer = fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{}}

	a.admit([]byte("tx1"))
	check := a.CheckTxHandler()
	check(nil, &abci.RequestCheckTx{Tx: []byte("tx2")}) //nolint:errcheck
	a.runRecheck([]sdk.Tx{&ptrTx{id: 1}}, a.gen.Load())

	if len(runner.ms) != 3 {
		t.Fatalf("expected 3 RunTx calls (admit, CheckTxHandler, runRecheck), got %d", len(runner.ms))
	}
	for i, ms := range runner.ms {
		if ms == nil {
			t.Fatalf("call %d: got a nil store, want the wired base", i)
		}
		if ms != storetypes.MultiStore(base) {
			t.Fatalf("call %d: got a different store instance than the wired base", i)
		}
	}
}

// fakeNonceStore stands in for the real branched CacheMultiStore's role as
// nonce authority: setNonce mirrors baseapp's ante write-back into the
// txMultiStore arg, getNonce mirrors a later RunTx reading it back out.
type fakeNonceStore struct {
	cacheMultiStoreIface // defined type, not the interface itself: see state_test.go
	mu                   sync.Mutex
	nonces               map[string]uint64
}

func newFakeNonceStore() *fakeNonceStore { return &fakeNonceStore{nonces: map[string]uint64{}} }

func (f *fakeNonceStore) setNonce(sender string, n uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nonces[sender] = n
}

func (f *fakeNonceStore) getNonce(sender string) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nonces[sender]
}

// nonceBranchRunner models baseapp's RunTx(txMultiStore) contract from the
// design doc: ReCheck writes a nonce bump back into the passed store; Check
// only succeeds once that write is visible through the same store.
type nonceBranchRunner struct{}

func (r *nonceBranchRunner) RunTx(mode sdk.ExecMode, _ []byte, _ sdk.Tx, _ int, ms storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	store, ok := ms.(*fakeNonceStore)
	if !ok {
		return sdk.GasInfo{}, nil, nil, errors.New("no branched store")
	}
	switch mode {
	case sdk.ExecModeReCheck:
		store.setNonce("alice", 8)
		return sdk.GasInfo{}, &sdk.Result{}, nil, nil
	case sdk.ExecModeCheck:
		if store.getNonce("alice") < 8 {
			return sdk.GasInfo{}, nil, nil, errors.New("nonce not yet visible")
		}
		return sdk.GasInfo{}, &sdk.Result{}, nil, nil
	default:
		return sdk.GasInfo{}, &sdk.Result{}, nil, nil
	}
}

// TestManager_RecheckWriteVisibleToLaterAdmit proves nonce continuity across
// the branch: a RunTx(ExecModeReCheck) write into base must be visible to a
// later admit() reading through the same shared base.
func TestManager_RecheckWriteVisibleToLaterAdmit(t *testing.T) {
	store := newFakeNonceStore()
	a := newManager(&nonceBranchRunner{}, nil, noopEncoder, nil)
	a.state = &mempoolState{base: store}

	a.runRecheck([]sdk.Tx{&ptrTx{id: 1}}, a.gen.Load())

	code, _, log := a.admit([]byte("alice-nonce-8-sibling"))
	if code != abci.CodeTypeOK {
		t.Fatalf("admission must see recheck's nonce write-back through the shared base, got code=%d log=%q", code, log)
	}
}

func TestManager_RefreshMempoolStateLockedSwapsBaseAndBumpsGen(t *testing.T) {
	first, second := newFakeCacheStore(), newFakeCacheStore()
	calls := 0
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.state = &mempoolState{provider: func() storetypes.CommitMultiStore {
		calls++
		if calls == 1 {
			return &fakeCommitStore{cache: first}
		}
		return &fakeCommitStore{cache: second}
	}}
	a.state.refreshLocked() // mirrors NewManager's initial refresh
	if got := a.state.store(); got != storetypes.MultiStore(first) {
		t.Fatalf("expected initial base, got %v", got)
	}

	beforeGen := a.gen.Load()
	a.RefreshMempoolStateLocked()

	if got := a.state.store(); got != storetypes.MultiStore(second) {
		t.Fatal("RefreshMempoolStateLocked must swap base identity")
	}
	if got := a.gen.Load(); got != beforeGen+1 {
		t.Fatalf("RefreshMempoolStateLocked must bump gen, got %d want %d", got, beforeGen+1)
	}
}

func TestManager_RefreshMempoolStateLockedNoopWithoutState(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, nil) // state nil (test ctor)
	a.RefreshMempoolStateLocked()                         // must not panic
	if a.gen.Load() != 0 {
		t.Fatal("nil state must leave gen untouched")
	}
}

// TestManager_NilBaseBeforeFirstRefresh mirrors the production wiring order:
// NewManager sets state.provider but must NOT refresh (the store isn't loaded
// yet at that point in baseAppOptions), so admit falls back to checkState
// (nil txMultiStore) until App calls RefreshMempoolStateLocked after
// LoadLatestVersion.
func TestManager_NilBaseBeforeFirstRefresh(t *testing.T) {
	runner := &msCaptureRunner{}
	a := newManager(runner, nil, noopEncoder, nil)
	base := newFakeCacheStore()
	calls := 0
	a.state = &mempoolState{provider: func() storetypes.CommitMultiStore {
		calls++
		return &fakeCommitStore{cache: base}
	}} // provider wired, no refreshLocked call yet: mirrors NewManager exactly

	a.admit([]byte("pre-refresh"))
	if len(runner.ms) != 1 || runner.ms[0] != nil {
		t.Fatalf("admit before the first refresh must pass a nil store (checkState fallback), got %v", runner.ms)
	}
	if calls != 0 {
		t.Fatal("provider must not be invoked before RefreshMempoolStateLocked")
	}

	a.RefreshMempoolStateLocked() // mirrors App's post-LoadLatestVersion call

	a.admit([]byte("post-refresh"))
	if len(runner.ms) != 2 || runner.ms[1] != storetypes.MultiStore(base) {
		t.Fatalf("admit after the first refresh must pass the wired base, got %v", runner.ms)
	}
}
