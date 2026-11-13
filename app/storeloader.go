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

// MaxVersionUpgradeStoreLoader applies storeUpgrades when the capped version is the block before upgradeHeight.
// It checks the capped version, not the multistore's latest, because that is the version the multistore loads.
func MaxVersionUpgradeStoreLoader(version, upgradeHeight int64, storeUpgrades *storetypes.StoreUpgrades) baseapp.StoreLoader {
	if version == 0 {
		return upgradetypes.UpgradeStoreLoader(upgradeHeight, storeUpgrades)
	}

	return func(ms storetypes.CommitMultiStore) error {
		hasUpgrades := len(storeUpgrades.Renamed) > 0 || len(storeUpgrades.Deleted) > 0 || len(storeUpgrades.Added) > 0
		if hasUpgrades && upgradeHeight == version+1 {
			return ms.LoadVersionAndUpgrade(version, storeUpgrades)
		}
		// Store upgrades only apply at load time, so loading below upgradeHeight-1 would replay
		// the upgrade block without them and fork. Loading the uncapped latest instead fails the
		// post-load versiondb lag check.
		if hasUpgrades && version+1 < upgradeHeight && upgradeHeight <= ms.LastCommitID().Version+1 {
			return fmt.Errorf(
				"versiondb version %d is behind the block before upgrade height %d: run the previous binary until it halts at the upgrade, then restart",
				version, upgradeHeight,
			)
		}
		return MaxVersionStoreLoader(version)(ms)
	}
}
