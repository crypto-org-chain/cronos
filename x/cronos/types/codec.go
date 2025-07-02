package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	// this line is used by starport scaffolding # 2
	cdc.RegisterConcrete(&TokenMappingChangeProposal{}, "cronos/TokenMappingChangeProposal", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	// this line is used by starport scaffolding # 3
	registry.RegisterImplementations(
		(*govtypes.Content)(nil),
		&TokenMappingChangeProposal{},
	)

	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgConvertVouchers{},
		&MsgTransferTokens{},
		&MsgUpdateTokenMapping{},
		&MsgTurnBridge{},
		&MsgUpdatePermissions{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
