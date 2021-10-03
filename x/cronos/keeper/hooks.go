package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/modules/apps/transfer/types"
	"github.com/ethereum/go-ethereum/common"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"
	cronostypes "github.com/crypto-org-chain/cronos/x/cronos/types"
	"strconv"
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
	coin := sdk.NewCoin(denom, event.Amount)
	err = k.ConvertCoinFromNativeToCRC20(ctx, addr, coin, true)
	if err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		cronostypes.EventTypeEthereumSendToCosmosHandled,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(cronostypes.AttributeKeySender, event.GetEthereumSender()),
		sdk.NewAttribute(cronostypes.AttributeKeyReceiver, event.GetCosmosReceiver()),
		sdk.NewAttribute(cronostypes.AttributeKeyAmount, coin.String()),
		sdk.NewAttribute(gravitytypes.AttributeKeyBridgeChainID, strconv.FormatUint(
			k.gravityKeeper.GetParams(ctx).BridgeChainId, 10,
		)),
		sdk.NewAttribute(cronostypes.AttributeKeyEthereumTokenContract, event.GetTokenContract()),
		sdk.NewAttribute(gravitytypes.AttributeKeyNonce, strconv.FormatUint(event.GetEventNonce(), 10)),
		sdk.NewAttribute(gravitytypes.AttributeKeyEthereumEventVoteRecordID,
			string(gravitytypes.MakeEthereumEventVoteRecordKey(event.GetEventNonce(), event.Hash()))),
	))

	return nil
}
