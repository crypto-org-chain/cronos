package keeper

import (
	"context"
	"math/big"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
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

			// Send ibc token escrow address
			err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, acc, types.ModuleName, sdk.NewCoins(c))
			if err != nil {
				return nil, err
			}
			// Compute new amount, because basecro is a 8 decimals token, we need to multiply by 10^10 to make it
			// a 18 decimals token
			ten := big.NewInt(10)
			exponent := ten.Exp(ten, ten, nil)
			newAmount := sdk.NewCoin(evmParams.EvmDenom, c.Amount.Mul(sdk.NewIntFromBigInt(exponent)))

			// Mint new coins
			if err := k.bankKeeper.MintCoins(
				ctx, types.ModuleName, sdk.NewCoins(newAmount),
			); err != nil {
				return nil, err
			}

			// Send Evm coins to receiver
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(
				ctx, types.ModuleName, acc, sdk.NewCoins(newAmount),
			); err != nil {
				return nil, err
			}

		default:
			if err := k.IsConvertEnabledCoins(ctx, msg.Amount...); err != nil {
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

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	)

	return &types.MsgConvertResponse{}, nil
}

func (k msgServer) SendToCryptoOrg(goCtx context.Context, msg *types.MsgSendToCryptoOrg) (*types.MsgConvertResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	acc, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return nil, err
	}

	evmParams := k.GetEvmParams(ctx)

	for _, c := range msg.Amount {
		switch c.Denom {
		case evmParams.EvmDenom:
			// Compute new amount, because evm token  is a 18 decimals token, we need to divide by 10^10 to make it
			// a 8 decimals token
			ten := big.NewInt(10)
			exponent := ten.Exp(ten, ten, nil)
			newAmount := sdk.NewCoin(evmParams.EvmDenom, c.Amount.Quo(sdk.NewIntFromBigInt(exponent)))

			// Send evm token escrow address
			err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, acc, types.ModuleName, sdk.NewCoins(newAmount))
			if err != nil {
				return nil, err
			}
			// Burns the evm token
			if err := k.bankKeeper.BurnCoins(
				ctx, types.ModuleName, sdk.NewCoins(newAmount),
			); err != nil {
				return nil, err
			}

			// Transfer coins to receiver through IBC
			// TODO

		default:
			if err := k.IsConvertEnabledCoins(ctx, msg.Amount...); err != nil {
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

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	)

	return &types.MsgConvertResponse{}, nil
}
