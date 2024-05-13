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

type ProposalHandler struct {
	TxDecoder  sdk.TxDecoder
	Identity   age.Identity
	TxSelector baseapp.TxSelector

	blocklist     map[string]struct{}
	lastBlockList []byte
}

func NewProposalHandler(txDecoder sdk.TxDecoder, identity age.Identity) *ProposalHandler {
	return &ProposalHandler{
		TxDecoder:  txDecoder,
		Identity:   identity,
		blocklist:  make(map[string]struct{}),
		TxSelector: baseapp.NewDefaultTxSelector(),
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

func (h *ProposalHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req abci.RequestPrepareProposal) abci.ResponsePrepareProposal {
		var maxBlockGas uint64
		if b := ctx.ConsensusParams().Block; b != nil {
			maxBlockGas = uint64(b.MaxGas)
		}

		defer h.TxSelector.Clear()

		for _, txBz := range req.Txs {
			memTx, err := h.TxDecoder(txBz)
			if err != nil {
				continue
			}

			if err := h.ValidateTransaction(memTx); err != nil {
				continue
			}

			stop := h.TxSelector.SelectTxForProposal(uint64(req.MaxTxBytes), maxBlockGas, memTx, txBz)
			if stop {
				break
			}
		}

		return abci.ResponsePrepareProposal{Txs: h.TxSelector.SelectedTxs()}
	}
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
