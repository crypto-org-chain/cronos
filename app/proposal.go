package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"filippo.io/age"
	abci "github.com/cometbft/cometbft/abci/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/core/address"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

type BlockList struct {
	Addresses []string `mapstructure:"addresses"`
}

var _ baseapp.TxSelector = &ExtTxSelector{}

// ExtTxSelector is a self-contained baseapp.TxSelector: blocklist + baseFee gate,
// then DefaultTxSelector-style byte/gas accounting using ProtoSizeForTx (no per-tx
// alloc) and overflow-safe gas math.
type ExtTxSelector struct {
	validateTx  func(sdk.Tx, []byte) error
	proposalFee func(sdk.Context) (*big.Int, string) // baseFee gate source; nil disables

	selectedTxs [][]byte
	totalBytes  int64
	totalGas    uint64

	// baseFee is constant within a proposal; cached on first Select, reset by Clear.
	feeReady bool
	baseFee  *big.Int
	evmDenom string
}

func NewExtTxSelector(validateTx func(sdk.Tx, []byte) error, proposalFee func(sdk.Context) (*big.Int, string)) *ExtTxSelector {
	return &ExtTxSelector{validateTx: validateTx, proposalFee: proposalFee}
}

func (ts *ExtTxSelector) SelectedTxs(_ context.Context) [][]byte {
	return ts.selectedTxs
}

func (ts *ExtTxSelector) Clear() {
	ts.selectedTxs = nil
	ts.totalBytes = 0
	ts.totalGas = 0
	ts.feeReady = false
	ts.baseFee = nil
	ts.evmDenom = ""
}

func (ts *ExtTxSelector) SelectTxForProposal(goCtx context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte) bool {
	// returned bool = stop iterating; true once the block is full.
	full := func() bool {
		return uint64(ts.totalBytes) >= maxTxBytes || (maxBlockGas > 0 && ts.totalGas >= maxBlockGas)
	}

	if err := ts.validateTx(memTx, txBz); err != nil {
		return full() // blocked/invalid: skip, keep scanning
	}

	txSize := cronosmempool.ProtoSizeForTx(txBz)
	if uint64(ts.totalBytes)+uint64(txSize) > maxTxBytes {
		return full() // too large: try smaller txs unless already full
	}

	feeTx, isFeeTx := memTx.(sdk.FeeTx)

	// baseFee gate: drop txs whose feeCap fell below a risen baseFee (idle senders
	// InsertTx ante + recheck miss); else they'd fail ante at FinalizeBlock. Skip,
	// don't evict: feeCap may clear next block.
	if isFeeTx {
		if bf, denom := ts.gateBaseFee(goCtx); bf != nil && denom != "" {
			if gas := feeTx.GetGas(); gas > 0 {
				feeCap := feeTx.GetFee().AmountOf(denom).Quo(sdkmath.NewIntFromUint64(gas))
				if feeCap.LT(sdkmath.NewIntFromBigInt(bf)) {
					return full()
				}
			}
		}
	}

	if maxBlockGas > 0 && isFeeTx {
		gasWanted := feeTx.GetGas()
		// Overflow-safe: totalGas <= maxBlockGas by induction.
		if gasWanted > maxBlockGas-ts.totalGas {
			return full()
		}
		ts.totalGas += gasWanted
	}

	ts.selectedTxs = append(ts.selectedTxs, txBz)
	ts.totalBytes += txSize
	return full()
}

// gateBaseFee reads (baseFee, evmDenom) once per proposal; nil baseFee disables the gate.
func (ts *ExtTxSelector) gateBaseFee(goCtx context.Context) (*big.Int, string) {
	if !ts.feeReady {
		ts.feeReady = true
		if ts.proposalFee != nil {
			ts.baseFee, ts.evmDenom = ts.proposalFee(sdk.UnwrapSDKContext(goCtx))
		}
	}
	return ts.baseFee, ts.evmDenom
}

