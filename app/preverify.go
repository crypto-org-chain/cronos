package app

import (
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethante "github.com/evmos/ethermint/ante"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// newEVMSigPreVerifier builds the lock-free pre-verification hook for the
// app-mempool Admitter (mempool.type=app): a stateless ecrecover run outside
// the admission mutex, where it dominates cost yet touches no store.
//
// It returns nil — deferring to the locked RunTx — for anything it can't cheaply
// pre-verify: non-EVM (or mixed) txs, undecodable bytes, or an unparseable chain
// ID. Only a genuine signature failure on a pure-EVM tx rejects early.
//
// The signer is parsed once from the immutable BaseApp chain-id string. We avoid
// EvmKeeper / EVMBlockConfig: those read keeper state BeginBlock rewrites every
// block (k.eip155ChainID) and write a per-block cache, neither lock-free-safe.
// LatestSignerForChainID is pure and, past all signer-relevant forks, recovers
// the same sender the in-lock ante's MakeSigner would. decoder is the caching
// decoder, whose cache is mutex-guarded.
//
// Pre-existing race (tracked, not fixed here): lock-free admission reads
// EvmKeeper.eip155ChainID while FinalizeBlock → BeginBlock → WithChainIDString
// rewrites it (same value, but a data race under -race). The ethermint fork
// guards the write with "skip when unchanged". Until that lands, running with
// -race on the app-mempool path will surface this as a false positive.
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
