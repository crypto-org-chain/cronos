package app

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// BlockAddressesDecorator block addresses from sending transactions
type BlockAddressesDecorator struct {
	blockedMap map[string]struct{}
}

func NewBlockAddressesDecorator(blacklist map[string]struct{}) BlockAddressesDecorator {
	return BlockAddressesDecorator{
		blockedMap: blacklist,
	}
}

func (bad BlockAddressesDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	if ctx.IsCheckTx() {
		if sigTx, ok := tx.(signing.SigVerifiableTx); ok {
			signers, err := sigTx.GetSigners()
			if err != nil {
				return ctx, err
			}
			for _, signer := range signers {
				if _, ok := bad.blockedMap[sdk.AccAddress(signer).String()]; ok {
					return ctx, fmt.Errorf("signer is blocked: %s", sdk.AccAddress(signer).String())
				}
			}
		}
	}
	return next(ctx, tx, simulate)
}
