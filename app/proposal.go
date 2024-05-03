package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"filippo.io/age"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

type BlockList struct {
	Addresses []string `mapstructure:"addresses"`
}

type ProposalHandler struct {
	TxDecoder     sdk.TxDecoder
	Identity      age.Identity
	Blocklist     map[string]struct{}
	LastBlockList []byte
}

func NewProposalHandler(txDecoder sdk.TxDecoder, identity age.Identity) *ProposalHandler {
	return &ProposalHandler{
		TxDecoder: txDecoder,
		Identity:  identity,
		Blocklist: make(map[string]struct{}),
	}
}

func (h *ProposalHandler) SetBlockList(blob []byte) error {
	if h.Identity == nil {
		return nil
	}

	if bytes.Equal(h.LastBlockList, blob) {
		return nil
	}
	h.LastBlockList = blob

	if len(blob) == 0 {
		h.Blocklist = make(map[string]struct{})
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

	h.Blocklist = m
	return nil
}

func (h *ProposalHandler) ValidateTransaction(txBz []byte) error {
	tx, err := h.TxDecoder(txBz)
	if err != nil {
		return err
	}

	sigTx, ok := tx.(signing.SigVerifiableTx)
	if !ok {
		return fmt.Errorf("tx of type %T does not implement SigVerifiableTx", tx)
	}

	for _, signer := range sigTx.GetSigners() {
		if _, ok := h.Blocklist[signer.String()]; ok {
			return fmt.Errorf("signer is blocked: %s", signer.String())
		}
	}
	return nil
}

func (h *ProposalHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req abci.RequestPrepareProposal) abci.ResponsePrepareProposal {
		txs := make([][]byte, 0, len(req.Txs))
		for _, txBz := range req.Txs {
			if err := h.ValidateTransaction(txBz); err != nil {
				continue
			}
			txs = append(txs, txBz)
		}

		return abci.ResponsePrepareProposal{Txs: txs}
	}
}

func (h *ProposalHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req abci.RequestProcessProposal) abci.ResponseProcessProposal {
		for _, txBz := range req.Txs {
			if err := h.ValidateTransaction(txBz); err != nil {
				return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
			}
		}

		return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}
	}
}
