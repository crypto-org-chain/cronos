package cronoskeeper

import (
	appparams "github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"

	ibchost "github.com/cosmos/ibc-go/modules/core/24-host"

	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	ibctransferkeeper "github.com/cosmos/ibc-go/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/modules/apps/transfer/types"
	ibckeeper "github.com/cosmos/ibc-go/modules/core/keeper"
	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	cronostypes "github.com/crypto-org-chain/cronos/x/cronos/types"
	evmkeeper "github.com/tharsis/ethermint/x/evm/keeper"

	ethermint "github.com/tharsis/ethermint/types"
)

func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

func Fuzz(data []byte) int {

	encodingConfig := appparams.EncodingConfig
	appCodec := encodingConfig.Marshaler

	keys := sdk.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, capabilitytypes.StoreKey,
		feegrant.StoreKey, authzkeeper.StoreKey,
		// ibc keys
		ibchost.StoreKey, ibctransfertypes.StoreKey,
		// ethermint keys
		evmtypes.StoreKey,
		// this line is used by starport scaffolding # stargate/app/storeKey
		cronostypes.StoreKey,
	)

	maccPerms := map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner}, // used for secure addition and subtraction of balance using module account
		cronostypes.ModuleName:         {authtypes.Minter, authtypes.Burner},
	}

	AccountKeeper := authkeeper.NewAccountKeeper(
		appCodec, keys[authtypes.StoreKey], GetSubspace(authtypes.ModuleName), ethermint.ProtoAccount, maccPerms,
	)

	BankKeeper := bankkeeper.NewBaseKeeper(
		appCodec, keys[banktypes.StoreKey], AccountKeeper, GetSubspace(banktypes.ModuleName), ModuleAccountAddrs(),
	)

	stakingKeeper := stakingkeeper.NewKeeper(
		appCodec, keys[stakingtypes.StoreKey], AccountKeeper, BankKeeper, GetSubspace(stakingtypes.ModuleName),
	)

	UpgradeKeeper := upgradekeeper.NewKeeper(skipUpgradeHeights, keys[upgradetypes.StoreKey], appCodec, homePath, BaseApp)

	

	IBCKeeper := ibckeeper.NewKeeper(
		appCodec, keys[ibchost.StoreKey], GetSubspace(ibchost.ModuleName), stakingKeeper, UpgradeKeeper, scopedIBCKeeper,
	)

	TransferKeeper := ibctransferkeeper.NewKeeper(
		appCodec, keys[ibctransfertypes.StoreKey], GetSubspace(ibctransfertypes.ModuleName),
		IBCKeeper.ChannelKeeper, &IBCKeeper.PortKeeper,
		AccountKeeper, BankKeeper, scopedTransferKeeper,
	)

	EvmKeeper := evmkeeper.NewKeeper(
		appCodec, keys[evmtypes.StoreKey], tkeys[evmtypes.TransientKey], GetSubspace(evmtypes.ModuleName),
		AccountKeeper, BankKeeper, stakingKeeper,
		tracer, bApp.Trace(), // debug EVM based on Baseapp options
	)

	CronosKeeper := *cronoskeeper.NewKeeper(
		appCodec,
		keys[cronostypes.StoreKey],
		keys[cronostypes.MemStoreKey],
		data,
		BankKeeper,
		TransferKeeper,
		EvmKeeper,
	)

	result := CronosKeeper.IsEmptyHash(string(data))
	if result {
		return 0
	}
	return 1
}
