package module

import (
	appparams "github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"

	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ibctransferkeeper "github.com/cosmos/ibc-go/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/modules/apps/transfer/types"
	ibchost "github.com/cosmos/ibc-go/modules/core/24-host"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"

	cronos "github.com/crypto-org-chain/cronos/x/cronos"
	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	cronostypes "github.com/crypto-org-chain/cronos/x/cronos/types"
)

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
	AccountKeeper := authkeeper.NewAccountKeeper(
		appCodec, keys[authtypes.StoreKey], GetSubspace(authtypes.ModuleName), ethermint.ProtoAccount, maccPerms,
	)

	BankKeeper := bankkeeper.NewBaseKeeper(
		appCodec, keys[banktypes.StoreKey], AccountKeeper, GetSubspace(banktypes.ModuleName), ModuleAccountAddrs(),
	)

	TransferKeeper := ibctransferkeeper.NewKeeper(
		appCodec, keys[ibctransfertypes.StoreKey], app.GetSubspace(ibctransfertypes.ModuleName),
		IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
		AccountKeeper, BankKeeper, scopedTransferKeeper,
	)

	EvmKeeper := evmkeeper.NewKeeper(
		appCodec, keys[evmtypes.StoreKey], tkeys[evmtypes.TransientKey], app.GetSubspace(evmtypes.ModuleName),
		AccountKeeper, BankKeeper, stakingKeeper,
		tracer, bApp.Trace(), // debug EVM based on Baseapp options
	)

	CronosKeeper := *cronoskeeper.NewKeeper(
		appparams.EncodingConfig.Marshaler,
		keys[cronostypes.StoreKey],
		keys[cronostypes.MemStoreKey],
		data,
		BankKeeper,
		TransferKeeper,
		EvmKeeper,
	)

	cronos.NewTokenMappingChangeProposalHandler(CronosKeeper)

}
