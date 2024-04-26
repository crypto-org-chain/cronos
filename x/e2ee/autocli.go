package e2ee

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: "e2ee.Query",
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod:      "Key",
					Use:            "key [address]",
					Short:          "Query an encryption key by address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "address"}},
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: "e2ee.Msg",
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "RegisterEncryptionKey",
					Use:       "set-encryption-key [key]",
					Short:     "Set encryption key is stored associated with the user address.",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "key"},
					},
				},
			},
		},
	}
}
