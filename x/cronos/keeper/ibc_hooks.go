package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
)

var _ types.TransferHooks = Keeper{}

func (k Keeper) AfterSendTransfer(
	ctx sdk.Context,
	sourcePort, sourceChannel string,
	token sdk.Coin,
	sender sdk.AccAddress,
	receiver string,
	isSource bool) {
}

func (k Keeper) AfterRecvTransfer(
	ctx sdk.Context,
	destPort, destChannel string,
	token sdk.Coin,
	receiver string,
	isSource bool) {
	// Only after minting vouchers
	if !isSource {
		k.OnRecvVouchers(ctx, sdk.NewCoins(token), receiver)
	}
}

func (k Keeper) AfterRefundTransfer(
	ctx sdk.Context,
	sourcePort, sourceChannel string,
	token sdk.Coin,
	sender string,
	isSource bool) {
	// Only after minting vouchers
	if !isSource {
		k.OnRecvVouchers(ctx, sdk.NewCoins(token), sender)
	}
}

func (k Keeper) OnRecvVouchers(
	ctx sdk.Context,
	tokens sdk.Coins,
	receiver string) {
	cacheCtx, commit := ctx.CacheContext()
	err := k.ConvertVouchersToEvmCoins(cacheCtx, receiver, tokens)
	if err == nil {
		commit()
		ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	} else {
		k.Logger(ctx).Error(
			fmt.Sprintf("Failed to convert vouchers to evm tokens for receiver %s, coins %s. Receive error %s",
				receiver, tokens.String(), err))
	}
}
