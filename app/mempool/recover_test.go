package mempool

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

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

func panicking(where string) { panic("boom in " + where) }

func okRunTx([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

func panicRunTx([]byte, sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	panicking("runTx")
	return sdk.GasInfo{}, nil, nil, nil
}

// withDecodeCache turns on the decode path so decoder and cacheTx panics are reachable.
func withDecodeCache(m *Manager, decoder sdk.TxDecoder, encoder sdk.TxEncoder) {
	m.encCache = NewEncoderCache(0, 0)
	m.decoder = decoder
	m.txEncoder = encoder
}

func decodePtrTx([]byte) (sdk.Tx, error) { return &ptrTx{}, nil }

type storingPool struct{ fakePool }

func (p *storingPool) Insert(_ context.Context, tx sdk.Tx) error {
	p.txs = append(p.txs, tx)
	return nil
}

// insertingRunner pools the tx like BaseApp.RunTx(ExecModeCheck).
type insertingRunner struct{ pool sdkmempool.Mempool }

func (r insertingRunner) RunTx(_ sdk.ExecMode, _ []byte, tx sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	return sdk.GasInfo{}, &sdk.Result{}, nil, r.pool.Insert(context.Background(), tx)
}

// withAdmittingPool makes admission pool the tx, then panics in cacheTx.
func withAdmittingPool(m *Manager) {
	pool := &storingPool{}
	m.mpool = pool
	m.runner = insertingRunner{pool: pool}
	withDecodeCache(m, decodePtrTx, func(sdk.Tx) ([]byte, error) { panicking("encode"); return nil, nil })
}

func (s *RecoverTestSuite) TestAdmissionRecoversPanics() {
	insertViaPeer := func(m *Manager) (uint32, error) {
		resp, err := m.InsertTxHandler()(&abci.RequestInsertTx{Tx: []byte("tx")})
		return resp.Code, err
	}
	insertViaRPC := func(m *Manager) (uint32, error) {
		resp, err := m.InsertTx([]byte("tx"))
		return resp.Code, err
	}
	checkWith := func(runTx sdk.RunTx) func(m *Manager) (uint32, error) {
		return func(m *Manager) (uint32, error) {
			resp, err := m.CheckTxHandler()(runTx, &abci.RequestCheckTx{Tx: []byte("tx")})
			return resp.Code, err
		}
	}

	checkPooling := func(m *Manager) (uint32, error) {
		runTx := func(_ []byte, tx sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
			return sdk.GasInfo{}, &sdk.Result{}, nil, m.mpool.Insert(context.Background(), tx)
		}
		return checkWith(runTx)(m)
	}

	testCases := []struct {
		name     string
		malleate func(m *Manager)
		call     func(m *Manager) (uint32, error)
		admitted bool // panics after RunTx pooled the tx, so the response stays OK
	}{
		{
			name: "peer InsertTx, panic in preVerify before the admission mutex",
			malleate: func(m *Manager) {
				m.SetPreVerify(func([]byte) error { panicking("preVerify"); return nil })
			},
			call: insertViaPeer,
		},
		{
			name: "RPC InsertTx, panic in RunTx under the admission mutex",
			malleate: func(m *Manager) {
				m.runner = &stubRunner{runTx: func([]byte) error { panicking("RunTx"); return nil }}
			},
			call: insertViaRPC,
		},
		{
			name:     "RPC InsertTx, panic in cacheTx after the tx was admitted",
			malleate: withAdmittingPool,
			call:     insertViaRPC,
			admitted: true,
		},
		{
			name:     "peer InsertTx, panic in cacheTx after the tx was admitted",
			malleate: withAdmittingPool,
			call:     insertViaPeer,
			admitted: true,
		},
		{
			name: "CheckTx, panic in the decoder before the admission mutex",
			malleate: func(m *Manager) {
				withDecodeCache(m, func([]byte) (sdk.Tx, error) { panicking("decode"); return nil, nil }, noopEncoder)
			},
			call: checkWith(okRunTx),
		},
		{
			name:     "CheckTx, panic in runTx under the admission mutex",
			malleate: func(*Manager) {},
			call:     checkWith(panicRunTx),
		},
		{
			name:     "CheckTx, panic in cacheTx after runTx succeeded",
			malleate: withAdmittingPool,
			call:     checkPooling,
			admitted: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			m := newManager(&stubRunner{}, nil, noopEncoder, nil)
			tc.malleate(m)
			epoch := m.pendingTxCache.epoch.Load()

			code, err := tc.call(m)

			s.Require().NoError(err, "a panic must surface as a response code, never a Go error")
			if tc.admitted {
				s.Require().Equal(abci.CodeTypeOK, code)
				s.Require().Equal(epoch+1, m.pendingTxCache.epoch.Load(), "pending cache must be invalidated for a pooled tx")
				s.Require().Len(m.PendingTxs(), 1, "the admitted tx must stay in the pool")
			} else {
				s.Require().Equal(sdkerrors.ErrPanic.ABCICode(), code)
			}
			s.Require().True(m.mu.TryLock(), "admission mutex must be released after a panic")
			m.mu.Unlock()
		})
	}
}

type panickingPool struct{ fakePool }

func (*panickingPool) SelectBy(context.Context, [][]byte, func(sdk.Tx) bool) { panicking("scan") }

// panicGasTx's GetGas runs after earlier txs are marked gossiped.
type panicGasTx struct{ ptrTx }

func (*panicGasTx) GetGas() uint64     { panicking("GetGas"); return 0 }
func (*panicGasTx) GetFee() sdk.Coins  { return nil }
func (*panicGasTx) FeePayer() []byte   { return nil }
func (*panicGasTx) FeeGranter() []byte { return nil }

func (s *RecoverTestSuite) TestReapRecoversPanics() {
	good1, encPoison, gasPoison, good2 := &ptrTx{id: 1}, &ptrTx{id: 2}, &panicGasTx{ptrTx{id: 4}}, &ptrTx{id: 3}
	encodeByID := func(tx sdk.Tx) ([]byte, error) {
		switch tx := tx.(type) {
		case *panicGasTx:
			return []byte(strconv.Itoa(tx.id)), nil
		case *ptrTx:
			if tx == encPoison {
				panicking("encode")
			}
			return []byte(strconv.Itoa(tx.id)), nil
		}
		return nil, nil
	}

	testCases := []struct {
		name    string
		pool    sdkmempool.Mempool
		expTxs  []string
		expNext []string // txs reaped by a second call on the same handler
	}{
		{
			name:    "tx whose encode panics is skipped and the rest are gossiped",
			pool:    &fakePool{txs: []sdk.Tx{good1, encPoison, good2}},
			expTxs:  []string{"1", "3"},
			expNext: []string{},
		},
		{
			name:    "tx whose GetGas panics after earlier txs were marked is skipped",
			pool:    &fakePool{txs: []sdk.Tx{good1, gasPoison, good2}},
			expTxs:  []string{"1", "3"},
			expNext: []string{},
		},
		{
			name:    "panic in the pool scan returns an empty reap",
			pool:    &panickingPool{},
			expTxs:  []string{},
			expNext: []string{},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			reap := NewReapTxsHandler(tc.pool, encodeByID, nil, time.Minute, 0, log.NewNopLogger())

			resp, err := reap(&abci.RequestReapTxs{})
			s.Require().NoError(err, "CometBFT drops the response when an error comes with it")
			s.Require().Equal(tc.expTxs, reapedStrings(resp))

			// Gossiped txs are deduped; poison txs stay skipped.
			resp, err = reap(&abci.RequestReapTxs{})
			s.Require().NoError(err)
			s.Require().Equal(tc.expNext, reapedStrings(resp))
		})
	}
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
