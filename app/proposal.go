package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"filippo.io/age"

	"cosmossdk.io/core/address"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	ValidateTx func(sdk.Tx) error
}

func NewExtTxSelector(parent baseapp.TxSelector, txDecoder sdk.TxDecoder, validateTx func(sdk.Tx) error) *ExtTxSelector {
	return &ExtTxSelector{
		TxSelector: parent,
		TxDecoder:  txDecoder,
		ValidateTx: validateTx,
	}
}

func (ts *ExtTxSelector) SelectTxForProposal(ctx context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte, gasWanted uint64) bool {
	var err error
	if memTx == nil {
		memTx, err = ts.TxDecoder(txBz)
		if err != nil {
			return false
		}
	}

	if err := ts.ValidateTx(memTx); err != nil {
		return false
	}

	// don't pass `memTx` to parent selector so it don't check tx gas wanted against block gas limit,
	// it conflicts with the max-tx-gas-wanted logic.
	return ts.TxSelector.SelectTxForProposal(ctx, maxTxBytes, maxBlockGas, nil, txBz, gasWanted)
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

func (h *ProposalHandler) ValidateTransaction(tx sdk.Tx) error {
	if len(h.blocklist) == 0 {
		// fast path, accept all txs
		return nil
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
	return nil
}

func (h *ProposalHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		if len(h.blocklist) == 0 {
			// fast path, accept all txs
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		}

		for _, txBz := range req.Txs {
			memTx, err := h.TxDecoder(txBz)
			if err != nil {
				return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
			}

			if err := h.ValidateTransaction(memTx); err != nil {
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
