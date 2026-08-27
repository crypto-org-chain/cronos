package app

import (
	icacontrollertypes "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/types"
	icaauthtypes "github.com/crypto-org-chain/cronos/app/legacy/icaauth/types"

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
	// ICA has been removed from the app
	// Keep its types registered so the client can still decode historical
	// ICA transactions.
	icatypes.RegisterInterfaces(registry)
	icacontrollertypes.RegisterInterfaces(registry)
	icahosttypes.RegisterInterfaces(registry)
}
