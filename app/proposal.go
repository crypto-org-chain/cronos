package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"filippo.io/age"

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
	Parent     baseapp.TxSelector
	ValidateTx func(sdk.Tx) error
}

func NewExtTxSelector(parent baseapp.TxSelector, validateTx func(sdk.Tx) error) *ExtTxSelector {
	return &ExtTxSelector{
		Parent:     parent,
		ValidateTx: validateTx,
	}
}

func (ts *ExtTxSelector) SelectTxForProposal(maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte) bool {
	if err := ts.ValidateTx(memTx); err != nil {
		return false
	}
	return ts.Parent.SelectTxForProposal(maxTxBytes, maxBlockGas, memTx, txBz)
}

func (ts *ExtTxSelector) SelectedTxs() [][]byte {
	return ts.Parent.SelectedTxs()
}

func (ts *ExtTxSelector) Clear() {
	ts.Parent.Clear()
}

type ProposalHandler struct {
	TxDecoder     sdk.TxDecoder
	Identity      age.Identity
	blocklist     map[string]struct{}
	lastBlockList []byte
}

func NewProposalHandler(txDecoder sdk.TxDecoder, identity age.Identity) *ProposalHandler {
	return &ProposalHandler{
		TxDecoder: txDecoder,
		Identity:  identity,
		blocklist: make(map[string]struct{}),
	}
}

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
		addr, err := sdk.AccAddressFromBech32(s)
		if err != nil {
			return fmt.Errorf("invalid bech32 address: %s, err: %w", s, err)
		}
		m[addr.String()] = struct{}{}
	}

	h.blocklist = m
	return nil
}

func (h *ProposalHandler) ValidateTransaction(tx sdk.Tx) error {
	sigTx, ok := tx.(signing.SigVerifiableTx)
	if !ok {
		return fmt.Errorf("tx of type %T does not implement SigVerifiableTx", tx)
	}

	for _, signer := range sigTx.GetSigners() {
		if _, ok := h.blocklist[signer.String()]; ok {
			return fmt.Errorf("signer is blocked: %s", signer.String())
		}
	}
	return nil
}

func (h *ProposalHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req abci.RequestProcessProposal) abci.ResponseProcessProposal {
		for _, txBz := range req.Txs {
			memTx, err := h.TxDecoder(txBz)
			if err != nil {
				continue
			}

			if err := h.ValidateTransaction(memTx); err != nil {
				return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
			}
		}

		return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}
	}
}