// fastNoOpPrepareProposal returns a PrepareProposal handler for the nil/NoOp
// mempool. It skips the per-tx TxDecode that v0.54's NewDefaultProposalHandler
// does (which aborts the whole proposal on one malformed tx), applies the cronos
// validateTx blocklist, and respects req.MaxTxBytes. When consensus MaxGas > 0
// it also caps total selected gas, so proposals stay within the gas budget.
//
// With MaxGas > 0 each tx must be decoded to read its gas; the fast path then
// only buys fault tolerance (a decode failure skips that tx instead of aborting).
// Real (priority) mempools delegate to the upstream default to keep ordering.
func fastNoOpPrepareProposal(
	mp mempool.Mempool,
	defaultHandler sdk.PrepareProposalHandler,
	txDecoder sdk.TxDecoder,
	validateTx func(sdk.Tx, []byte) error,
) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		_, isNoOp := mp.(mempool.NoOpMempool)
		if mp != nil && !isNoOp {
			return defaultHandler(ctx, req)
		}

		maxTxBytes := req.MaxTxBytes
		if maxTxBytes <= 0 {
			return &abci.ResponsePrepareProposal{}, nil
		}

		var maxBlockGas uint64
		if b := ctx.ConsensusParams().Block; b != nil && b.MaxGas > 0 {
			maxBlockGas = uint64(b.MaxGas)
		}

		var (
			selected   [][]byte
			totalBytes int64
			totalGas   uint64
		)
		for _, txBz := range req.Txs {
			// Match baseapp.DefaultTxSelector's accounting so the block
			// respects cometbft's MaxBytes wire limit, not just raw payload sum.
			txSize := cronosmempool.ProtoSizeForTx(txBz)
			if totalBytes+txSize > maxTxBytes {
				continue // skip an over-budget tx; a smaller later one may still fit
			}

			// Decode only when gas accounting needs it; otherwise validateTx
			// decodes lazily if its blocklist demands it.
			var tx sdk.Tx
			if maxBlockGas > 0 {
				var err error
				if tx, err = txDecoder(txBz); err != nil {
					continue
				}
			}

			if err := validateTx(tx, txBz); err != nil {
				continue
			}

			if maxBlockGas > 0 {
				if feeTx, ok := tx.(sdk.FeeTx); ok {
					gasWanted := feeTx.GetGas()
					// Overflow-safe: totalGas <= maxBlockGas by induction, so
					// maxBlockGas-totalGas can't underflow even for attacker-set gas.
					if gasWanted > maxBlockGas-totalGas {
						continue // skip; a smaller-gas later tx may still fit (matches ExtTxSelector)
					}
					totalGas += gasWanted
				}
				// Non-FeeTx: no gas accounting. All cronos txs are FeeTx, so
				// unreachable in practice but safe (can't inflate totalGas).
			}

			selected = append(selected, txBz)
			totalBytes += txSize
		}
		return &abci.ResponsePrepareProposal{Txs: selected}, nil
	}
}

var _ baseapp.ProposalTxVerifier = &NoCheckProposalTxVerifier{}

// NoCheckProposalTxVerifier replaces BaseApp.PrepareProposalVerifyTx (which runs
// a full ante) with a cache lookup: ante already ran at admission and recheck
// evicts stale txs, so PrepareProposal only needs canonical bytes. Cache hit ->
// cached bytes; miss -> encode. Mirrors cosmos/evm. The skipped baseFee check for
// idle senders is reapplied by ExtTxSelector's gate.
type NoCheckProposalTxVerifier struct {
	*baseapp.BaseApp
	encCache *cronosmempool.EncoderCache
}

func NewNoCheckProposalTxVerifier(app *baseapp.BaseApp, encCache *cronosmempool.EncoderCache) *NoCheckProposalTxVerifier {
	return &NoCheckProposalTxVerifier{BaseApp: app, encCache: encCache}
}

func (txv *NoCheckProposalTxVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	bz, hit, err := cronosmempool.EncodeTx(txv.encCache, txv.TxEncode, tx)
	result := "miss"
	if hit {
		result = "hit"
	}
	telemetry.IncrCounter(1, "cronos", "mempool", "prepare", "encode_cache", result) //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
	return bz, err
}

type ProposalHandler struct {
	TxDecoder sdk.TxDecoder
	// Identity is nil if it's not a validator node
	Identity      age.Identity
	blocklist     map[string]struct{}
	lastBlockList []byte
	addressCodec  address.Codec
}

func NewProposalHandler(txDecoder sdk.TxDecoder, identity age.Identity, addressCodec address.Codec) *ProposalHandler {
	return &ProposalHandler{
		TxDecoder:    txDecoder,
		Identity:     identity,
		blocklist:    make(map[string]struct{}),
		addressCodec: addressCodec,
	}
}

