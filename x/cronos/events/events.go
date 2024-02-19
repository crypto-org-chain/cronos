package events

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v7/modules/apps/29-fee/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ica "github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/ica"
	relayer "github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/relayer"
	cronoseventstypes "github.com/crypto-org-chain/cronos/v2/x/cronos/events/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	RelayerEvents        map[string]*EventDescriptor
	IcaEvents            map[string]*EventDescriptor
	RelayerValueDecoders = ValueDecoders{
		channeltypes.AttributeKeyDataHex:      ConvertPacketData,
		transfertypes.AttributeKeyAmount:      ConvertAmount,
		banktypes.AttributeKeyRecipient:       ConvertAccAddressFromBech32,
		banktypes.AttributeKeySpender:         ConvertAccAddressFromBech32,
		banktypes.AttributeKeyReceiver:        ConvertAccAddressFromBech32,
		banktypes.AttributeKeySender:          ConvertAccAddressFromBech32,
		banktypes.AttributeKeyMinter:          ConvertAccAddressFromBech32,
		banktypes.AttributeKeyBurner:          ConvertAccAddressFromBech32,
		channeltypes.AttributeKeySequence:     ReturnStringAsIs,
		channeltypes.AttributeKeySrcPort:      ReturnStringAsIs,
		channeltypes.AttributeKeySrcChannel:   ReturnStringAsIs,
		channeltypes.AttributeKeyDstPort:      ReturnStringAsIs,
		channeltypes.AttributeKeyDstChannel:   ReturnStringAsIs,
		channeltypes.AttributeKeyConnectionID: ReturnStringAsIs,
		ibcfeetypes.AttributeKeyFee:           ReturnStringAsIs,
		transfertypes.AttributeKeyDenom:       ReturnStringAsIs,
	}
	IcaValueDecoders = ValueDecoders{
		cronoseventstypes.AttributeKeySeq:   ConvertUint64,
		channeltypes.AttributeKeySrcChannel: ReturnStringAsIs,
	}
)

func init() {
	var relayerABI abi.ABI
	if err := relayerABI.UnmarshalJSON([]byte(relayer.RelayerModuleMetaData.ABI)); err != nil {
		panic(err)
	}
	RelayerEvents = NewEventDescriptors(relayerABI)

	var icaABI abi.ABI
	if err := icaABI.UnmarshalJSON([]byte(ica.ICAModuleMetaData.ABI)); err != nil {
		panic(err)
	}
	IcaEvents = NewEventDescriptors(icaABI)
}

func RelayerConvertEvent(event sdk.Event) (*ethtypes.Log, error) {
	desc, ok := RelayerEvents[event.Type]
	if !ok {
		return nil, nil
	}
	return desc.ConvertEvent(event.Attributes, RelayerValueDecoders)
}

func IcaConvertEvent(event sdk.Event) (*ethtypes.Log, error) {
	desc, ok := IcaEvents[event.Type]
	if !ok {
		return nil, nil
	}
	return desc.ConvertEvent(event.Attributes, IcaValueDecoders)
}
