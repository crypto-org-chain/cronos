package app

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v11/modules/core/exported"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

const planName = "v1.8"

// croBridgeContractAddresses are the EVM addresses of CroBridge contracts
// authorized on Cronos mainnet. Empty list disables the SendCroToIbc hook.
var croBridgeContractAddresses = []string{
	"0x6b1b50c2223eb31E0d4683b046ea9C6CB0D0ea4F",
	"0xCE13a6F3d4167CE958f4764D423e6D62a114c751",
}

// RegisterUpgradeHandlers returns if store loader is overridden.
// No store-key churn from v0.53→v0.54 in this app, so the default
// MaxVersionStoreLoader (set by the caller when this returns false)
// covers both regular and upgrade-height boots.
func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	for _, addr := range croBridgeContractAddresses {
		if !common.IsHexAddress(addr) || (common.HexToAddress(addr) == common.Address{}) {
			panic(fmt.Sprintf("invalid croBridgeContractAddresses entry %q: must be a non-zero EVM hex address", addr))
		}
	}
	app.UpgradeKeeper.SetUpgradeHandler(planName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return toVM, err
			}
			// Populate staking queue pending-slot indexes after migrations so the
			// indexes are built on fully-migrated queue keys (cosmos-sdk PR #26023
			// optimization, exposed as opt-in utility per crypto-org-chain
			// cosmos-sdk PR #1814 instead of an auto-migration). The keeper
			// implementation overwrites the per-time slot via Set, so re-running
			// at the same height is idempotent.
			if err := app.StakingKeeper.PopulateQueuePendingSlots(ctx); err != nil {
				return toVM, fmt.Errorf("populate queue pending slots: %w", err)
			}
			// Prune stale IBC client store keys left by the pre-v9 ibc-go migration.
			// The v7 ibc-go migration only cleaned canonical 2-segment consensusStates
			// keys; 3-segment variants (clients/<id>/consensusStates/<rev>/<h>/clientState)
			// survived and cause the ClientStates gRPC handler to panic on unmarshal,
			// enabling an unauthenticated REST DoS via GET /ibc/core/client/v1/client_states.
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			if err := pruneStaleIBCConsensusStateSubkeys(sdkCtx, runtime.KVStoreAdapter(
				runtime.NewKVStoreService(app.keys[ibcexported.StoreKey]).OpenKVStore(sdkCtx),
			)); err != nil {
				return toVM, fmt.Errorf("prune stale ibc consensus state subkeys: %w", err)
			}
			// Set CroBridgeContractAddresses to authorize the canonical CroBridge contracts
			// for the SendCroToIbc hook. This closes the unauthenticated-drain vulnerability
			// where any contract emitting __CronosSendCroToIbc could drain CRO balances.
			cronosParams := app.CronosKeeper.GetParams(sdkCtx)
			cronosParams.CroBridgeContractAddresses = croBridgeContractAddresses
			if err := app.CronosKeeper.SetParams(sdkCtx, cronosParams); err != nil {
				return toVM, fmt.Errorf("set cro bridge contract addresses: %w", err)
			}
			return toVM, nil
		},
	)
	return false
}

// pruneStaleIBCConsensusStateSubkeys deletes stale keys of the form
// clients/<id>/consensusStates/<revision>/<height>/clientState left behind when
// old-format consensus state entries were not cleaned up by the ibc-go v7 migration.
// Idempotent — safe to run multiple times.
func pruneStaleIBCConsensusStateSubkeys(ctx sdk.Context, store storetypes.KVStore) error {
	iterator := storetypes.KVStorePrefixIterator(store, host.KeyClientStorePrefix)
	defer sdk.LogDeferred(ctx.Logger(), func() error { return iterator.Close() })

	var staleKeys [][]byte
	for ; iterator.Valid(); iterator.Next() {
		// iterator.Key() includes the full "clients/" prefix.
		// Canonical: clients/<id>/clientState (3 parts)
		// Stale:     clients/<id>/consensusStates/<rev>/<h>/clientState (≥5 parts)
		parts := strings.Split(string(iterator.Key()), "/")
		if len(parts) >= 5 &&
			parts[2] == host.KeyConsensusStatePrefix &&
			parts[len(parts)-1] == host.KeyClientState {
			staleKeys = append(staleKeys, bytes.Clone(iterator.Key()))
		}
	}

	for _, k := range staleKeys {
		store.Delete(k)
	}
	return nil
}
