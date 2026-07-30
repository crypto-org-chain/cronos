package mempool

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// admitter is the admission half of the app mempool: peer-relayed InsertTx and
// RPC CheckTx, both validated by RunTx against the shared branch.
type admitter struct {
	exec  *txExec
	trace bool
	// preVerify runs cheap verification lock-free before the tx admission mutex; set to nil for skip.
	preVerify func([]byte) error
}

// admit is the shared admission path: preVerify + decode unlocked (bad txs skip
// the mutex), then RunTx(ExecModeCheck) + cacheTx under it. Over-capacity maps to
// CodeTypeRetry. tx stays nil when encCache is nil; BaseApp.RunTx accepts nil
// sdk.Tx (uses txBytes).
func (a *admitter) admit(txBytes []byte) (code uint32, codespace, log string) {
	if a.preVerify != nil {
		if err := a.preVerify(txBytes); err != nil {
			cs, c, l := errorsmod.ABCIInfo(err, false)
			return c, cs, l
		}
	}

	var tx sdk.Tx
	if a.exec.encCache != nil {
		var err error
		if tx, err = a.exec.decoder(txBytes); err != nil {
			cs, c, l := errorsmod.ABCIInfo(sdkerrors.ErrTxDecode.Wrap(err.Error()), false)
			return c, cs, l
		}
	}

	a.exec.mu.Lock()
	defer a.exec.mu.Unlock()

	_, _, _, err := a.exec.runTxLocked(sdk.ExecModeCheck, txBytes, tx)
	if err != nil {
		if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
			return abci.CodeTypeRetry, "", "mempool is full"
		}
		cs, c, l := errorsmod.ABCIInfo(err, false)
		return c, cs, l
	}

	a.cacheTx(tx, txBytes)
	return abci.CodeTypeOK, "", ""
}

// cacheTx registers the already-decoded tx under its canonical bytes (raw
// req.Tx bytes on encode error). No-op without a cache.
func (a *admitter) cacheTx(tx sdk.Tx, raw []byte) {
	if a.exec.encCache == nil {
		return
	}
	bz := raw
	if canonical, err := a.exec.txEncoder(tx); err == nil {
		bz = canonical
	}
	a.exec.encCache.Set(tx, bz)
}

// insertTxHandler validates peer-relayed txs via RunTx(ExecModeCheck) before
// admitting them.
func (a *admitter) insertTxHandler() sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		code, _, _ := a.admit(req.Tx)
		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

// checkTxHandler runs RPC CheckTx. It calls the runner directly instead of the
// runTx closure baseapp passes in (abci.go CheckTx), which hardcodes
// txMultiStore = nil; the exec-mode mapping below mirrors BaseApp.CheckTx so
// req.Type stays authoritative.
func (a *admitter) checkTxHandler() sdk.CheckTxHandler {
	return func(_ sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		var mode sdk.ExecMode
		switch req.Type {
		case abci.CheckTxType_New:
			mode = sdk.ExecModeCheck
		case abci.CheckTxType_Recheck:
			mode = sdk.ExecModeReCheck
		default:
			return nil, fmt.Errorf("unknown RequestCheckTx type: %s", req.Type)
		}

		// Decode before locking: proto unmarshal is CPU-intensive; decoder and
		// DecodeCache have their own locks. Bad txs return without acquiring the mutex.
		var tx sdk.Tx
		if a.exec.encCache != nil {
			var err error
			if tx, err = a.exec.decoder(req.Tx); err != nil {
				return sdkerrors.ResponseCheckTxWithEvents(sdkerrors.ErrTxDecode.Wrap(err.Error()), 0, 0, nil, a.trace), nil
			}
		}

		a.exec.mu.Lock()
		defer a.exec.mu.Unlock()

		gasInfo, result, anteEvents, err := a.exec.runTxLocked(mode, req.Tx, tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		a.cacheTx(tx, req.Tx)

		// No MarkEventsToIndex (unlike default CheckTx): that flag only feeds
		// the tx indexer on FinalizeBlock results, not CheckTx.
		return &abci.ResponseCheckTx{
			GasWanted: int64(gasInfo.GasWanted),
			GasUsed:   int64(gasInfo.GasUsed),
			Log:       result.Log,
			Data:      result.Data,
			Events:    result.Events,
		}, nil
	}
}
