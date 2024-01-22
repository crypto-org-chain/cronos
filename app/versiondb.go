//go:build rocksdb
// +build rocksdb

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/versiondb"
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
)

func (app *App) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
) (sdk.MultiStore, versiondb.VersionStore, error) {
	dataDir := filepath.Join(homePath, "data", "versiondb")
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		return nil, nil, err
	}
	versionDB, err := tsrocksdb.NewStore(dataDir)
	if err != nil {
		return nil, nil, err
	}

	// default to exposing all
	exposeStoreKeys := make([]storetypes.StoreKey, 0, len(keys))
	for _, storeKey := range keys {
		exposeStoreKeys = append(exposeStoreKeys, storeKey)
	}

	service := versiondb.NewStreamingService(versionDB, exposeStoreKeys)
	app.SetStreamingService(service)

	verDB := versiondb.NewMultiStore(app.CommitMultiStore(), versionDB, exposeStoreKeys)
	verDB.MountTransientStores(tkeys)
	verDB.MountMemoryStores(memKeys)

	app.SetQueryMultiStore(verDB)
	return verDB, versionDB, nil
}

func waitForFiles(storeKeyNames []string, outDir, file string) error {
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		allFoldersContainFiles := true
		for _, storeKeyName := range storeKeyNames {
			matches, err := filepath.Glob(filepath.Join(outDir, storeKeyName, file))
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				allFoldersContainFiles = false
				break
			}
		}
		if allFoldersContainFiles {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("partial dump from store")
}

func (app *App) buildVersionDBSSTFiles(
	storeKeyNames []string,
	homePath string,
	start, end int64,
) ([]string, error) {
	// wait changeset dump
	outDir := fmt.Sprintf("%s/dump", homePath)
	file := fmt.Sprintf("block-%d.zz", start)
	if err := waitForFiles(storeKeyNames, outDir, file); err != nil {
		return nil, err
	}
	// changeset build-versiondb-sst
	sstDir := fmt.Sprintf("%s/build", homePath)
	if err := os.MkdirAll(sstDir, os.ModePerm); err != nil {
		return nil, err
	}
	concurrency := 1
	if err := versiondbclient.ConvertSingleStores(
		storeKeyNames, outDir, sstDir,
		versiondbclient.DefaultSSTFileSize, versiondbclient.DefaultSorterChunkSize,
		concurrency,
	); err != nil {
		return nil, err
	}

	return versiondbclient.GetSSTFilePaths(sstDir)
}
