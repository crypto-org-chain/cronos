package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type IbcKeeperMock struct {
}

func (i IbcKeeperMock) SendTransfer(ctx sdk.Context, sourcePort, sourceChannel string, token sdk.Coin, sender sdk.AccAddress, receiver string, timeoutHeight clienttypes.Height, timeoutTimestamp uint64) error {
	return nil
}

func (i IbcKeeperMock) GetDenomTrace(ctx sdk.Context, denomTraceHash tmbytes.HexBytes) (types.DenomTrace, bool) {
	return types.DenomTrace{
		Path:      "transfer/channel-0",
		BaseDenom: "basetcro",
	}, true
}
