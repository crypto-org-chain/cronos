package mempool

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/suite"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type RecoverTestSuite struct {
	suite.Suite
}

func TestRecoverTestSuite(t *testing.T) {
	suite.Run(t, new(RecoverTestSuite))
}

const encodeSite = "encode"

func panicking(where string) { panic("boom in " + where) }

func okRunTx([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

func panicRunTx([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	panicking("runTx")
	return sdk.GasInfo{}, nil, nil, nil
}

// withDecodeCache turns on the decode path so decoder and encoder panics are reachable.
func withDecodeCache(m *Manager, decoder sdk.TxDecoder, encoder sdk.TxEncoder) {
	m.encCache = NewEncoderCache(0, 0)
	m.decoder = decoder
	m.txEncoder = encoder
}

type storingPool struct{ fakePool }

func (p *storingPool) Insert(_ context.Context, tx sdk.Tx) error {
	p.txs = append(p.txs, tx)
	return nil
}

// insertingRunner pools the tx like BaseApp.RunTx(ExecModeCheck).
type insertingRunner struct {
	pool sdkmempool.Mempool
	ran  atomic.Bool
}

func (r *insertingRunner) RunTx(_ sdk.ExecMode, _ []byte, tx sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	r.ran.Store(true)
	return sdk.GasInfo{}, &sdk.Result{}, nil, r.pool.Insert(context.Background(), tx)
}

var encodePanicTx = &ptrTx{id: 7}

func withEncodePanic(m *Manager) {
	pool := &storingPool{}
	m.mpool = pool
	m.runner = &insertingRunner{pool: pool}
	withDecodeCache(m, func([]byte) (sdk.Tx, error) { return encodePanicTx, nil }, func(sdk.Tx) ([]byte, error) { panicking(encodeSite); return nil, nil })
}

func (s *RecoverTestSuite) TestAdmissionRecoversPanics() {
	insertViaPeer := func(m *Manager) (uint32, string, error) {
		resp, err := m.InsertTxHandler()(&abci.RequestInsertTx{Tx: []byte("tx")})
		return resp.Code, "", err
	}
	insertViaRPC := func(m *Manager) (uint32, string, error) {
		resp, err := m.InsertTx([]byte("tx"))
		return resp.Code, resp.RawLog, err
	}
	checkWith := func(runTx sdk.RunTx) func(m *Manager) (uint32, string, error) {
		return func(m *Manager) (uint32, string, error) {
			resp, err := m.CheckTxHandler()(runTx, &abci.RequestCheckTx{Tx: []byte("tx")})
			return resp.Code, resp.Log, err
		}
	}

	checkPooling := func(m *Manager) (uint32, string, error) {
		runTx := func(bz []byte, tx sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
			return m.runner.RunTx(sdk.ExecModeCheck, bz, tx, -1, nil, nil)
		}
		return checkWith(runTx)(m)
	}

	_, panicCode, _ := errorsmod.ABCIInfo(errRecoveredPanic, false)
	cometTxHash := fmt.Sprintf("%X", sha256.Sum256([]byte("tx")))

	testCases := []struct {
		name         string
		malleate     func(m *Manager)
		call         func(m *Manager) (uint32, string, error)
		path         string
		where        string // the panic site, as passed to panicking
		encodePanics bool   // the tx must be rejected before RunTx touches ante state or the pool
	}{
		{
			name: "peer InsertTx, panic in preVerify before the admission gate",
			malleate: func(m *Manager) {
				m.SetPreVerify(func([]byte) error { panicking("preVerify"); return nil })
			},
			call:  insertViaPeer,
			path:  panicPathAdmit,
			where: "preVerify",
		},
		{
			name: "RPC InsertTx, panic in RunTx under the admission gate",
			malleate: func(m *Manager) {
				m.runner = &stubRunner{runTx: func([]byte) error { panicking("RunTx"); return nil }}
			},
			call:  insertViaRPC,
			path:  panicPathAdmit,
			where: "RunTx",
		},
		{
			name:         "RPC InsertTx, panic in the encoder before RunTx",
			malleate:     withEncodePanic,
			call:         insertViaRPC,
			path:         panicPathAdmit,
			where:        encodeSite,
			encodePanics: true,
		},
		{
			name: "CheckTx, panic in the decoder before the admission gate",
			malleate: func(m *Manager) {
				withDecodeCache(m, func([]byte) (sdk.Tx, error) { panicking("decode"); return nil, nil }, noopEncoder)
			},
			call:  checkWith(okRunTx),
			path:  panicPathCheckTx,
			where: "decode",
		},
		{
			name:     "CheckTx, panic in runTx under the admission gate",
			malleate: func(*Manager) {},
			call:     checkWith(panicRunTx),
			path:     panicPathCheckTx,
			where:    "runTx",
		},
		{
			name:         "CheckTx, panic in the encoder before runTx",
			malleate:     withEncodePanic,
			call:         checkPooling,
			path:         panicPathCheckTx,
			where:        encodeSite,
			encodePanics: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			m := newManager(&stubRunner{}, nil, noopEncoder, nil)
			logger := &recordingLogger{Logger: log.NewNopLogger()}
			m.logger = logger
			m.SetRPCMaxInflight(1)
			tc.malleate(m)
			epoch := m.pendingTxCache.epoch.Load()

			code, rawLog, err := tc.call(m)

			s.Require().NoError(err, "a panic must surface as a response code, never a Go error")
			s.Require().Len(logger.errors, 1, "a panic is logged once")
			s.Require().Equal(tc.path, keyval(logger.errors[0], "path"))
			s.Require().Equal("boom in "+tc.where, keyval(logger.errors[0], "panic"))
			s.Require().Equal(cometTxHash, keyval(logger.errors[0], "tx"))
			s.Require().Equal(panicCode, code)
			s.Require().Less(code, abci.CodeTypeRetry, "CometBFT must not soon retry a panicking tx")
			s.Require().NotContains(rawLog, "boom", "the panic value must not reach clients")
			s.Require().Equal(epoch, m.pendingTxCache.epoch.Load(), "a rejected tx must not invalidate the pending cache")
			if tc.encodePanics {
				s.Require().Empty(m.PendingTxs(), "a tx whose encoding panics must never be pooled")
				s.Require().False(m.runner.(*insertingRunner).ran.Load(), "RunTx must not run for a tx whose encoding panics")
			}
			s.Require().True(m.gate.mu.TryLock(), "admission gate must be released after a panic")
			m.gate.mu.Unlock()
			release, ok := m.acquireRPCSlot()
			s.Require().True(ok, "RPC slot must be released after a panic")
			release()
		})
	}
}

func (s *RecoverTestSuite) TestNewManagerLogsRecoveredPanics() {
	recording := &recordingLogger{Logger: log.NewNopLogger()}

	// A nil logger must not re-panic inside the recover handler.
	for _, logger := range []log.Logger{recording, nil} {
		app := baseapp.NewBaseApp("test", log.NewNopLogger(), dbm.NewMemDB(), nil)
		m := NewManager(app, logger, nil, noopEncoder, &fakePool{}, nil, nil, 0, 0, true, false)
		m.SetPreVerify(func([]byte) error { panicking("preVerify"); return nil })

		resp, err := m.InsertTx([]byte("tx"))

		m.Close()
		s.Require().NoError(err)
		s.Require().NotZero(resp.Code)
	}
	s.Require().Len(recording.errors, 1, "NewManager must log recovered panics to the given logger")
}

type panickingPool struct{ fakePool }

func (*panickingPool) SelectBy(context.Context, [][]byte, func(sdk.Tx) bool) { panicking("scan") }

type panicGasTx struct{ ptrTx }

func (*panicGasTx) GetGas() uint64     { panicking("GetGas"); return 0 }
func (*panicGasTx) GetFee() sdk.Coins  { return nil }
func (*panicGasTx) FeePayer() []byte   { return nil }
func (*panicGasTx) FeeGranter() []byte { return nil }

func (s *RecoverTestSuite) TestReapRecoversPanics() {
	good1, encPoison, gasPoison, good2 := &ptrTx{id: 1}, &ptrTx{id: 2}, &panicGasTx{ptrTx{id: 4}}, &ptrTx{id: 3}
	encFails := &ptrTx{id: 5}
	encodeByID := func(tx sdk.Tx) ([]byte, error) {
		switch tx := tx.(type) {
		case *panicGasTx:
			return []byte(strconv.Itoa(tx.id)), nil
		case *ptrTx:
			switch tx {
			case encPoison:
				panicking(encodeSite)
			case encFails:
				return nil, sdkerrors.ErrInvalidRequest.Wrap("unencodable")
			}
			return []byte(strconv.Itoa(tx.id)), nil
		}
		return nil, nil
	}

	testCases := []struct {
		name     string
		pool     sdkmempool.Mempool
		expTxs   []string
		expPath  any // path field of each error line; nil for a plain encode error
		expPanic any // panic field of each error line
		expTx    any // tx field of each error line; nil when the tx has no bytes yet
	}{
		{
			name:     "tx whose encode panics is skipped and the rest are gossiped",
			pool:     &fakePool{txs: []sdk.Tx{good1, encPoison, good2}},
			expTxs:   []string{"1", "3"},
			expPath:  panicPathReapTx,
			expPanic: "boom in " + encodeSite,
		},
		{
			// Same SDK code as errRecoveredPanic, so it must still be logged.
			name:   "tx whose encode errors is logged and skipped",
			pool:   &fakePool{txs: []sdk.Tx{good1, encFails, good2}},
			expTxs: []string{"1", "3"},
		},
		{
			name:     "tx whose GetGas panics after earlier txs were marked is skipped",
			pool:     &fakePool{txs: []sdk.Tx{good1, gasPoison, good2}},
			expTxs:   []string{"1", "3"},
			expPath:  panicPathReapTx,
			expPanic: "boom in GetGas",
			expTx:    fmt.Sprintf("%X", sha256.Sum256([]byte("4"))),
		},
		{
			name:     "panic in the pool scan returns an empty reap",
			pool:     &panickingPool{},
			expTxs:   []string{},
			expPath:  panicPathReap,
			expPanic: "boom in scan",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			logger := &recordingLogger{Logger: log.NewNopLogger()}
			reap := NewReapTxsHandler(tc.pool, encodeByID, nil, time.Minute, 0, logger)

			resp, err := reap(&abci.RequestReapTxs{})
			s.Require().NoError(err, "CometBFT drops the response when an error comes with it")
			s.Require().Equal(tc.expTxs, reapedStrings(resp))

			// Gossiped txs are deduped; poison txs stay skipped.
			resp, err = reap(&abci.RequestReapTxs{})
			s.Require().NoError(err)
			s.Require().Empty(reapedStrings(resp))
			s.Require().Len(logger.errors, 2, "each reap logs its one bad tx once")
			for _, keyvals := range logger.errors {
				s.Require().Equal(tc.expPath, keyval(keyvals, "path"))
				s.Require().Equal(tc.expPanic, keyval(keyvals, "panic"))
				s.Require().Equal(tc.expTx, keyval(keyvals, "tx"))
			}
		})
	}
}

type poisonTx struct {
	ptrTx
	timeoutPanics bool
}

func (t *poisonTx) GetTimeoutHeight() uint64 {
	if t.timeoutPanics {
		panicking("GetTimeoutHeight")
	}
	return 0
}

type poisonSigner struct {
	fakeSigner
	poison sdk.Tx
}

func (p poisonSigner) GetSigners(tx sdk.Tx) ([]sdkmempool.SignerData, error) {
	if tx == p.poison {
		panicking("GetSigners")
	}
	return p.fakeSigner.GetSigners(tx)
}

func (s *RecoverTestSuite) TestRecheckRecoversPanics() {
	alice := sdk.AccAddress("alice").String()

	testCases := []struct {
		name       string
		malleate   func(f *recheckFixture, poison *poisonTx)
		path       string
		where      string
		expEvicted bool // poison is evicted and its sibling still rechecked
		expRunTx   bool // poison reached RunTx
	}{
		{
			name:     "panic in the pool scan aborts the cycle and keeps its senders",
			malleate: func(f *recheckFixture, _ *poisonTx) { f.a.mpool = &panickingPool{} },
			path:     panicPathRecheck,
			where:    "scan",
		},
		{
			name:       "tx whose timeout check panics is evicted",
			malleate:   func(_ *recheckFixture, poison *poisonTx) { poison.timeoutPanics = true },
			path:       panicPathRecheckTx,
			where:      "GetTimeoutHeight",
			expEvicted: true,
		},
		{
			name:       "tx whose signer extraction panics is evicted",
			malleate:   func(f *recheckFixture, poison *poisonTx) { f.a.signer = poisonSigner{f.signer, poison} },
			path:       panicPathRecheckTx,
			where:      "GetSigners",
			expEvicted: true,
		},
		{
			name: "tx whose encode panics on a cache miss is evicted",
			malleate: func(f *recheckFixture, poison *poisonTx) {
				f.enc.Evict(poison)
				f.a.txEncoder = func(sdk.Tx) ([]byte, error) { panicking(encodeSite); return nil, nil }
			},
			path:       panicPathRecheckTx,
			where:      encodeSite,
			expEvicted: true,
		},
		{
			name:       "tx whose RunTx panics under the admission gate is evicted",
			malleate:   func(f *recheckFixture, _ *poisonTx) { f.runner.panicBytes = map[string]bool{"alice-0": true} },
			path:       panicPathRecheckTx,
			where:      "RunTx",
			expEvicted: true,
			expRunTx:   true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			f := newRecheckFixture("alice-1") // the sibling fails recheck, so it's evicted only if the cycle goes on
			logger := &recordingLogger{Logger: log.NewNopLogger()}
			f.a.logger = logger
			poison := &poisonTx{ptrTx: ptrTx{id: 1}}
			f.signer.m[poison] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
			s.Require().NoError(f.pool.Insert(sdk.Context{}, poison))
			f.enc.Set(poison, []byte("alice-0"))
			sibling := f.add(2, "alice", 1, "alice-1")
			f.a.recheckSenders = map[string]struct{}{alice: {}}
			f.a.deferred = []sdk.Tx{poison}
			tc.malleate(f, poison)
			startAsyncWorker(f)
			defer f.a.Close()
			epoch := f.a.pendingTxCache.epoch.Load()

			f.a.TriggerRecheck()

			s.Require().False(f.a.WaitForRecheckTimedOut(context.Background(), 5*time.Second), "a panic must still release WaitForRecheck")
			s.Require().Len(logger.errors, 1)
			s.Require().Equal(tc.path, keyval(logger.errors[0], "path"))
			s.Require().Equal("boom in "+tc.where, keyval(logger.errors[0], "panic"))
			s.Require().Equal(!tc.expEvicted, poolHas(f.pool, poison))
			s.Require().Equal(!tc.expEvicted, poolHas(f.pool, sibling))
			s.Require().Equal(tc.expRunTx, f.runner.seen["alice-0"], "an evicted poison tx must not be rechecked")
			if !tc.expEvicted {
				s.Require().Contains(f.a.recheckSenders, alice, "an aborted cycle's senders must be retried")
				s.Require().Equal([]sdk.Tx{poison}, f.a.deferred, "an aborted cycle's carry must be retried")
			}
			s.Require().NotEqual(epoch, f.a.pendingTxCache.epoch.Load(), "the pending cache must be invalidated")
			s.Require().True(f.a.gate.mu.TryLock(), "admission gate must be released after a panic")
			f.a.gate.mu.Unlock()
			s.Require().True(f.a.recheckMu.TryLock(), "recheckMu must be released after a panic")
			f.a.recheckMu.Unlock()

			f.a.TriggerRecheck()
			s.Require().False(f.a.WaitForRecheckTimedOut(context.Background(), 5*time.Second), "the worker must keep serving rechecks")
		})
	}
}

