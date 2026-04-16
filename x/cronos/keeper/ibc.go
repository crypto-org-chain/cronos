package keeper

import (
	"errors"
	"fmt"

	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/hashicorp/go-metrics"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) ConvertVouchersToEvmCoins(ctx sdk.Context, from string, coins sdk.Coins) error {
	acc, err := sdk.AccAddressFromBech32(from)
	if err != nil {
		return err
	}

	params := k.GetParams(ctx)
	evmParams := k.GetEvmParams(ctx)
	for _, c := range coins {
		switch c.Denom {
		case params.IbcCroDenom:
			if params.IbcCroDenom == "" {
				return errorsmod.Wrap(types.ErrIbcCroDenomEmpty, "ibc is disabled")
			}

			// Send ibc tokens to escrow address
			err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, acc, types.ModuleName, sdk.NewCoins(c))
			if err != nil {
				return err
			}
			// Compute new amount, because basecro is a 8 decimals token, we need to multiply by 10^10 to make it
			// a 18 decimals token
			amount18dec := sdk.NewCoin(evmParams.EvmDenom, c.Amount.Mul(sdkmath.NewIntFromBigInt(types.TenPowTen)))

			// Mint new evm tokens
			if err := k.bankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(amount18dec),
			); err != nil {
				return err
			}

			// Send evm tokens to receiver
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(
				ctx, types.ModuleName, acc, sdk.NewCoins(amount18dec),
			); err != nil {
				return err
			}

		default:
			err := k.ConvertCoinFromNativeToCRC21(ctx, common.BytesToAddress(acc.Bytes()), c, params.EnableAutoDeployment)
			if err != nil {
				return err
			}
		}
	}
	defer func() {
		for _, a := range coins {
			if a.Amount.IsInt64() {
				telemetry.SetGaugeWithLabels(
					[]string{"tx", "msg", "ConvertVouchersToEvmCoins"},
					float32(a.Amount.Int64()),
					[]metrics.Label{telemetry.NewLabel("denom", a.Denom)},
				)
			}
		}
	}()
	return nil
}

func (k Keeper) IbcTransferCoins(ctx sdk.Context, from, destination string, coins sdk.Coins, channelId string) error {
	acc, err := sdk.AccAddressFromBech32(from)
	if err != nil {
		return err
	}

	if len(destination) == 0 {
		return errors.New("to address cannot be empty")
	}

	params := k.GetParams(ctx)
	evmParams := k.GetEvmParams(ctx)

	for _, c := range coins {
		switch c.Denom {
		case evmParams.EvmDenom:
			// Compute the remainder, we won't transfer anything lower than 10^10
			amount8decRem := c.Amount.Mod(sdkmath.NewIntFromBigInt(types.TenPowTen))
			amountToBurn := c.Amount.Sub(amount8decRem)
			if amountToBurn.IsZero() {
				// Amount too small
				continue
			}
			coins := sdk.NewCoins(sdk.NewCoin(evmParams.EvmDenom, amountToBurn))

			// Send evm tokens to escrow address
			err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, acc, types.ModuleName, coins)
			if err != nil {
				return err
			}
			// Burns the evm tokens
			if err := k.bankKeeper.BurnCoins(
				ctx, types.ModuleName, coins); err != nil {
				return err
			}

			// Transfer ibc tokens back to the user
			// We divide by 10^10 to come back to an 8decimals token
			amount8dec := c.Amount.Quo(sdkmath.NewIntFromBigInt(types.TenPowTen))
			ibcCoin := sdk.NewCoin(params.IbcCroDenom, amount8dec)
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(
				ctx, types.ModuleName, acc, sdk.NewCoins(ibcCoin),
			); err != nil {
				return err
			}

			// No need to specify the channelId because it's not a source token
			err = k.ibcSendTransfer(ctx, acc, destination, ibcCoin, "")
			if err != nil {
				return err
			}

		default:
			if !types.IsValidIBCDenom(c.Denom) && !types.IsValidCronosDenom(c.Denom) {
				return fmt.Errorf("the coin %s is neither an ibc voucher or a cronos token", c.Denom)
			}
			_, found := k.GetContractByDenom(ctx, c.Denom)
			if !found {
				return fmt.Errorf("coin %s is not supported", c.Denom)
			}
			err = k.ibcSendTransfer(ctx, acc, destination, c, channelId)
			if err != nil {
				return err
			}
		}
	}

	defer func() {
		for _, a := range coins {
			if a.Amount.IsInt64() {
				telemetry.SetGaugeWithLabels(
					[]string{"tx", "msg", "IbcTransferCoins"},
					float32(a.Amount.Int64()),
					[]metrics.Label{telemetry.NewLabel("denom", a.Denom)},
				)
			}
		}
	}()
	return nil
}

func (k Keeper) ibcSendTransfer(ctx sdk.Context, sender sdk.AccAddress, destination string, coin sdk.Coin, channelId string) error {
	if types.IsSourceCoin(coin.Denom) {
		if !channeltypes.IsValidChannelID(channelId) {
			return errors.New("invalid channel id for ibc transfer of source token")
		}
	} else {
		// If it is not source, then coin is a voucher so we can extract the channel id from the denom
		channelDenom := coin.Denom
		sourceChannelID, err := k.GetSourceChannelID(ctx, channelDenom)
		if err != nil {
			return err
		}
		channelId = sourceChannelID
	}

	// Transfer coins to receiver through IBC
	// We use current time for timeout timestamp and zero height for timeoutHeight
	// it means it can never fail by timeout
	params := k.GetParams(ctx)
	timeoutTimestamp := uint64(ctx.BlockTime().UnixNano()) + params.IbcTimeout
	timeoutHeight := ibcclienttypes.ZeroHeight()
	msg := ibctransfertypes.MsgTransfer{
		SourcePort:       ibctransfertypes.PortID,
		SourceChannel:    channelId,
		Token:            coin,
		Sender:           sender.String(),
		Receiver:         destination,
		TimeoutHeight:    timeoutHeight,
		TimeoutTimestamp: timeoutTimestamp,
	}
	if _, err := k.transferKeeper.Transfer(ctx, &msg); err != nil {
		return err
	}
	return nil
}
