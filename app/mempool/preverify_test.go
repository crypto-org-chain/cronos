package mempool

import (
	"errors"
	"testing"

	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestEVMSigPreVerifierGate(t *testing.T) {
	verify := NewEVMSigPreVerifier("cronos_777-1", nil)
	require.NotNil(t, verify)

	// A MsgEthereumTx with neither Raw nor Data fails VerifyEthSig, so an error shows the gate let it through.
	testCases := []struct {
		name    string
		msgs    []sdk.Msg
		wantErr error
	}{
		{name: "no msgs defers"},
		{name: "mixed EVM and cosmos msgs defers", msgs: []sdk.Msg{&evmtypes.MsgEthereumTx{}, &banktypes.MsgSend{}}},
		{name: "pure EVM msgs are verified", msgs: []sdk.Msg{&evmtypes.MsgEthereumTx{}}, wantErr: sdkerrors.ErrUnknownRequest},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := verify(&fakeTx{msgs: tc.msgs})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPreVerifierRegistry(t *testing.T) {
	reject := errors.New("reject")

	t.Run("empty registry defers", func(t *testing.T) {
		var r PreVerifierRegistry
		require.NoError(t, r.Verify(nil))
	})

	t.Run("nil verifier ignored", func(t *testing.T) {
		var r PreVerifierRegistry
		r.Register(nil)
		require.NoError(t, r.Verify(nil))
	})

	t.Run("first rejection wins, later verifiers not run", func(t *testing.T) {
		var r PreVerifierRegistry
		ran := false
		r.Register(func(sdk.Tx) error { return reject })
		r.Register(func(sdk.Tx) error { ran = true; return nil })
		require.ErrorIs(t, r.Verify(nil), reject)
		require.False(t, ran, "verifier after a rejection must not run")
	})

	t.Run("all defer", func(t *testing.T) {
		var r PreVerifierRegistry
		count := 0
		r.Register(func(sdk.Tx) error { count++; return nil })
		r.Register(func(sdk.Tx) error { count++; return nil })
		require.NoError(t, r.Verify(nil))
		require.Equal(t, 2, count)
	})
}
