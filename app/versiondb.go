//go:build rocksdb

package app

import (
	"os"
	"path/filepath"

	"github.com/crypto-org-chain/cronos-store/versiondb"
	"github.com/crypto-org-chain/cronos-store/versiondb/tsrocksdb"

	storetypes "cosmossdk.io/store/types"
)

func (app *App) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	okeys map[string]*storetypes.ObjectStoreKey,
) (storetypes.RootMultiStore, error) {
	dataDir := filepath.Join(homePath, "data", "versiondb")
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		return nil, err
	}

	versionDB, err := tsrocksdb.NewStore(dataDir)
	if err != nil {
		return nil, err
	}

	// always listen for all keys to simplify configuration
	exposedKeys := make([]storetypes.StoreKey, 0, len(keys))
	for _, key := range keys {
		exposedKeys = append(exposedKeys, key)
	}

	// see: https://github.com/crypto-org-chain/cronos/issues/1683
	versionDB.SetSkipVersionZero(true)

	app.CommitMultiStore().AddListeners(exposedKeys)

	// register in app streaming manager
	sm := app.StreamingManager()
	sm.ABCIListeners = append(sm.ABCIListeners,
		versiondb.NewStreamingService(versionDB),
	)
	app.SetStreamingManager(sm)

	delegatedStoreKeys := make(map[storetypes.StoreKey]struct{})
	for _, k := range tkeys {
		delegatedStoreKeys[k] = struct{}{}
	}
	for _, k := range okeys {
		delegatedStoreKeys[k] = struct{}{}
	}

	verDB := versiondb.NewMultiStore(app.CommitMultiStore(), versionDB, keys, delegatedStoreKeys)
	app.SetQueryMultiStore(verDB)
	return verDB, nil
}
