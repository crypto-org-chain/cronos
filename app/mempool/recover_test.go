package mempool

import (
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
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

func (s *RecoverTestSuite) TestAdmissionPanicReturnsErrPanicCode() {
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

	testCases := []struct {
		name     string
		malleate func(m *Manager)
		call     func(m *Manager) (uint32, error)
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
			name: "RPC InsertTx, panic in cacheTx after the tx was admitted",
			malleate: func(m *Manager) {
				withDecodeCache(m, decodePtrTx, func(sdk.Tx) ([]byte, error) { panicking("encode"); return nil, nil })
			},
			call: insertViaRPC,
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
			name: "CheckTx, panic in cacheTx after runTx succeeded",
			malleate: func(m *Manager) {
				withDecodeCache(m, decodePtrTx, func(sdk.Tx) ([]byte, error) { panicking("encode"); return nil, nil })
			},
			call: checkWith(okRunTx),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			m := newManager(&stubRunner{}, nil, noopEncoder, nil)
			tc.malleate(m)

			code, err := tc.call(m)

			s.Require().NoError(err, "a panic must surface as a response code, never a Go error")
			s.Require().Equal(sdkerrors.ErrPanic.ABCICode(), code)
			s.Require().True(m.mu.TryLock(), "admission mutex must be released after a panic")
			m.mu.Unlock()
		})
	}
}

func (s *RecoverTestSuite) TestReapPanicReturnsEmptyResponse() {
	pool := &fakePool{txs: []sdk.Tx{&ptrTx{id: 1}}}
	var shouldPanic atomic.Bool
	shouldPanic.Store(true)
	encoder := func(sdk.Tx) ([]byte, error) {
		if shouldPanic.Load() {
			panicking("encode")
		}
		return []byte("tx"), nil
	}
	reap := NewReapTxsHandler(pool, encoder, nil, time.Minute, 0, log.NewNopLogger())

	resp, err := reap(&abci.RequestReapTxs{})
	s.Require().ErrorIs(err, sdkerrors.ErrPanic)
	s.Require().NotNil(resp)
	s.Require().Empty(resp.Txs)

	// The panic hit before the tx was marked gossiped, so the same handler reaps it next time.
	shouldPanic.Store(false)
	resp, err = reap(&abci.RequestReapTxs{})
	s.Require().NoError(err)
	s.Require().Len(resp.Txs, 1)
}
