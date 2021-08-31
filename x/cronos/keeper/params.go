package keeper

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	transferTypes "github.com/cosmos/ibc-go/modules/apps/transfer/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	evmTypes "github.com/tharsis/ethermint/x/evm/types"
)

// GetParams returns the total set of cronos parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return params
}

// SetParams sets the total set of cronos parameters.
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramSpace.SetParamSet(ctx, &params)
}

// IsConvertEnabledCoins checks the coins provide and returns an ErrConvertDisabled if
// any of the coins are not configured for converting.  Returns nil if converting is enabled
// for all provided coin
func (k Keeper) IsConvertEnabledCoins(ctx sdk.Context, coins ...sdk.Coin) error {
	for _, coin := range coins {
		if !k.IsConvertEnabledCoin(ctx, coin) {
			return sdkerrors.Wrapf(types.ErrConvertDisabled, "%s converts are currently disabled", coin.Denom)
		}
	}
	return nil
}

// IsConvertEnabledCoin returns the current ConvertEnabled status of the provided coin's denom
func (k Keeper) IsConvertEnabledCoin(ctx sdk.Context, coin sdk.Coin) bool {
	return k.GetParams(ctx).ConvertEnabledDenom(coin.Denom)
}

// GetEvmParams returns the total set of evm parameters.
func (k Keeper) GetEvmParams(ctx sdk.Context) (params evmTypes.Params) {
	k.evmParamSpace.GetParamSet(ctx, &params)
	return params
}

// GetSourceChannelID returns the channel id for an ibc voucher
// The voucher has for format ibc/hash(path)
func (k Keeper) GetSourceChannelID(ctx sdk.Context, ibcVoucherDenom string) (channelID string, err error) {
	// remove the ibc
	hash := strings.Split(ibcVoucherDenom, "/")[1]
	hexDenomBytes, err := transferTypes.ParseHexHash(hash)
	if err != nil {
		return "", sdkerrors.Wrapf(types.ErrIbcCroDenomInvalid, "%s is invalid", ibcVoucherDenom)
	}
	denomTrace, exists := k.transferKeeper.GetDenomTrace(ctx, hexDenomBytes)
	if !exists {
		return "", sdkerrors.Wrapf(types.ErrIbcCroDenomInvalid, "%s is invalid", ibcVoucherDenom)
	}

	// the path has for format port/channelId
	return strings.Split(denomTrace.Path, "/")[1], nil
}