// SetBlockList don't fail if the identity is not set or the block list is empty.
func (h *ProposalHandler) SetBlockList(blob []byte) error {
	if h.Identity == nil {
		return nil
	}

	if bytes.Equal(h.lastBlockList, blob) {
		return nil
	}
	h.lastBlockList = make([]byte, len(blob))
	copy(h.lastBlockList, blob)

	if len(blob) == 0 {
		h.blocklist = make(map[string]struct{})
		return nil
	}

	reader, err := age.Decrypt(bytes.NewBuffer(blob), h.Identity)
	if err != nil {
		return err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	var blocklist BlockList
	if err := json.Unmarshal(data, &blocklist); err != nil {
		return err
	}

	// convert to map
	m := make(map[string]struct{}, len(blocklist.Addresses))
	for _, s := range blocklist.Addresses {
		addr, err := h.addressCodec.StringToBytes(s)
		if err != nil {
			return fmt.Errorf("invalid bech32 address: %s, err: %w", s, err)
		}
		if IsUnblockable(addr) {
			continue
		}
		encoded, err := h.addressCodec.BytesToString(addr)
		if err != nil {
			return fmt.Errorf("invalid bech32 address: %s, err: %w", s, err)
		}
		m[encoded] = struct{}{}
	}

	h.blocklist = m
	return nil
}

func (h *ProposalHandler) ValidateTransaction(tx sdk.Tx, txBz []byte) error {
	if len(h.blocklist) == 0 {
		// fast path, accept all txs
		return nil
	}

	var err error
	if tx == nil {
		tx, err = h.TxDecoder(txBz)
		if err != nil {
			return err
		}
	}

	sigTx, ok := tx.(signing.SigVerifiableTx)
	if !ok {
		return fmt.Errorf("tx of type %T does not implement SigVerifiableTx", tx)
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return err
	}
	for _, signer := range signers {
		encoded, err := h.addressCodec.BytesToString(signer)
		if err != nil {
			return fmt.Errorf("invalid bech32 address: %s, err: %w", signer, err)
		}
		if _, ok := h.blocklist[encoded]; ok {
			return fmt.Errorf("signer is blocked: %s", encoded)
		}
	}

	for _, msg := range tx.GetMsgs() {
		msgEthTx, ok := msg.(*evmtypes.MsgEthereumTx)
		if ok {
			ethTx := msgEthTx.AsTransaction()
			// check the destination address
			if ethTx.To() != nil {
				encoded, err := h.addressCodec.BytesToString(ethTx.To().Bytes())
				if err != nil {
					return fmt.Errorf("invalid bech32 address: %s, err: %w", ethTx.To(), err)
				}
				if _, ok := h.blocklist[encoded]; ok {
					return fmt.Errorf("destination address is blocked: %s", encoded)
				}
			}
			// check EIP-7702 authorisation list
			if ethTx.SetCodeAuthorizations() != nil {
				for _, auth := range ethTx.SetCodeAuthorizations() {
					addr, err := auth.Authority()
					if err == nil {
						encoded, err := h.addressCodec.BytesToString(addr.Bytes())
						if err != nil {
							return fmt.Errorf("invalid bech32 address: %s, err: %w", addr, err)
						}
						if _, ok := h.blocklist[encoded]; ok {
							return fmt.Errorf("signer is blocked: %s", encoded)
						}
					}
					// check the target address
					encoded, err := h.addressCodec.BytesToString(auth.Address.Bytes())
					if err != nil {
						return fmt.Errorf("invalid bech32 address: %s, err: %w", auth.Address, err)
					}
					if _, ok := h.blocklist[encoded]; ok {
						return fmt.Errorf("authorisation address is blocked: %s", encoded)
					}
				}
			}
		}
	}

	return nil
}

func (h *ProposalHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		if len(h.blocklist) == 0 {
			// fast path, accept all txs
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		}

		for _, txBz := range req.Txs {
			if err := h.ValidateTransaction(nil, txBz); err != nil {
				return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
			}
		}

		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	}
}

// noneIdentity is a dummy identity which postpone the failure to the decryption time
type noneIdentity struct{}

var _ age.Identity = noneIdentity{}

func (noneIdentity) Unwrap([]*age.Stanza) ([]byte, error) {
	return nil, age.ErrIncorrectIdentity
}
