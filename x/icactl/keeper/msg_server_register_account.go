package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/icactl/types"
)

func (k msgServer) RegisterAccount(goCtx context.Context, msg *types.MsgRegisterAccount) (*types.MsgRegisterAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.RegisterInterchainAccount(ctx, msg.ConnectionId, msg.Owner, ""); err != nil {
		return nil, err
	}

	return &types.MsgRegisterAccountResponse{}, nil
}
