package keeper

import (
	"context"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	ibctransfertypes "github.com/cosmos/ibc-go/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (k msgServer) ConvertTokens(goCtx context.Context, msg *types.MsgConvertTokens) (*types.MsgConvertResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	acc, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return nil, err
	}

	params := k.GetParams(ctx)
	evmParams := k.GetEvmParams(ctx)
	for _, c := range msg.Amount {
		switch c.Denom {
		case params.IbcCroDenom:
			if params.IbcCroDenom == "" {
				return nil, sdkerrors.Wrap(types.ErrIbcCroDenomEmpty, "ibc is disabled")
			}

			// Send ibc tokens to escrow address
			err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, acc, types.ModuleName, sdk.NewCoins(c))
			if err != nil {
				return nil, err
			}
			// Compute new amount, because basecro is a 8 decimals token, we need to multiply by 10^10 to make it
			// a 18 decimals token
			amount18dec := sdk.NewCoin(evmParams.EvmDenom, c.Amount.Mul(sdk.NewIntFromBigInt(types.TenPowTen)))

			// Mint new evm tokens
			if err := k.bankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(amount18dec),
			); err != nil {
				return nil, err
			}

			// Send evm tokens to receiver
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(
				ctx, types.ModuleName, acc, sdk.NewCoins(amount18dec),
			); err != nil {
				return nil, err
			}

		default:
			if err := k.IsConvertEnabledCoins(ctx, c); err != nil {
				return nil, err
			}
			// TODO wrap to erc20 tokens
		}
	}
	defer func() {
		for _, a := range msg.Amount {
			if a.Amount.IsInt64() {
				telemetry.SetGaugeWithLabels(
					[]string{"tx", "msg", "convertTokens"},
					float32(a.Amount.Int64()),
					[]metrics.Label{telemetry.NewLabel("denom", a.Denom)},
				)
			}
		}
	}()

	// emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		types.NewConvertCoinEvent(acc, msg.Amount),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		)},
	)

	return &types.MsgConvertResponse{}, nil
}

func (k msgServer) SendToCryptoOrg(goCtx context.Context, msg *types.MsgSendToCryptoOrg) (*types.MsgConvertResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	acc, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return nil, err
	}

	params := k.GetParams(ctx)
	evmParams := k.GetEvmParams(ctx)

	for _, c := range msg.Amount {
		switch c.Denom {
		case evmParams.EvmDenom:
			// Compute the remainder, we won't transfer anything lower than 10^10
			amount8decRem := c.Amount.Mod(sdk.NewIntFromBigInt(types.TenPowTen))
			amountToBurn := c.Amount.Sub(amount8decRem)
			if amountToBurn.IsZero() {
				// Amount too small
				break
			}
			coins := sdk.NewCoins(sdk.NewCoin(evmParams.EvmDenom, amountToBurn))

			// Send evm tokens to escrow address
			err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, acc, types.ModuleName, coins)
			if err != nil {
				return nil, err
			}
			// Burns the evm tokens
			if err := k.bankKeeper.BurnCoins(
				ctx, types.ModuleName, coins); err != nil {
				return nil, err
			}

			// Transfer ibc tokens back to the user
			// We divide by 10^10 to come back to an 8decimals token
			amount8dec := c.Amount.Quo(sdk.NewIntFromBigInt(types.TenPowTen))
			ibcCoin := sdk.NewCoin(params.IbcCroDenom, amount8dec)
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(
				ctx, types.ModuleName, acc, sdk.NewCoins(ibcCoin),
			); err != nil {
				return nil, err
			}

			channelID, err := k.GetSourceChannelID(ctx, params.IbcCroDenom)
			if err != nil {
				return nil, err
			}
			// Transfer coins to receiver through IBC
			// We use current time for timeout timestamp and zero height for timeoutHeight
			// it means it can never fail by timeout
			// TODO Might need to consider add timeout option in configuration.
			timeoutTimestamp := ctx.BlockTime().UnixNano()
			timeoutHeight := ibcclienttypes.ZeroHeight()
			err = k.transferKeeper.SendTransfer(
				ctx,
				ibctransfertypes.PortID,
				channelID,
				ibcCoin,
				acc,
				msg.To,
				timeoutHeight,
				uint64(timeoutTimestamp))
			if err != nil {
				return nil, err
			}

		default:
			if err := k.IsConvertEnabledCoins(ctx, c); err != nil {
				return nil, err
			}
			// TODO wrap to erc20 tokens
		}
	}

	defer func() {
		for _, a := range msg.Amount {
			if a.Amount.IsInt64() {
				telemetry.SetGaugeWithLabels(
					[]string{"tx", "msg", "sendToCryptoOrg"},
					float32(a.Amount.Int64()),
					[]metrics.Label{telemetry.NewLabel("denom", a.Denom)},
				)
			}
		}
	}()

	// emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		types.NewSendToCryptoOrgEvent(msg.From, msg.To, msg.Amount),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		)},
	)
	return &types.MsgConvertResponse{}, nil
}
