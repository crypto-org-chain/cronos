package cronos

import (
	"fmt"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
)

// NewHandler for Cronos
func NewHandler(k keeper.Keeper) sdk.Handler {
	msgServer := keeper.NewMsgServerImpl(k)
	// this line is used by starport scaffolding # handler/msgServer

	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		// this line is used by starport scaffolding # 1
		case *types.MsgConvertVouchers:
			res, err := msgServer.ConvertVouchers(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgTransferTokens:
			res, err := msgServer.TransferTokens(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgUpdateTokenMapping:
			res, err := msgServer.UpdateTokenMapping(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgTurnBridge:
			res, err := msgServer.TurnBridge(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		default:
			errMsg := fmt.Sprintf("unrecognized %s message type: %T", types.ModuleName, msg)
			return nil, errors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}
