package app

import (
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethante "github.com/evmos/ethermint/ante"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// newEVMSigPreVerifier returns a lock-free EVM sig checker for the Admitter: runs ecrecover
// outside the admission mutex. Returns nil for non-EVM txs, undecodable bytes, or unparseable chain ID.
func newEVMSigPreVerifier(app *App, decoder sdk.TxDecoder) func([]byte) error {
	chainID, err := ethermint.ParseChainID(app.ChainID())
	if err != nil {
		return nil // not an EVM chain ID; leave admission fully locked
	}
	signer := ethtypes.LatestSignerForChainID(chainID)

	return func(raw []byte) error {
		tx, err := decoder(raw)
		if err != nil {
			return nil // let the locked RunTx surface the canonical decode error
		}
		msgs := tx.GetMsgs()
		if len(msgs) == 0 {
			return nil
		}
		for _, msg := range msgs {
			if _, ok := msg.(*evmtypes.MsgEthereumTx); !ok {
				return nil // not a pure EVM tx; the locked path verifies it
			}
		}
		return ethante.VerifyEthSig(tx, signer)
	}
}
