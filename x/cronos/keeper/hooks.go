package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"
)

// TODO Implements GravityHooks interface
func (k Keeper) AfterContractCallExecutedEvent(ctx sdk.Context, event gravitytypes.ContractCallExecutedEvent) {
}

func (k Keeper) AfterERC20DeployedEvent(ctx sdk.Context, event gravitytypes.ERC20DeployedEvent) {}

func (k Keeper) AfterSignerSetExecutedEvent(ctx sdk.Context, event gravitytypes.SignerSetTxExecutedEvent) {
}

func (k Keeper) AfterBatchExecutedEvent(ctx sdk.Context, event gravitytypes.BatchExecutedEvent) {}

func (k Keeper) AfterSendToCosmosEvent(ctx sdk.Context, event gravitytypes.SendToCosmosEvent) {
	cacheCtx, commit := ctx.CacheContext()
	err := k.doAfterSendToCosmosEvent(cacheCtx, event)
	if err == nil {
		commit()
		ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	} else {
		k.Logger(ctx).Error("AfterSendToCosmosEvent hook failed", "error", err)
	}
}

func (k Keeper) doAfterSendToCosmosEvent(ctx sdk.Context, event gravitytypes.SendToCosmosEvent) error {
	isCosmosOriginated, denom := k.gravityKeeper.ERC20ToDenomLookup(ctx, event.TokenContract)
	if isCosmosOriginated {
		// ignore cosmos originated transfer
		return nil
	}
	// Try to convert the newly minted native tokens to erc20 contract
	cosmosAddr, err := sdk.AccAddressFromBech32(event.CosmosReceiver)
	if err != nil {
		return err
	}
	addr := common.BytesToAddress(cosmosAddr.Bytes())
	// Use auto deploy here for testing.
	// FIXME update after gov feature is implemented: https://github.com/crypto-org-chain/cronos/issues/46
	err = k.ConvertCoinFromNativeToCRC20(ctx, addr, sdk.NewCoin(denom, event.Amount), true)
	if err != nil {
		return err
	}
	return nil
}
