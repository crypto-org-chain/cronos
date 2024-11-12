//go:build rocksdb
// +build rocksdb

package app

import (
	"os"
	"path/filepath"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/versiondb"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

func (app *App) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
) (sdk.MultiStore, error) {
	dataDir := filepath.Join(homePath, "data", "versiondb")
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		return nil, err
	}

	versionDB, err := tsrocksdb.NewStore(dataDir)
	if err != nil {
		return nil, err
	}

	// default to exposing all
	exposeStoreKeys := make([]storetypes.StoreKey, 0, len(keys))
	for _, storeKey := range keys {
		exposeStoreKeys = append(exposeStoreKeys, storeKey)
	}

	// see: https://github.com/crypto-org-chain/cronos/issues/1683
	versionDB.SetSkipVersionZero(true)

	service := versiondb.NewStreamingService(versionDB, exposeStoreKeys)
	app.SetStreamingService(service)

	verDB := versiondb.NewMultiStore(app.CommitMultiStore(), versionDB, exposeStoreKeys)
	verDB.MountTransientStores(tkeys)
	verDB.MountMemoryStores(memKeys)

	app.SetQueryMultiStore(verDB)
	return verDB, nil
}
