package mempool

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/ethereum/go-ethereum/common"
	antecache "github.com/evmos/ethermint/ante/cache"
	"github.com/stretchr/testify/suite"

	errorsmod "cosmossdk.io/errors"
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

type storingPool struct{ fakePool }

func (p *storingPool) Insert(_ context.Context, tx sdk.Tx) error {
	p.txs = append(p.txs, tx)
	return nil
}

func (p *storingPool) Remove(tx sdk.Tx) error {
	p.txs = slices.DeleteFunc(p.txs, func(pooled sdk.Tx) bool { return pooled == tx })
	return nil
}

// insertingRunner caches the nonce and pools the tx like BaseApp.RunTx(ExecModeCheck).
type insertingRunner struct {
	pool sdkmempool.Mempool
	ante *antecache.AnteCache
}

func (r insertingRunner) RunTx(_ sdk.ExecMode, _ []byte, tx sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	if eth, ok := tx.(*ethTx); ok {
		r.ante.Set(anteKey(eth))
	}
	return sdk.GasInfo{}, &sdk.Result{}, nil, r.pool.Insert(context.Background(), tx)
}

func withPoolingThenEncodePanic(m *Manager) {
	pool, ante := &storingPool{}, antecache.NewAnteCache(0)
	m.mpool = pool
	m.runner = insertingRunner{pool: pool, ante: ante}
	m.SetAnteCache(ante)
	tx := newEthTx(common.Address{0x1}, 7)
	withDecodeCache(m, func([]byte) (sdk.Tx, error) { return tx, nil }, func(sdk.Tx) ([]byte, error) { panicking("encode"); return nil, nil })
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

	testCases := []struct {
		name     string
		malleate func(m *Manager)
		call     func(m *Manager) (uint32, string, error)
		pooled   bool // RunTx pooled the tx before the panic, so it must be evicted
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
			name:     "RPC InsertTx, panic in cacheTx after RunTx pooled the tx",
			malleate: withPoolingThenEncodePanic,
			call:     insertViaRPC,
			pooled:   true,
		},
		{
			name:     "peer InsertTx, panic in cacheTx after RunTx pooled the tx",
			malleate: withPoolingThenEncodePanic,
			call:     insertViaPeer,
			pooled:   true,
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
			name:     "CheckTx, panic in cacheTx after runTx pooled the tx",
			malleate: withPoolingThenEncodePanic,
			call:     checkPooling,
			pooled:   true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			m := newManager(&stubRunner{}, nil, noopEncoder, nil)
			tc.malleate(m)
			epoch := m.pendingTxCache.epoch.Load()

			code, rawLog, err := tc.call(m)

			s.Require().NoError(err, "a panic must surface as a response code, never a Go error")
			s.Require().Equal(panicCode, code)
			s.Require().Less(code, abci.CodeTypeRetry, "CometBFT must not soon retry a panicking tx")
			s.Require().NotContains(rawLog, "boom", "the panic value must not reach clients")
			if tc.pooled {
				s.Require().Equal(epoch+1, m.pendingTxCache.epoch.Load(), "pending cache must be invalidated after the eviction")
				s.Require().Empty(m.PendingTxs(), "a tx whose encoding panics must be evicted")
				tx, _ := m.decoder(nil)
				eth, ok := tx.(*ethTx)
				s.Require().True(ok)
				s.Require().False(m.anteCache.Exists(anteKey(eth)), "eviction must clear the tx's ante-cache nonce")
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
	encFails := &ptrTx{id: 5}
	encodeByID := func(tx sdk.Tx) ([]byte, error) {
		switch tx := tx.(type) {
		case *panicGasTx:
			return []byte(strconv.Itoa(tx.id)), nil
		case *ptrTx:
			switch tx {
			case encPoison:
				panicking("encode")
			case encFails:
				return nil, sdkerrors.ErrInvalidRequest.Wrap("unencodable")
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
			// Same SDK code as errRecoveredPanic, so it must still be logged.
			name:    "tx whose encode errors is logged and skipped",
			pool:    &fakePool{txs: []sdk.Tx{good1, encFails, good2}},
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
			logger := &errorCountingLogger{Logger: log.NewNopLogger()}
			reap := NewReapTxsHandler(tc.pool, encodeByID, nil, time.Minute, 0, logger)

			resp, err := reap(&abci.RequestReapTxs{})
			s.Require().NoError(err, "CometBFT drops the response when an error comes with it")
			s.Require().Equal(tc.expTxs, reapedStrings(resp))

			// Gossiped txs are deduped; poison txs stay skipped.
			resp, err = reap(&abci.RequestReapTxs{})
			s.Require().NoError(err)
			s.Require().Equal(tc.expNext, reapedStrings(resp))
			s.Require().Equal(int64(2), logger.errors.Load(), "each reap logs its one bad tx once")
		})
	}
}

type errorCountingLogger struct {
	log.Logger
	errors atomic.Int64
}

func (l *errorCountingLogger) Error(string, ...any) { l.errors.Add(1) }

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
