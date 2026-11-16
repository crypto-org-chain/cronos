package app

import (
	"context"
	"errors"
	"fmt"

	channelv11 "github.com/cosmos/ibc-go/v11/modules/core/04-channel/migrations/v11"
	channeltypes "github.com/cosmos/ibc-go/v11/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v11/modules/core/exported"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/evmos/ethermint/upgrades/sdk55"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/runtime"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

const planName = "v1.9"

// RegisterUpgradeHandlers returns true if it installed the upgrade store loader.
func (app *App) RegisterUpgradeHandlers(maxVersion int64) bool {
	app.UpgradeKeeper.SetUpgradeHandler(planName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			// Anyone can pre-fund the pool address, and a plain account there makes every key rotation panic.
			sdk55.EnsureModuleAccount(sdkCtx, app.AccountKeeper, stakingtypes.KeyRotationFeePoolName)

			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return toVM, err
			}
			// After RunMigrations, so an unguarded copy of this migration that a later ibc-go
			// release registers runs first and this run keeps its result.
			if err := app.migrateChannelSendSequences(sdkCtx); err != nil {
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

// migrateChannelSendSequences moves IBC v1 channel send sequences to the key ibc-go v11.2.1 reads,
// which ibc-go registers no migration for. Safe to repeat and to run after an upstream copy of the
// migration, but it cannot restore a sequence that a repeated upstream run already set to 0.
func (app *App) migrateChannelSendSequences(ctx sdk.Context) error {
	// ibc-go's MigrateStore sets every channel without a v1 key to sequence 0, so first copy an
	// already-migrated sequence back to the v1 key for it to move again.
	ibcStore := runtime.NewKVStoreService(app.keys[ibcexported.StoreKey])
	store := ibcStore.OpenKVStore(ctx)
	var err error
	app.IBCKeeper.ChannelKeeper.IterateChannels(ctx, func(ic channeltypes.IdentifiedChannel) bool {
		v1Key := channelv11.NextSequenceSendV1Key(ic.PortId, ic.ChannelId)
		var has bool
		if has, err = store.Has(v1Key); err != nil {
			return true
		}
		if has {
			return false
		}
		if seq, found := app.IBCKeeper.ChannelKeeperV2.GetNextSequenceSend(ctx, ic.ChannelId); found {
			err = store.Set(v1Key, sdk.Uint64ToBigEndian(seq))
		}
		return err != nil
	})
	if err != nil {
		return err
	}
	return channelv11.MigrateStore(ctx, ibcStore, app.appCodec, app.IBCKeeper)
}

// setV19StakingParams sets the smallest valid key rotation fee, and drops HistoricalInfo once EIP-2935 serves BLOCKHASH.
func (app *App) setV19StakingParams(ctx sdk.Context) error {
	params, err := app.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("get staking params: %w", err)
	}
	// Most operators hold no liquid bond denom, so the default fee would block their rotations.
	params.KeyRotationFee = sdk.NewCoin(params.BondDenom, math.OneInt())

	if err := app.blockHashHistoryGap(ctx); err != nil {
		ctx.Logger().Info("keeping staking historical_entries", "value", params.HistoricalEntries, "reason", err)
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

// blockHashHistoryGap returns why the EIP-2935 contract can't serve BLOCKHASH for the last header_hash_num blocks, or nil if it can.
func (app *App) blockHashHistoryGap(ctx sdk.Context) error {
	acct := app.EvmKeeper.GetAccount(ctx, ethparams.HistoryStorageAddress)
	if acct == nil || !acct.IsContract() {
		return errors.New("EIP-2935 history storage contract not deployed")
	}
	evmParams := app.EvmKeeper.GetParams(ctx)
	if window := evmParams.EffectiveHistoryServeWindow(); window < evmParams.HeaderHashNum {
		return fmt.Errorf("history_serve_window %d is below header_hash_num %d", window, evmParams.HeaderHashNum)
	}
	// Contract-only read: GetHeaderHash also falls back to the legacy store, which HistoricalInfo backs up.
	upgradeHeight := uint64(ctx.BlockHeight())
	for i := uint64(1); i <= evmParams.HeaderHashNum && i < upgradeHeight; i++ {
		h := upgradeHeight - i
		if _, ok := app.EvmKeeper.GetHistoryContractHash(ctx, h); !ok {
			return fmt.Errorf("no EIP-2935 header hash at height %d", h)
		}
	}
	return nil
}

// pruneHistoricalInfo deletes entries from the block before the upgrade downward, since TrackHistoricalInfo
// with zero entries stops at the current height. Raw keys avoid failing on an entry that no longer decodes.
func (app *App) pruneHistoricalInfo(ctx sdk.Context) {
	store := ctx.KVStore(app.keys[stakingtypes.StoreKey])
	for h := ctx.BlockHeight() - 1; h >= 0; h-- {
		key := stakingtypes.GetHistoricalInfoKey(h)
		if !store.Has(key) {
			return
		}
		store.Delete(key)
	}
}
