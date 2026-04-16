package keeper_test

import (
	"context"

	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	"github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	emptyTraceIbcDenomHash = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	EmptyTraceIbcDenom     = "ibc/" + emptyTraceIbcDenomHash
)

type IbcKeeperMock struct{}

func (i IbcKeeperMock) Transfer(goCtx context.Context, msg *types.MsgTransfer) (*types.MsgTransferResponse, error) {
	return nil, nil
}

func (i IbcKeeperMock) GetDenom(ctx sdk.Context, denomTraceHash tmbytes.HexBytes) (types.Denom, bool) {
	if denomTraceHash.String() == "6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865" {
		return types.Denom{
			Trace: []types.Hop{
				{PortId: "transfer", ChannelId: "channel-0"},
			},
			Base: "basetcro",
		}, true
	}
	if denomTraceHash.String() == "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
		return types.Denom{
			Trace: []types.Hop{
				{PortId: "transfer", ChannelId: "channel-0"},
			},
			Base: "correctIBCToken",
		}, true
	}

	if denomTraceHash.String() == emptyTraceIbcDenomHash {
		return types.Denom{
			Trace: []types.Hop{},
			Base:  "emptyTraceToken",
		}, true
	}
	return types.Denom{}, false
}
