package mempool

import (
	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

const (
	mempoolFullLog    = "mempool is full"
	rpcTxLimitFullLog = "too many in-flight RPC txs"
)

// admitSource decides whether an admission counts against rpcTxLimit.
type admitSource int

const (
	fromGossip admitSource = iota
	fromRPC
)

// admitter is the admission half of the app mempool: peer-relayed InsertTx and
// RPC CheckTx, both validated by RunTx under the shared gate.
type admitter struct {
	exec  *txExec
	trace bool
	// preVerify runs cheap verification lock-free before the tx admission mutex; set to nil for skip.
	preVerify func([]byte) error
	// rpcTxLimit is applied to RPC admissions only; gossip is already bounded by
	// p2p receive concurrency.
	rpcTxLimit inflightLimit
}

// acquireRPCSlot takes an RPC admission slot and returns its release; when none
// is free it counts a shed and returns ok=false.
func (a *admitter) acquireRPCSlot() (release func(), ok bool) {
	if !a.rpcTxLimit.tryAcquire() {
		telemetry.IncrCounter(1, "cronos", "mempool", "rpc", "shed")
		return nil, false
	}
	return a.rpcTxLimit.release, true
}

// admit is the shared admission path: preVerify + decode unlocked (bad txs skip
// the mutex), then RunTx(ExecModeCheck) + cacheTx under it. Over-capacity and,
// for RPC txs, a full rpcTxLimit map to CodeTypeRetry. tx stays nil when
// encCache is nil; BaseApp.RunTx accepts nil sdk.Tx (uses txBytes).
func (a *admitter) admit(txBytes []byte, source admitSource) (code uint32, codespace, log string) {
	if source == fromRPC {
		release, ok := a.acquireRPCSlot()
		if !ok {
			return abci.CodeTypeRetry, "", rpcTxLimitFullLog
		}
		defer release()
	}

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

	a.exec.gate.lock()
	defer a.exec.gate.unlock()

	_, _, _, err := a.exec.runTxLocked(sdk.ExecModeCheck, txBytes, tx)
	if err != nil {
		if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
			return abci.CodeTypeRetry, "", mempoolFullLog
		}
		cs, c, l := errorsmod.ABCIInfo(err, false)
		return c, cs, l
	}

	a.cacheTx(tx, txBytes)
	a.exec.pending.invalidate()
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
		code, _, _ := a.admit(req.Tx, fromGossip)
		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

// checkTxHandler runs RPC CheckTx through the runTx closure baseapp passes in,
// which already maps req.Type to the exec mode.
func (a *admitter) checkTxHandler() sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		release, ok := a.acquireRPCSlot()
		if !ok {
			return &abci.ResponseCheckTx{Code: abci.CodeTypeRetry, Log: rpcTxLimitFullLog}, nil
		}
		defer release()

		// Decode before locking: proto unmarshal is CPU-intensive; decoder and
		// DecodeCache have their own locks. Bad txs return without acquiring the mutex.
		var tx sdk.Tx
		if a.exec.encCache != nil {
			var err error
			if tx, err = a.exec.decoder(req.Tx); err != nil {
				return sdkerrors.ResponseCheckTxWithEvents(sdkerrors.ErrTxDecode.Wrap(err.Error()), 0, 0, nil, a.trace), nil
			}
		}

		a.exec.gate.lock()
		defer a.exec.gate.unlock()

		gasInfo, result, anteEvents, err := runTx(req.Tx, tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		a.cacheTx(tx, req.Tx)
		a.exec.pending.invalidate()

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
