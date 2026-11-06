package mempool

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

var _ sdkmempool.SignerExtractionAdapter = MsgSignerExtractionAdapter{}

// PubKeyPolicy sets whether MsgSignerExtractionAdapter accepts a SignerInfo without a public key.
type PubKeyPolicy int

const (
	// PubKeyIgnore accepts it: a missing key is valid once the signer's key is on-chain.
	PubKeyIgnore PubKeyPolicy = iota
	// PubKeyRequire rejects it; a malformed key is left to ante.
	PubKeyRequire
)

// MsgSignerExtractionAdapter is the SDK default adapter, except it takes each signer from the
// tx's msg signers instead of its SignerInfo public key, which the SDK adapter panics on when
// missing or malformed.
type MsgSignerExtractionAdapter struct {
	policy PubKeyPolicy
}

func NewMsgSignerExtractionAdapter(policy PubKeyPolicy) MsgSignerExtractionAdapter {
	return MsgSignerExtractionAdapter{policy: policy}
}

func (a MsgSignerExtractionAdapter) GetSigners(tx sdk.Tx) ([]sdkmempool.SignerData, error) {
	sigTx, ok := tx.(signing.SigVerifiableTx)
	if !ok {
		return nil, fmt.Errorf("tx of type %T does not implement SigVerifiableTx", tx)
	}
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return nil, err
	}
	addrs, err := sigTx.GetSigners()
	if err != nil {
		return nil, err
	}
	// Ante rejects the mismatch, so only an invalid tx gets here.
	if len(addrs) != len(sigs) {
		return nil, fmt.Errorf("tx has %d signer infos but %d signers", len(sigs), len(addrs))
	}
	signers := make([]sdkmempool.SignerData, len(sigs))
	for i, sig := range sigs {
		if a.policy == PubKeyRequire && sig.PubKey == nil {
			return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidPubKey, "signer info %d has no public key", i)
		}
		signers[i] = sdkmempool.NewSignerData(addrs[i], sig.Sequence)
	}
	return signers, nil
}
