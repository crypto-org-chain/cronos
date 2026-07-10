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

// ExtTxSelector is a custom tx selector for cronos
type ExtTxSelector struct {
	validateTx       func(sdk.Tx, []byte) error
	baseFeeRetriever func(sdk.Context) (*big.Int, string) // baseFee gate source; nil disables

	selectedTxs [][]byte
	totalBytes  int64
	totalGas    uint64

	// baseFee/evmDenom are constant within a proposal
	baseFee  *big.Int
	evmDenom string

	// gateSkipped accumulates raw bytes of txs rejected by the baseFee gate this
	// round. Drained after each proposal so stranded senders are staged for recheck.
	gateSkipped [][]byte
}

func NewExtTxSelector(validateTx func(sdk.Tx, []byte) error, baseFeeRetriever func(sdk.Context) (*big.Int, string)) *ExtTxSelector {
	return &ExtTxSelector{validateTx: validateTx, baseFeeRetriever: baseFeeRetriever}
}

func (ts *ExtTxSelector) SelectedTxs(_ context.Context) [][]byte {
	txs := make([][]byte, len(ts.selectedTxs))
	copy(txs, ts.selectedTxs)
	return txs
}

func (ts *ExtTxSelector) Clear() {
	ts.selectedTxs = nil
	ts.totalBytes = 0
	ts.totalGas = 0
	ts.baseFee = nil
	ts.evmDenom = ""
}

// DrainGateSkipped returns and clears the raw bytes of txs rejected by the
// baseFee gate this proposal round.
func (ts *ExtTxSelector) DrainGateSkipped() [][]byte {
	out := ts.gateSkipped
	ts.gateSkipped = nil
	return out
}

func (ts *ExtTxSelector) SelectTxForProposal(goCtx context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte) bool {
	// returned bool = stop iterating; true once the block is full.
	isFull := func() bool {
		return uint64(ts.totalBytes) >= maxTxBytes || (maxBlockGas > 0 && ts.totalGas >= maxBlockGas)
	}

	if err := ts.validateTx(memTx, txBz); err != nil {
		return isFull() // blocked/invalid: skip, keep scanning
	}

	txSize := cronosmempool.ProtoSizeForTx(txBz)
	if uint64(ts.totalBytes)+uint64(txSize) > maxTxBytes {
		return isFull() // too large: try smaller txs unless already full
	}

	feeTx, isFeeTx := memTx.(sdk.FeeTx)

	// baseFee gate: drop txs whose feeCap fell below a risen baseFee
	if isFeeTx {
		if bf, denom := ts.gateBaseFee(goCtx); bf != nil && denom != "" {
			if gas := feeTx.GetGas(); gas > 0 {
				feeCap := feeTx.GetFee().AmountOf(denom).Quo(sdkmath.NewIntFromUint64(gas))
				if feeCap.LT(sdkmath.NewIntFromBigInt(bf)) {
					ts.gateSkipped = append(ts.gateSkipped, txBz)
					telemetry.IncrCounter(1, "cronos", "mempool", "proposal", "gate", "skipped")
					return isFull()
				}
			}
		}
	}

	if maxBlockGas > 0 && isFeeTx {
		gasWanted := feeTx.GetGas()
		// Overflow-safe: totalGas <= maxBlockGas by induction.
		if gasWanted > maxBlockGas-ts.totalGas {
			return isFull()
		}
		ts.totalGas += gasWanted
	}

	ts.selectedTxs = append(ts.selectedTxs, txBz)
	ts.totalBytes += txSize
	return isFull()
}

// gateBaseFee reads (baseFee, evmDenom) once per proposal; nil baseFee disables the gate.
func (ts *ExtTxSelector) gateBaseFee(goCtx context.Context) (*big.Int, string) {
	if ts.baseFee == nil && ts.baseFeeRetriever != nil {
		ts.baseFee, ts.evmDenom = ts.baseFeeRetriever(sdk.UnwrapSDKContext(goCtx))
	}
	return ts.baseFee, ts.evmDenom
}

// MempoolProposalHandler defines a custom PrepareProposal for cronos.
type MempoolProposalHandler struct {
	mempoolManager *cronosmempool.Manager
	extSel         *ExtTxSelector
	inner          sdk.PrepareProposalHandler
}

// NewMempoolProposalHandler wraps h with the blocklist + baseFee gate ExtTxSelector.
func NewMempoolProposalHandler(h *baseapp.DefaultProposalHandler, validateTx func(sdk.Tx, []byte) error, baseFeeRetriever func(sdk.Context) (*big.Int, string), signerExtractor mempool.SignerExtractionAdapter) *MempoolProposalHandler {
	extSel := NewExtTxSelector(validateTx, baseFeeRetriever)
	h.SetTxSelector(extSel)
	if signerExtractor != nil {
		h.SetSignerExtractionAdapter(signerExtractor)
	}
	return &MempoolProposalHandler{extSel: extSel, inner: h.PrepareProposalHandler()}
}

func (h *MempoolProposalHandler) SetMempoolManager(m *cronosmempool.Manager) {
	h.mempoolManager = m
}

func (h *MempoolProposalHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		if h.mempoolManager != nil {
			// On timeout, propose from the current pool instead of an empty block
			if h.mempoolManager.WaitForRecheckTimedOut(ctx, recheckWaitTimeout) {
				telemetry.IncrCounter(1, "cronos", "mempool", "recheck", "proposal_timeout")
			}
		}
		resp, err := h.inner(ctx, req)
		skipped := h.extSel.DrainGateSkipped() // drain every round, even on error, so it never goes stale
		if err == nil && h.mempoolManager != nil && len(skipped) > 0 {
			h.mempoolManager.StageSkippedSenders(skipped)
		}
		return resp, err
	}
}

var _ baseapp.ProposalTxVerifier = &CacheProposalTxVerifier{}

// CacheProposalTxVerifier is used to cache encoded transactions to avoid cpu overhead during proposal.
type CacheProposalTxVerifier struct {
	baseapp.ProposalTxVerifier
	encCache *cronosmempool.EncoderCache
}

func NewCacheProposalTxVerifier(verifier baseapp.ProposalTxVerifier, encCache *cronosmempool.EncoderCache) *CacheProposalTxVerifier {
	return &CacheProposalTxVerifier{ProposalTxVerifier: verifier, encCache: encCache}
}

func (txv *CacheProposalTxVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	// Wrapped in a closure so a cache hit never forms a bound method value off
	// the embedded interface; that panics if it's nil (unlike a nil pointer embed).
	bz, hit, err := cronosmempool.EncodeTx(txv.encCache, func(t sdk.Tx) ([]byte, error) { return txv.TxEncode(t) }, tx)
	result := "miss"
	if hit {
		result = "hit"
	}
	telemetry.IncrCounter(1, "cronos", "mempool", "prepare", "encode_cache", result)
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
