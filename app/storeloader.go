package app

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

// MaxVersionStoreLoader will be used when there's versiondb to cap the loaded iavl version
func MaxVersionStoreLoader(version int64) baseapp.StoreLoader {
	if version == 0 {
		return baseapp.DefaultStoreLoader
	}

	return func(ms storetypes.CommitMultiStore) error {
		return ms.LoadVersion(version)
	}
}

// MaxVersionUpgradeStoreLoader applies storeUpgrades when the loaded version is the block before upgradeHeight.
func MaxVersionUpgradeStoreLoader(version, upgradeHeight int64, storeUpgrades *storetypes.StoreUpgrades) baseapp.StoreLoader {
	if version == 0 {
		return upgradetypes.UpgradeStoreLoader(upgradeHeight, storeUpgrades)
	}

	return func(ms storetypes.CommitMultiStore) error {
		hasUpgrades := len(storeUpgrades.Renamed) > 0 || len(storeUpgrades.Deleted) > 0 || len(storeUpgrades.Added) > 0
		if !hasUpgrades {
			return MaxVersionStoreLoader(version)(ms)
		}
		// memiavl loads its latest for a higher target, so the store's own height also bounds the loaded version.
		last := ms.LastCommitID().Version
		loaded := min(version, last)
		if loaded == upgradeHeight-1 {
			return ms.LoadVersionAndUpgrade(loaded, storeUpgrades)
		}
		// Loading below the block before the upgrade either halts in x/upgrade (once state holds an earlier
		// upgrade or the plan) or replays the upgrade block without the store upgrades and forks, so fail early
		// with recovery advice when either side has reached it. When versiondb lags, loading the uncapped latest
		// instead fails the post-load versiondb lag check. When both are behind, this relies on that x/upgrade halt.
		if loaded < upgradeHeight-1 {
			switch {
			case upgradeHeight <= last:
				return fmt.Errorf(
					"versiondb version %d is behind the block before upgrade height %d and the store is already past it at %d: restore versiondb to at least version %d or resync it",
					version, upgradeHeight, last, upgradeHeight-1,
				)
			case upgradeHeight == last+1:
				return fmt.Errorf(
					"versiondb version %d is behind the block before upgrade height %d: run the previous binary until it halts at the upgrade, then restart",
					version, upgradeHeight,
				)
			case version >= upgradeHeight-1:
				return fmt.Errorf(
					"store version %d is behind the block before upgrade height %d while versiondb is at %d: on memiavl, run the previous binary until it halts at the upgrade, then restart; on iavl, restore the store to at least version %d",
					last, upgradeHeight, version, version,
				)
			}
		}
		return MaxVersionStoreLoader(version)(ms)
	}
}
