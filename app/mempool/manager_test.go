package mempool

import (
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

func insertHandler(runner txRunner) sdk.InsertTxHandler {
	return newMempoolManager(runner, nil, noopEncoder, nil).InsertTxHandler()
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

func TestNewMempoolManager_PanicsOnMissingDeps(t *testing.T) {
	enc := NewEncoderCache(0, 0)
	noopDecoder := func([]byte) (sdk.Tx, error) { return &ptrTx{}, nil }

	assertPanics(t, "nil txEncoder with encCache", func() {
		newMempoolManager(&stubRunner{}, enc, nil, noopDecoder)
	})
	assertPanics(t, "nil decoder with encCache", func() {
		newMempoolManager(&stubRunner{}, enc, noopEncoder, nil)
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
	h := newMempoolManager(runner, enc, txEncoder, decoder).InsertTxHandler()

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
	h := newMempoolManager(runner, enc, txEncoder, decoder).InsertTxHandler()

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
	h := newMempoolManager(runner, enc, txEncoder, decoder).InsertTxHandler()

	if _, err := h(&abci.RequestInsertTx{Tx: []byte("bad")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := enc.Get(tx); ok {
		t.Fatal("rejected tx must not be registered in encCache")
	}
}

// raceRunner models the real txRunner: RunTx mutates shared, non-thread-safe
// state (a plain Go map, standing in for baseapp's checkState multistore) with
// NO internal lock, so the MempoolManager MUST serialize admission. Run under -race
// to expose a missing mutex.
type raceRunner struct {
	state map[string]struct{} // intentionally lock-free
}

func (r *raceRunner) RunTx(_ sdk.ExecMode, txBytes []byte, _ sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	// Unsynchronized read+write, mirroring cacheTxContext + msCache.Write into
	// the shared checkState. Safe only because the MempoolManager holds its mutex.
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
	a := newMempoolManager(&stubRunner{}, nil, noopEncoder, nil)
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

func TestCheckTxHandler_MapsError(t *testing.T) {
	a := newMempoolManager(&stubRunner{}, nil, noopEncoder, nil)
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
	a := newMempoolManager(&stubRunner{}, enc, txEncoder, decoder)
	check := a.CheckTxHandler()

	runTx := func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return sdk.GasInfo{}, &sdk.Result{}, nil, nil
	}
	if _, err := check(runTx, &abci.RequestCheckTx{Tx: raw}); err != nil {
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
	a := newMempoolManager(&stubRunner{}, enc, noopEncoder, decoder)
	check := a.CheckTxHandler()

	anteErr := errorsmod.Register("test-check-rej", 1, "bad")
	runTx := func([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
		return sdk.GasInfo{}, nil, nil, anteErr
	}
	if _, err := check(runTx, &abci.RequestCheckTx{Tx: []byte("bad")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := enc.Get(tx); ok {
		t.Fatal("rejected tx must not be registered")
	}
}

func TestMempoolManager_InsertAndCheckShareMutex(t *testing.T) {
	runner := &raceRunner{state: make(map[string]struct{})}
	a := newMempoolManager(runner, nil, noopEncoder, nil)
	insert := a.InsertTxHandler()
	check := a.CheckTxHandler()

	// CheckTx's runTx closure mirrors BaseApp: it drives the same lock-free
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

func TestMempoolManager_AdmissionMutexGatesAdmission(t *testing.T) {
	a := newMempoolManager(&stubRunner{}, nil, noopEncoder, nil)
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
