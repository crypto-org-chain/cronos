package keeper

import (
	"strings"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transferTypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	evmTypes "github.com/evmos/ethermint/x/evm/types"
)

// GetParams returns the total set of cronos parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return params
	}
	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// SetParams sets the total set of cronos parameters.
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}

	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(&params)
	store.Set(types.ParamsKey, bz)

	return nil
}

// GetEvmParams returns the total set of evm parameters.
func (k Keeper) GetEvmParams(ctx sdk.Context) evmTypes.Params {
	return k.evmKeeper.GetParams(ctx)
}

// GetSourceChannelID returns the channel id for an ibc voucher
// The voucher has for format ibc/hash(path)
func (k Keeper) GetSourceChannelID(ctx sdk.Context, ibcVoucherDenom string) (channelID string, err error) {
	// remove the ibc
	hash := strings.Split(ibcVoucherDenom, "/")
	if len(hash) != 2 {
		return "", errors.Wrapf(types.ErrIbcCroDenomInvalid, "%s is invalid", ibcVoucherDenom)
	}
	hexDenomBytes, err := transferTypes.ParseHexHash(hash[1])
	if err != nil {
		return "", errors.Wrapf(types.ErrIbcCroDenomInvalid, "%s is invalid", ibcVoucherDenom)
	}
	denomTrace, exists := k.transferKeeper.GetDenom(ctx, hexDenomBytes)
	if !exists {
		return "", errors.Wrapf(types.ErrIbcCroDenomInvalid, "%s is invalid", ibcVoucherDenom)
	}

	// the path has for format port/channelId
	return denomTrace.Trace[0].ChannelId, nil
}