type recordingLogger struct {
	log.Logger
	errors [][]any // keyvals of each Error call
}

func (l *recordingLogger) Error(_ string, keyvals ...any) { l.errors = append(l.errors, keyvals) }

func keyval(keyvals []any, key string) any {
	for i := 0; i+1 < len(keyvals); i += 2 {
		if keyvals[i] == key {
			return keyvals[i+1]
		}
	}
	return nil
}

func reapedStrings(resp *abci.ResponseReapTxs) []string {
	out := make([]string, 0, len(resp.Txs))
	for _, bz := range resp.Txs {
		out = append(out, string(bz))
	}
	return out
}

// freshStackPath clears path's rate-limit state, which outlives a single test run.
func freshStackPath(path string) string {
	lastStackLog.Delete(path)
	return path
}

func (s *RecoverTestSuite) TestStackLogIsRateLimitedPerPath() {
	start := time.Now().UnixNano()
	path, other := freshStackPath(s.T().Name()+"/a"), freshStackPath(s.T().Name()+"/b")

	s.Require().True(shouldLogStack(path, start), "first panic on a path logs its stack")
	s.Require().False(shouldLogStack(path, start+int64(time.Second)), "a repeat within the interval doesn't")
	s.Require().True(shouldLogStack(other, start+int64(time.Second)), "paths are limited independently")
	s.Require().True(shouldLogStack(path, start+int64(stackLogInterval)), "the interval re-arms the stack")
}

