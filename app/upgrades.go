package app

import (
	"context"
	"encoding/binary"
	"fmt"

	ibcexported "github.com/cosmos/ibc-go/v11/modules/core/exported"
	"github.com/ethereum/go-ethereum/common"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/evmos/ethermint/upgrades/ibcv1121"
	"github.com/evmos/ethermint/upgrades/sdk55"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

const planName = "v1.9"

// RegisterUpgradeHandlers returns true if it installed the upgrade store loader.
func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	app.UpgradeKeeper.SetUpgradeHandler(planName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			// Before migrations: anyone can pre-fund the pool address, and a plain account there makes every key rotation panic.
			sdk55.EnsureModuleAccount(sdkCtx, app.AccountKeeper, stakingtypes.KeyRotationFeePoolName)

			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return toVM, err
			}
			// ibc-go v11.2.1 reads send sequences from a new key and registers no migration for the move.
			ibcStore := runtime.NewKVStoreService(app.keys[ibcexported.StoreKey])
			if err := ibcv1121.MigrateChannelSendSequences(sdkCtx, ibcStore, app.appCodec, app.IBCKeeper); err != nil {
				return toVM, fmt.Errorf("migrate ibc channel send sequences: %w", err)
			}
			if err := app.setV19StakingParams(sdkCtx); err != nil {
				return toVM, err
			}
			return toVM, nil
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk: %s", err))
	}
	if upgradeInfo.Name != planName || app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		return false
	}
	storeUpgrades := storetypes.StoreUpgrades{
		Deleted: []string{"params"},
	}
	app.SetStoreLoader(MaxVersionUpgradeStoreLoader(maxVersion, upgradeInfo.Height, &storeUpgrades))
	return true
}

// setV19StakingParams sets the smallest valid key rotation fee, and drops HistoricalInfo once EIP-2935 serves BLOCKHASH.
func (app *App) setV19StakingParams(ctx sdk.Context) error {
	params, err := app.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("get staking params: %w", err)
	}
	// Most operators hold no liquid bond denom, so the default fee would block their rotations.
	params.KeyRotationFee = sdk.NewCoin(params.BondDenom, math.OneInt())

	if reason := app.blockHashHistoryGap(ctx); reason != "" {
		ctx.Logger().Info("keeping staking historical_entries", "value", params.HistoricalEntries, "reason", reason)
	} else {
		params.HistoricalEntries = 0
	}

	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate staking params: %w", err)
	}
	if err := app.StakingKeeper.SetParams(ctx, params); err != nil {
		return fmt.Errorf("set staking params: %w", err)
	}
	if params.HistoricalEntries == 0 {
		app.pruneHistoricalInfo(ctx)
	}
	return nil
}

// blockHashHistoryGap returns why the EIP-2935 contract can't serve BLOCKHASH for the last header_hash_num blocks, or "" if it can.
func (app *App) blockHashHistoryGap(ctx sdk.Context) string {
	acct := app.EvmKeeper.GetAccount(ctx, ethparams.HistoryStorageAddress)
	if acct == nil || !acct.IsContract() {
		return "EIP-2935 history storage contract not deployed"
	}
	evmParams := app.EvmKeeper.GetParams(ctx)
	window := evmParams.HistoryServeWindow
	if window == 0 {
		window = evmtypes.DefaultHistoryServeWindow
	}
	if window < evmParams.HeaderHashNum {
		return fmt.Sprintf("history_serve_window %d is below header_hash_num %d", window, evmParams.HeaderHashNum)
	}
	// Reads the contract slot directly: GetHeaderHash also falls back to the legacy store, which HistoricalInfo backs up.
	upgradeHeight := uint64(ctx.BlockHeight())
	for i := uint64(1); i <= evmParams.HeaderHashNum && i < upgradeHeight; i++ {
		h := upgradeHeight - i
		var slot common.Hash
		binary.BigEndian.PutUint64(slot[24:], h%window)
		if app.EvmKeeper.GetState(ctx, ethparams.HistoryStorageAddress, slot) == (common.Hash{}) {
			return fmt.Sprintf("no EIP-2935 header hash at height %d", h)
		}
	}
	return ""
}

// pruneHistoricalInfo deletes entries from the block before the upgrade downward, since TrackHistoricalInfo
// with zero entries stops at the current height. Raw keys avoid failing on an entry that no longer decodes.
func (app *App) pruneHistoricalInfo(ctx sdk.Context) {
	store := runtime.KVStoreAdapter(runtime.NewKVStoreService(app.keys[stakingtypes.StoreKey]).OpenKVStore(ctx))
	for h := ctx.BlockHeight() - 1; h >= 0; h-- {
		key := stakingtypes.GetHistoricalInfoKey(h)
		if !store.Has(key) {
			return
		}
		store.Delete(key)
	}
}
