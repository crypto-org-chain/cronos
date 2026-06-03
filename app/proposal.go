package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"filippo.io/age"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/core/address"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"

	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
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

// fastNoOpPrepareProposal returns a PrepareProposal handler that, when the
// mempool is nil or NoOp, skips the per-tx TxDecode that cosmos-sdk v0.54's
// NewDefaultProposalHandler performs (and that aborts the proposal on a single
// malformed tx). It applies the cronos `validateTx` block-list filter and
// respects req.MaxTxBytes. When the consensus param MaxGas is set, it also
// caps total selected gas — required so proposals do not exceed the
// consensus gas budget under NoOp.
//
// When MaxGas > 0 the handler must decode each tx to read its gas; the "fast
// path" then matters only for fault tolerance (a decode failure skips the
// offending tx instead of aborting). Real (priority) mempools delegate to the
// upstream default to preserve ordering semantics.
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
			nextBytes := totalBytes + int64(len(txBz))
			if nextBytes > maxTxBytes {
				break
			}

			// Decode only when we need gas accounting; otherwise let
			// validateTx lazily decode if its blocklist demands it.
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
					if totalGas+gasWanted > maxBlockGas {
						break
					}
					totalGas += gasWanted
				}
				// Non-FeeTx: admitted without gas accounting. All cronos txs implement
				// sdk.FeeTx (cosmos auth.Tx does), so this branch is unreachable in
				// practice but safe — a non-FeeTx cannot inflate totalGas past the cap.
			}

			selected = append(selected, txBz)
			totalBytes = nextBytes
		}
		return &abci.ResponsePrepareProposal{Txs: selected}, nil
	}
}

// fastPrepareProposalAppMempool returns a PrepareProposal handler for the
// mempool.type=app path. Iterates the SDK priority mempool directly, reads
// raw tx bytes from encCache (populated by InsertTxHandler), applies the
// blocklist filter, and respects MaxTxBytes / consensus MaxGas.
//
// Skips baseapp.PrepareProposalVerifyTx (which re-encodes + re-runs the full
// AnteHandler per tx). InsertTxHandler already ran ante via RunTx(ExecModeCheck)
// at admission, so the SDK mempool only contains txs that have passed ante for
// the current state. The PrepareProposalVerifyTx defense is redundant for
// cronos and dominates step_propose latency at high throughput.
//
// encCache MUST be non-nil; when it is nil the caller should wire the slower
// `defaultHandler` path which still performs PrepareProposalVerifyTx. txEncoder
// is the fallback for txs whose decoded pointer is not registered in the cache
// (eviction race after Reap; expected to be rare).
func fastPrepareProposalAppMempool(
	mp mempool.Mempool,
	encCache *cronosmempool.EncoderCache,
	txEncoder sdk.TxEncoder,
	validateTx func(sdk.Tx, []byte) error,
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

		var (
			selected   [][]byte
			totalBytes int64
			totalGas   uint64
			stop       bool
		)
		mempool.SelectBy(ctx, mp, nil, func(memTx sdk.Tx) bool {
			bz, ok := encCache.Bytes(memTx)
			if !ok {
				var err error
				bz, err = txEncoder(memTx)
				if err != nil {
					// skip and continue iteration
					return true
				}
			}

			// Use the same accounting baseapp.DefaultTxSelector uses so the
			// resulting block respects cometbft's MaxBytes wire limit, not
			// just the raw payload sum.
			txSize := cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{bz})
			if totalBytes+txSize > maxTxBytes {
				stop = true
				return false
			}

			if err := validateTx(memTx, bz); err != nil {
				return true
			}

			if maxBlockGas > 0 {
				if feeTx, ok := memTx.(sdk.FeeTx); ok {
					gasWanted := feeTx.GetGas()
					if totalGas+gasWanted > maxBlockGas {
						stop = true
						return false
					}
					totalGas += gasWanted
				}
			}

			selected = append(selected, bz)
			totalBytes += txSize
			return true
		})
		_ = stop
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
						if _, ok := h.blocklist[sdk.AccAddress(addr.Bytes()).String()]; ok {
							return fmt.Errorf("signer is blocked: %s", addr.String())
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
