package app

import (
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

// MaxVersionUpgradeStoreLoader is used to prepare baseapp with a fixed StoreLoader
func MaxVersionUpgradeStoreLoader(version, upgradeHeight int64, storeUpgrades *storetypes.StoreUpgrades) baseapp.StoreLoader {
	if version == 0 {
		return upgradetypes.UpgradeStoreLoader(upgradeHeight, storeUpgrades)
	}

	return func(ms storetypes.CommitMultiStore) error {
		if upgradeHeight == ms.LastCommitID().Version+1 {
			// Check if the current commit version and upgrade height matches
			if len(storeUpgrades.Renamed) > 0 || len(storeUpgrades.Deleted) > 0 || len(storeUpgrades.Added) > 0 {
				return ms.LoadLatestVersionAndUpgrade(storeUpgrades)
			}
		}

		// Otherwise load default store loader
		return MaxVersionStoreLoader(version)(ms)
	}
}
