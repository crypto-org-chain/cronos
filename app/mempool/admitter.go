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
	// encodeMu serializes the pre-RunTx encode, which fills empty body/auth-info
	// bytes in a shared decoded tx. Pooled txs passed ante, so recheck's encode
	// only reads. Never held with gate.
	encodeMu sync.Mutex
	trace    bool
	// preVerify runs cheap checks before the gate; nil skips.
	preVerify func([]byte) error
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

// admit runs preVerify and decoding before the gate, so bad txs never take it.
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

// decodeAndEncode encodes before RunTx so a tx whose encode panics never reaches
// the pool. Returns nils without a cache (RunTx then decodes), raw bytes on an encode error.
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

		return &abci.ResponseCheckTx{
			GasWanted: int64(gasInfo.GasWanted),
			GasUsed:   int64(gasInfo.GasUsed),
			Log:       result.Log,
			Data:      result.Data,
			Events:    result.Events,
		}, nil
	}
}