func (s *RecoverTestSuite) TestRecoveredPanicLogsStackOncePerInterval() {
	path := freshStackPath(s.T().Name())
	logger := &recordingLogger{Logger: log.NewNopLogger()}

	logRecoveredPanic(logger, path, "boom")
	logRecoveredPanic(logger, path, "boom")

	s.Require().Len(logger.errors, 2, "every panic is logged")
	s.Require().NotNil(keyval(logger.errors[0], "stack"), "the first panic logs its stack")
	s.Require().Nil(keyval(logger.errors[1], "stack"), "a repeat within the interval doesn't")
}

func (s *RecoverTestSuite) TestRecoveredPanicTruncatesValue() {
	atLimit := strings.Repeat("x", maxPanicLogLen)
	long := atLimit + "x"

	testCases := []struct {
		name     string
		value    any
		expValue string
	}{
		{name: "short value is logged as is", value: "boom", expValue: "boom"},
		{name: "non-string value is formatted", value: 42, expValue: "42"},
		{name: "value at the limit is not truncated", value: atLimit, expValue: atLimit},
		{name: "long value is truncated", value: long, expValue: atLimit + "...(truncated)"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			logger := &recordingLogger{Logger: log.NewNopLogger()}

			logRecoveredPanic(logger, freshStackPath(s.T().Name()), tc.value)

			s.Require().Equal(tc.expValue, keyval(logger.errors[0], "panic"))
		})
	}
}

func (s *RecoverTestSuite) TestStackLogConcurrentPanicsLogOnce() {
	const callers = 64

	testCases := []struct {
		name     string
		lastSeen func(now int64) int64 // 0 = no stack logged yet
	}{
		{name: "first panic on the path", lastSeen: func(int64) int64 { return 0 }},
		{name: "last stack older than the interval", lastSeen: func(now int64) int64 { return now - int64(stackLogInterval) }},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			path, now := freshStackPath(s.T().Name()), time.Now().UnixNano()
			if seen := tc.lastSeen(now); seen != 0 {
				last := new(atomic.Int64)
				last.Store(seen)
				lastStackLog.Store(path, last)
			}

			var logged atomic.Int64
			var wg sync.WaitGroup
			start := make(chan struct{})
			for range callers {
				wg.Go(func() {
					<-start
					if shouldLogStack(path, now) {
						logged.Add(1)
					}
				})
			}
			close(start)
			wg.Wait()

			s.Require().Equal(int64(1), logged.Load(), "only the CAS winner logs the stack")
		})
	}
}
