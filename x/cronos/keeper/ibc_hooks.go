package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/modules/apps/transfer/types"
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
		err := k.ConvertVouchersToEvmCoins(ctx, receiver, sdk.NewCoins(token))
		if err != nil {
			k.Logger(ctx).Error(
				fmt.Sprintf("Failed to convert vouchers to evm tokens for receiver %s, coins %s. Receive error %s",
					receiver, token.String(), err))
		}
	}
}

func (k Keeper) AfterRefundTransfer(
	ctx sdk.Context,
	sourcePort, sourceChannel string,
	token sdk.Coin,
	sender string,
	isSource bool) {
}
