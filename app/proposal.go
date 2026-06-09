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

// ExtTxSelector extends a baseapp.TxSelector with extra tx validation method
type ExtTxSelector struct {
	baseapp.TxSelector
	TxDecoder  sdk.TxDecoder
	ValidateTx func(sdk.Tx, []byte) error
}

func NewExtTxSelector(parent baseapp.TxSelector, txDecoder sdk.TxDecoder, validateTx func(sdk.Tx, []byte) error) *ExtTxSelector {
	return &ExtTxSelector{
		TxSelector: parent,
		TxDecoder:  txDecoder,
		ValidateTx: validateTx,
	}
}

func (ts *ExtTxSelector) SelectTxForProposal(ctx context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte) bool {
	if err := ts.ValidateTx(memTx, txBz); err != nil {
		return false
	}

	// Pass memTx so the parent selector can read tx gas wanted and stop at maxBlockGas.
	return ts.TxSelector.SelectTxForProposal(ctx, maxTxBytes, maxBlockGas, memTx, txBz)
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
				break
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
						break
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

// fastPrepareProposalAppMempool returns a PrepareProposal handler for
// mempool.type=app: iterates the priority mempool directly, reads raw bytes from
// encCache (populated by InsertTxHandler), respects MaxTxBytes / consensus MaxGas.
//
// Skips baseapp.PrepareProposalVerifyTx (full ante re-run): InsertTxHandler ran
// ante at admission and RecheckTxs evicts stale nonce/balance after each block.
// The gap both miss — a risen baseFee for an idle (never-rechecked) sender — is
// closed by the inline feeCap<baseFee gate below, far cheaper than a full ante.
// Tradeoff: ante decorators that differ between Check and PrepareProposal modes
// aren't re-evaluated, so such a tx is caught only at FinalizeBlock.
//
// proposalFee returns (baseFee, evmDenom); nil baseFee disables the gate.
// encCache MUST be non-nil. txEncoder is the fallback for uncached txs (rare).
func fastPrepareProposalAppMempool(
	mp mempool.Mempool,
	encCache *cronosmempool.EncoderCache,
	txEncoder sdk.TxEncoder,
	validateTx func(sdk.Tx, []byte) error,
	proposalFee func(sdk.Context) (baseFee *big.Int, evmDenom string),
) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		maxTxBytes := req.MaxTxBytes
		if maxTxBytes <= 0 {
			return &abci.ResponsePrepareProposal{}, nil
		}

		var maxBlockGas uint64
		if b := ctx.ConsensusParams().Block; b != nil && b.MaxGas > 0 {
			maxBlockGas = uint64(b.MaxGas)
		}

		// baseFee is constant within a block; read once. nil => gate disabled.
		var (
			baseFee  *big.Int
			evmDenom string
		)
		if proposalFee != nil {
			baseFee, evmDenom = proposalFee(ctx)
		}

		var (
			selected   [][]byte
			totalBytes int64
			totalGas   uint64
			cacheHits  float32
			cacheMiss  float32
		)
		snapshot := cronosmempool.PoolSnapshot(ctx, mp)

		for _, memTx := range snapshot {
			bz, hit, err := cronosmempool.EncodeTx(encCache, txEncoder, memTx)
			if hit {
				cacheHits++
			} else {
				cacheMiss++
			}
			if err != nil {
				continue
			}

			// Match baseapp.DefaultTxSelector's accounting so the block
			// respects cometbft's MaxBytes wire limit, not just raw payload sum.
			txSize := cronosmempool.ProtoSizeForTx(bz)
			if totalBytes+txSize > maxTxBytes {
				break
			}

			if err := validateTx(memTx, bz); err != nil {
				continue
			}

			feeTx, isFeeTx := memTx.(sdk.FeeTx)

			// baseFee gate: drop txs whose feeCap fell below a risen baseFee
			// (the case InsertTx ante + RecheckTxs miss for idle senders), else
			// they'd fail ante at FinalizeBlock with ErrInsufficientFee. Mirrors the
			// fatal check in ethermint NewDynamicFeeChecker (feeCap < baseFee). continue,
			// not break: snapshot priority order isn't strictly feeCap order.
			if isFeeTx && baseFee != nil && evmDenom != "" {
				if gas := feeTx.GetGas(); gas > 0 {
					feeCap := feeTx.GetFee().AmountOf(evmDenom).Quo(sdkmath.NewIntFromUint64(gas))
					if feeCap.LT(sdkmath.NewIntFromBigInt(baseFee)) {
						continue
					}
				}
			}

			if maxBlockGas > 0 && isFeeTx {
				gasWanted := feeTx.GetGas()
				// Overflow-safe: see fastNoOpPrepareProposal.
				if gasWanted > maxBlockGas-totalGas {
					break
				}
				totalGas += gasWanted
			}

			selected = append(selected, bz)
			totalBytes += txSize
		}
		// Emit cache hit/miss once per proposal (not per tx) so the proto.Marshal
		// fallback rate is observable when pool depth exceeds the encoder-cache
		// size. No-op unless telemetry is enabled.
		if cacheHits > 0 {
			telemetry.IncrCounter(cacheHits, "cronos", "mempool", "prepare", "encode_cache", "hit") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if cacheMiss > 0 {
			telemetry.IncrCounter(cacheMiss, "cronos", "mempool", "prepare", "encode_cache", "miss") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		return &abci.ResponsePrepareProposal{Txs: selected}, nil
	}
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
