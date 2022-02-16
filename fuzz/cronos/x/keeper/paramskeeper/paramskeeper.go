package cronoskeeper

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	cronostypes "github.com/crypto-org-chain/cronos/x/cronos/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	ibctransfertypes "github.com/cosmos/ibc-go/modules/apps/transfer/types"
	ibchost "github.com/cosmos/ibc-go/modules/core/24-host"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
)

func Fuzz(data []byte) int {
	appCodec := encodingConfig.Marshaler

	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	cdc := encodingConfig.Amino
	
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable())
	paramsKeeper.Subspace(crisistypes.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(ibchost.ModuleName)
	paramsKeeper.Subspace(evmtypes.ModuleName)
	// this line is used by starport scaffolding # stargate/app/paramSubspace
	paramsKeeper.Subspace(cronostypes.ModuleName)

	ParamsKeeper = initParamsKeeper(appCodec, cdc, keys[paramstypes.StoreKey], tkeys[paramstypes.TStoreKey])
	
}
