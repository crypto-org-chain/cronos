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

// admitter admits peer InsertTx and RPC CheckTx via RunTx under the gate.
type admitter struct {
	exec  *txExec
	trace bool
	// preVerify runs cheap checks on the decoded tx before the gate; nil skips.
	preVerify func(sdk.Tx) error
	// rpcTxLimit caps RPC admissions only; gossip is bounded by p2p concurrency.
	rpcTxLimit inflightLimit
}

// acquireRPCSlot returns ok=false, counting a shed, when no slot is free.
func (a *admitter) acquireRPCSlot() (release func(), ok bool) {
	if !a.rpcTxLimit.tryAcquire() {
		telemetry.IncrCounter(1, "cronos", "mempool", "rpc", "shed")
		return nil, false
	}
	return a.rpcTxLimit.release, true
}

// admit prepares the tx before the gate, so bad txs never take it.
// A full pool or rpcTxLimit maps to CodeTypeRetry.
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

	tx, canonical, err := a.prepare(txBytes)
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

// prepare decodes, pre-verifies and encodes a tx outside the admission gate, so bad txs never take it.
func (a *admitter) prepare(raw []byte) (tx sdk.Tx, canonical []byte, err error) {
	if tx, err = a.decode(raw); err != nil {
		return nil, nil, err
	}
	if a.preVerify != nil {
		if err = a.preVerify(tx); err != nil {
			return nil, nil, err
		}
	}
	return tx, a.encode(tx, raw), nil
}

// decode returns a nil tx without a decoder; BaseApp.RunTx then decodes txBytes itself.
func (a *admitter) decode(raw []byte) (sdk.Tx, error) {
	if a.exec.decoder == nil {
		return nil, nil
	}
	tx, err := a.exec.decoder(raw)
	if err != nil {
		return nil, sdkerrors.ErrTxDecode.Wrap(err.Error())
	}
	return tx, nil
}

// Encoding before RunTx keeps a tx whose encoding panics out of ante and the pool; nil without a cache, raw bytes on encode error.
func (a *admitter) encode(tx sdk.Tx, raw []byte) []byte {
	if a.exec.encCache == nil || tx == nil {
		return nil
	}
	if bz, err := a.exec.txEncoder(tx); err == nil {
		return bz
	}
	return raw
}

func (a *admitter) insertTxHandler() sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		code, _, _ := a.admit(req.Tx, fromGossip)
		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

// checkTxHandler runs RPC CheckTx through the runTx closure baseapp passes in,
// which already maps req.Type to the exec mode.
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

		tx, canonical, err := a.prepare(req.Tx)
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

		return &abci.ResponseCheckTx{
			GasWanted: int64(gasInfo.GasWanted),
			GasUsed:   int64(gasInfo.GasUsed),
			Log:       result.Log,
			Data:      result.Data,
			Events:    result.Events,
		}, nil
	}
}
