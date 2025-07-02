package app

import (
	icaauthtypes "github.com/crypto-org-chain/cronos/v2/app/legacy/icaauth/types"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
)

func RegisterLegacyCodec(cdc *codec.LegacyAmino) {
	icaauthtypes.RegisterCodec(cdc)
	authz.RegisterLegacyAminoCodec(cdc)
}

func RegisterLegacyInterfaces(registry cdctypes.InterfaceRegistry) {
	icaauthtypes.RegisterInterfaces(registry)
	authz.RegisterInterfaces(registry)
}
