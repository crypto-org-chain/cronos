package app

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
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
