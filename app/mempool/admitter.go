package mempool

import (
	"sync"

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

// admitter admits peer InsertTx and RPC CheckTx via RunTx under the gate.
type admitter struct {
	exec *txExec
	// encodeMu serializes the pre-RunTx encode, which writes empty body/auth-info bytes into a
	// shared decoded tx. Pooled txs passed ante, so other encodes only read. Never held with gate.
	encodeMu sync.Mutex
	trace    bool
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

// admit is the shared admission path: preVerify + decodeAndEncode unlocked (bad txs skip
// the gate), then RunTx(ExecModeCheck) + encCache.Set under it. Over-capacity and,
// for RPC txs, a full rpcTxLimit map to CodeTypeRetry. tx stays nil when
// encCache is nil; BaseApp.RunTx accepts nil sdk.Tx (uses txBytes).
func (a *admitter) admit(txBytes []byte, source admitSource) (code uint32, codespace, rawLog string) {
	defer func() {
		if r := recover(); r != nil {
			codespace, code, rawLog = errorsmod.ABCIInfo(recoveredPanic(a.exec.logger, panicPathAdmit, r, "tx", txHash(txBytes)), false)
		}
	}()

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

	tx, canonical, err := a.decodeAndEncode(txBytes)
	if err != nil {
		cs, c, l := errorsmod.ABCIInfo(err, false)
		return c, cs, l
	}

	a.exec.gate.lock()
	defer a.exec.gate.unlock()

	if _, _, _, err = a.exec.runTxLocked(sdk.ExecModeCheck, txBytes, tx); err != nil {
		if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
			return abci.CodeTypeRetry, "", mempoolFullLog
		}
		cs, c, l := errorsmod.ABCIInfo(err, false)
		return c, cs, l
	}

	a.exec.encCache.Set(tx, canonical)
	a.exec.pending.invalidate()
	return abci.CodeTypeOK, "", ""
}

// Encoding before RunTx keeps a tx whose encoding panics out of ante and the pool; nils without a cache, raw bytes on encode error.
func (a *admitter) decodeAndEncode(raw []byte) (sdk.Tx, []byte, error) {
	if a.exec.encCache == nil {
		return nil, nil, nil
	}
	tx, err := a.exec.decoder(raw)
	if err != nil {
		return nil, nil, sdkerrors.ErrTxDecode.Wrap(err.Error())
	}
	if bz, err := a.encode(tx); err == nil {
		return tx, bz, nil
	}
	return tx, raw, nil
}

// Deferred unlock: a recovered encoder panic must not leave encodeMu held.
func (a *admitter) encode(tx sdk.Tx) ([]byte, error) {
	a.encodeMu.Lock()
	defer a.encodeMu.Unlock()
	return a.exec.txEncoder(tx)
}

func (a *admitter) insertTxHandler() sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		code, _, _ := a.admit(req.Tx, fromGossip)
		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

func (a *admitter) checkTxHandler() sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (resp *abci.ResponseCheckTx, _ error) {
		defer func() {
			if r := recover(); r != nil {
				resp = sdkerrors.ResponseCheckTxWithEvents(recoveredPanic(a.exec.logger, panicPathCheckTx, r, "tx", txHash(req.Tx)), 0, 0, nil, a.trace)
			}
		}()

		release, ok := a.acquireRPCSlot()
		if !ok {
			return &abci.ResponseCheckTx{Code: abci.CodeTypeRetry, Log: rpcTxLimitFullLog}, nil
		}
		defer release()

		// Decode and encode before locking
		tx, canonical, err := a.decodeAndEncode(req.Tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, 0, 0, nil, a.trace), nil
		}

		a.exec.gate.lock()
		defer a.exec.gate.unlock()

		gasInfo, result, anteEvents, err := runTx(req.Tx, tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		a.exec.encCache.Set(tx, canonical)
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
