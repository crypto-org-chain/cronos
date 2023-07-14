package keeper

import (
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cronostypes "github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"
)

// TODO Implements GravityHooks interface
func (k Keeper) AfterContractCallExecutedEvent(ctx sdk.Context, event gravitytypes.ContractCallExecutedEvent) {
	if k.gravityKeeper == nil {
		return
	}
}

func (k Keeper) AfterERC20DeployedEvent(ctx sdk.Context, event gravitytypes.ERC20DeployedEvent) {
	if k.gravityKeeper == nil {
		return
	}
}

func (k Keeper) AfterSignerSetExecutedEvent(ctx sdk.Context, event gravitytypes.SignerSetTxExecutedEvent) {
	if k.gravityKeeper == nil {
		return
	}
}

func (k Keeper) AfterBatchExecutedEvent(ctx sdk.Context, event gravitytypes.BatchExecutedEvent) {
	if k.gravityKeeper == nil {
		return
	}
}

func (k Keeper) AfterSendToCosmosEvent(ctx sdk.Context, event gravitytypes.SendToCosmosEvent) {
	if k.gravityKeeper == nil {
		return
	}

	cacheCtx, commit := ctx.CacheContext()
	err := k.doAfterSendToCosmosEvent(cacheCtx, event)
	if err == nil {
		commit()
	} else {
		k.Logger(ctx).Error("AfterSendToCosmosEvent hook failed", "error", err)
	}
}

func (k Keeper) doAfterSendToCosmosEvent(ctx sdk.Context, event gravitytypes.SendToCosmosEvent) error {
	if k.gravityKeeper == nil {
		return fmt.Errorf("gravity is not enable")
	}

	_, denom := k.gravityKeeper.ERC20ToDenomLookup(ctx, common.HexToAddress(event.TokenContract))
	coin := sdk.NewCoin(denom, event.Amount)
	// TODO: Remove after event is emitted at Gravity module https://github.com/crypto-org-chain/gravity-bridge/pull/12
	coins := sdk.Coins{sdk.NewCoin(denom, event.Amount)}
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		cronostypes.EventTypeEthereumSendToCosmosHandled,
		sdk.NewAttribute(sdk.AttributeKeyModule, gravitytypes.ModuleName),
		sdk.NewAttribute(cronostypes.AttributeKeySender, event.GetEthereumSender()),
		sdk.NewAttribute(cronostypes.AttributeKeyReceiver, event.GetCosmosReceiver()),
		sdk.NewAttribute(cronostypes.AttributeKeyAmount, coins.String()),
		sdk.NewAttribute(gravitytypes.AttributeKeyBridgeChainID, strconv.FormatUint(
			k.gravityKeeper.GetParams(ctx).BridgeChainId, 10,
		)),
		sdk.NewAttribute(cronostypes.AttributeKeyEthereumTokenContract, event.GetTokenContract()),
		sdk.NewAttribute(gravitytypes.AttributeKeyNonce, strconv.FormatUint(event.GetEventNonce(), 10)),
		sdk.NewAttribute(gravitytypes.AttributeKeyEthereumEventVoteRecordID,
			string(gravitytypes.MakeEthereumEventVoteRecordKey(event.GetEventNonce(), event.Hash()))),
	))

	// Try to convert the newly minted native tokens to erc20 contract
	cosmosAddr, err := sdk.AccAddressFromBech32(event.CosmosReceiver)
	if err != nil {
		return err
	}
	addr := common.BytesToAddress(cosmosAddr.Bytes())
	enableAutoDeployment := k.GetParams(ctx).EnableAutoDeployment
	err = k.ConvertCoinFromNativeToCRC21(ctx, addr, coin, enableAutoDeployment)
	if err != nil {
		return err
	}

	return nil
}
