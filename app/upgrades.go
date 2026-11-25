package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	channelv11 "github.com/cosmos/ibc-go/v11/modules/core/04-channel/migrations/v11"
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

// RegisterUpgradeHandlers installs no store loader: v1.9 adds and deletes no stores (params stays mounted for historical queries).
// An upgrade that adds or deletes stores must swap app.go's MaxVersionStoreLoader for MaxVersionUpgradeStoreLoader, so versiondb nodes keep their version cap.
func (app *App) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(planName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			// Anyone can pre-fund the pool address, and a plain account there makes every key rotation panic.
			sdk55.EnsureModuleAccount(sdkCtx, app.AccountKeeper, stakingtypes.KeyRotationFeePoolName)

			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return toVM, err
			}
			// After RunMigrations, so a copy of this migration a later ibc-go release registers runs first and this run keeps its result.
			if err := app.migrateChannelSendSequences(sdkCtx); err != nil {
				return toVM, fmt.Errorf("migrate ibc channel send sequences: %w", err)
			}
			if err := app.setMinKeyRotationFee(sdkCtx); err != nil {
				return toVM, err
			}
			if err := app.disableHistoricalInfo(sdkCtx); err != nil {
				return toVM, err
			}
			return toVM, nil
		},
	)
}

// migrateChannelSendSequences runs the send-sequence migration ibc-go v11.2.1 ships but never registers.
// Safe to repeat, but it cannot restore a sequence that a repeated upstream run already set to 0.
func (app *App) migrateChannelSendSequences(ctx sdk.Context) error {
	// MigrateStore sets every channel without a v1 key to sequence 0, so copy an already-migrated sequence back first.
	ibcStore := runtime.NewKVStoreService(app.keys[ibcexported.StoreKey])
	store := ibcStore.OpenKVStore(ctx)
	for _, ic := range app.IBCKeeper.ChannelKeeper.GetAllChannels(ctx) {
		seq, found := app.IBCKeeper.ChannelKeeperV2.GetNextSequenceSend(ctx, ic.ChannelId)
		if !found {
			continue
		}
		if err := store.Set(channelv11.NextSequenceSendV1Key(ic.PortId, ic.ChannelId), sdk.Uint64ToBigEndian(seq)); err != nil {
			return err
		}
	}
	return channelv11.MigrateStore(ctx, ibcStore, app.appCodec, app.IBCKeeper)
}

// Most operators hold no liquid bond denom, so the default fee would block their rotations.
func (app *App) setMinKeyRotationFee(ctx sdk.Context) error {
	return app.updateStakingParams(ctx, func(params *stakingtypes.Params) {
		params.KeyRotationFee = sdk.NewCoin(params.BondDenom, math.OneInt())
	})
}

// disableHistoricalInfo stops storing HistoricalInfo and deletes what is stored, but only once EIP-2935 serves BLOCKHASH.
func (app *App) disableHistoricalInfo(ctx sdk.Context) error {
	if err := app.blockHashHistoryGap(ctx); err != nil {
		ctx.Logger().Info("keeping staking historical_entries", "reason", err)
		return nil
	}
	if err := app.updateStakingParams(ctx, func(params *stakingtypes.Params) {
		params.HistoricalEntries = 0
	}); err != nil {
		return err
	}
	app.pruneHistoricalInfo(ctx)
	return nil
}

func (app *App) updateStakingParams(ctx sdk.Context, update func(*stakingtypes.Params)) error {
	params, err := app.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("get staking params: %w", err)
	}
	update(&params)
	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate staking params: %w", err)
	}
	if err := app.StakingKeeper.SetParams(ctx, params); err != nil {
		return fmt.Errorf("set staking params: %w", err)
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

// pruneHistoricalInfo deletes by raw key prefix: TrackHistoricalInfo with zero entries prunes nothing older, and an entry may no longer decode.
func (app *App) pruneHistoricalInfo(ctx sdk.Context) {
	store := ctx.KVStore(app.keys[stakingtypes.StoreKey])
	for _, key := range prefixKeys(store, stakingtypes.HistoricalInfoKey) {
		store.Delete(key)
	}
}

func prefixKeys(store storetypes.KVStore, prefix []byte) [][]byte {
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		// The iterator contract lets a backend reuse the key buffer on Next.
		keys = append(keys, bytes.Clone(iter.Key()))
	}
	return keys
}
