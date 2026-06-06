package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethante "github.com/evmos/ethermint/ante"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

// newEVMSigPreVerifier builds the lock-free pre-verification hook for the
// app-mempool Admitter (mempool.type=app). It runs the stateless EVM signature
// check (ecrecover) outside the admission mutex, where it dominates cost yet
// touches no store.
//
// It returns nil — deferring to the fully-locked RunTx — for anything it can't
// cheaply pre-verify: non-EVM (or mixed) txs, undecodable bytes, or (at build
// time) a chain ID that doesn't parse. Only a genuine signature failure on a
// pure-EVM tx rejects early.
//
// The signer is the latest signer for the chain's EIP-155 ID, parsed once from
// the immutable BaseApp chain-id string. We deliberately avoid EvmKeeper /
// EVMBlockConfig here: those read keeper state that BeginBlock rewrites every
// block (k.eip155ChainID) and write a per-block object-store cache, neither safe
// to touch lock-free. LatestSignerForChainID is pure and, for a chain past all
// signer-relevant forks, recovers the same sender the in-lock ante's MakeSigner
// would — so a tx accepted here is accepted there. decoder is the caching tx
// decoder, whose cache is mutex-guarded (safe to call concurrently).
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
