package events

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	ibctypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	generated "github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/relayer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	IBCEvents        map[string]*EventDescriptor
	IBCValueDecoders = ValueDecoders{
		ibctypes.AttributeKeyData:        ConvertPacketData,
		transfertypes.AttributeKeyAmount: ConvertAmount,
	}
)

func init() {
	var ibcABI abi.ABI
	if err := ibcABI.UnmarshalJSON([]byte(generated.RelayerModuleMetaData.ABI)); err != nil {
		panic(err)
	}
	IBCEvents = NewEventDescriptors(ibcABI)
}

func ConvertEvent(event sdk.Event) (*ethtypes.Log, error) {
	desc, ok := IBCEvents[event.Type]
	if !ok {
		return nil, nil
	}
	return desc.ConvertEvent(event.Attributes, IBCValueDecoders)
}
