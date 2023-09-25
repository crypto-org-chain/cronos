package events

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v7/modules/apps/29-fee/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	ibctypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	generated "github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/relayer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	IBCEvents        map[string]*EventDescriptor
	IBCValueDecoders = ValueDecoders{
		ibctypes.AttributeKeyDataHex:         ConvertPacketData,
		ibctypes.AttributeKeyConnection:      ReturnStringAsIs,
		ibctypes.AttributeKeyChannelOrdering: ReturnStringAsIs,
		ibctypes.AttributeKeySrcPort:         ReturnStringAsIs,
		ibctypes.AttributeKeySrcChannel:      ReturnStringAsIs,
		ibctypes.AttributeKeyDstPort:         ReturnStringAsIs,
		ibctypes.AttributeKeyDstChannel:      ReturnStringAsIs,
		ibcfeetypes.AttributeKeyFee:          ReturnStringAsIs,
		transfertypes.AttributeKeyDenom:      ReturnStringAsIs,
		transfertypes.AttributeKeyAmount:     ConvertAmount,
		banktypes.AttributeKeyRecipient:      ConvertAccAddressFromBech32,
		banktypes.AttributeKeySpender:        ConvertAccAddressFromBech32,
		banktypes.AttributeKeyReceiver:       ConvertAccAddressFromBech32,
		banktypes.AttributeKeySender:         ConvertAccAddressFromBech32,
		banktypes.AttributeKeyMinter:         ConvertAccAddressFromBech32,
		banktypes.AttributeKeyBurner:         ConvertAccAddressFromBech32,
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
