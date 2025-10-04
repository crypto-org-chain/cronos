package app

import (
	"fmt"

	"github.com/crypto-org-chain/cronos/x/cronos/types"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// BlockAddressesDecorator block addresses from sending transactions
type BlockAddressesDecorator struct {
	blockedMap map[string]struct{}
	getParams  func(ctx sdk.Context) types.Params
}

func NewBlockAddressesDecorator(
	blacklist map[string]struct{},
	getParams func(ctx sdk.Context) types.Params,
) BlockAddressesDecorator {
	return BlockAddressesDecorator{
		blockedMap: blacklist,
		getParams:  getParams,
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
		admin := bad.getParams(ctx).CronosAdmin
		for _, msg := range tx.GetMsgs() {
			if blocklistMsg, ok := msg.(*types.MsgStoreBlockList); ok {
				if admin != blocklistMsg.From {
					return ctx, errors.Wrap(sdkerrors.ErrUnauthorized, "msg sender is not authorized")
				}
			}
		}
	}
	return next(ctx, tx, simulate)
}
