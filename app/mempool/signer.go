package mempool

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

var _ sdkmempool.SignerExtractionAdapter = MsgSignerExtractionAdapter{}

// MsgSignerExtractionAdapter is the SDK default adapter, except it takes each signer from the
// tx's msg signers instead of its SignerInfo public key. A SignerInfo may omit the key (valid
// once it is on-chain) or, in an unvalidated block tx, carry a malformed one; both make the
// SDK adapter panic.
type MsgSignerExtractionAdapter struct{}

func NewMsgSignerExtractionAdapter() MsgSignerExtractionAdapter {
	return MsgSignerExtractionAdapter{}
}

func (MsgSignerExtractionAdapter) GetSigners(tx sdk.Tx) ([]sdkmempool.SignerData, error) {
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
		signers[i] = sdkmempool.NewSignerData(addrs[i], sig.Sequence)
	}
	return signers, nil
}
