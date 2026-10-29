package mempool

import (
	"context"
	"strconv"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

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

	panickingEncoder := func(sdk.Tx) ([]byte, error) { panicking("encode"); return nil, nil }

	testCases := []struct {
		name     string
		malleate func(m *Manager)
		call     func(m *Manager) (uint32, error)
		// admitted cases panic after RunTx pooled the tx: the response must stay OK
		// and the pending cache must still see the tx.
		admitted bool
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
			malleate: func(m *Manager) { withDecodeCache(m, decodePtrTx, panickingEncoder) },
			call:     insertViaRPC,
			admitted: true,
		},
		{
			name:     "peer InsertTx, panic in cacheTx after the tx was admitted",
			malleate: func(m *Manager) { withDecodeCache(m, decodePtrTx, panickingEncoder) },
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
			malleate: func(m *Manager) { withDecodeCache(m, decodePtrTx, panickingEncoder) },
			call:     checkWith(okRunTx),
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
			} else {
				s.Require().Equal(sdkerrors.ErrPanic.ABCICode(), code)
			}
			s.Require().True(m.mu.TryLock(), "admission mutex must be released after a panic")
			m.mu.Unlock()
		})
	}
}

// panickingPool panics while the reap scans it.
type panickingPool struct{ fakePool }

func (*panickingPool) SelectBy(context.Context, [][]byte, func(sdk.Tx) bool) { panicking("scan") }

func (s *RecoverTestSuite) TestReapRecoversPanics() {
	good1, poison, good2 := &ptrTx{id: 1}, &ptrTx{id: 2}, &ptrTx{id: 3}
	encodeByID := func(tx sdk.Tx) ([]byte, error) {
		if tx == poison {
			panicking("encode")
		}
		return []byte(strconv.Itoa(tx.(*ptrTx).id)), nil
	}

	testCases := []struct {
		name    string
		pool    sdkmempool.Mempool
		expTxs  []string
		expErr  error
		expNext []string // txs reaped by a second call on the same handler
	}{
		{
			name:    "poison tx is skipped and the rest are gossiped",
			pool:    &fakePool{txs: []sdk.Tx{good1, poison, good2}},
			expTxs:  []string{"1", "3"},
			expNext: []string{},
		},
		{
			name:    "panic in the pool scan returns an empty reap and the error",
			pool:    &panickingPool{},
			expTxs:  []string{},
			expErr:  sdkerrors.ErrPanic,
			expNext: []string{},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			reap := NewReapTxsHandler(tc.pool, encodeByID, nil, time.Minute, 0, log.NewNopLogger())

			resp, err := reap(&abci.RequestReapTxs{})
			if tc.expErr != nil {
				s.Require().ErrorIs(err, tc.expErr)
			} else {
				s.Require().NoError(err)
			}
			s.Require().Equal(tc.expTxs, reapedStrings(resp))

			// Gossiped txs are deduped and the poison tx keeps being skipped without failing the reap.
			resp, err = reap(&abci.RequestReapTxs{})
			if tc.expErr != nil {
				s.Require().ErrorIs(err, tc.expErr)
			} else {
				s.Require().NoError(err)
			}
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

func (s *RecoverTestSuite) TestStackLogIsRateLimitedPerPath() {
	start := time.Now().UnixNano()
	path, other := s.T().Name()+"/a", s.T().Name()+"/b"

	s.Require().True(shouldLogStack(path, start), "first panic on a path logs its stack")
	s.Require().False(shouldLogStack(path, start+int64(time.Second)), "a repeat within the interval doesn't")
	s.Require().True(shouldLogStack(other, start+int64(time.Second)), "paths are limited independently")
	s.Require().True(shouldLogStack(path, start+int64(stackLogInterval)), "the interval re-arms the stack")
}
