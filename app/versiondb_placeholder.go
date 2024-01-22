//go:build !rocksdb
// +build !rocksdb

package app

import (
	"errors"

	dbm "github.com/cometbft/cometbft-db"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/versiondb"
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/linxGnu/grocksdb"
)

func (app *App) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
) (sdk.MultiStore, versiondb.VersionStore, error) {
	return nil, nil, errors.New("versiondb is not supported in this binary")
}

func GetOptions(storeNames []string) versiondbclient.Options {
	return versiondbclient.Options{
		DefaultStores: storeNames,
		OpenReadOnlyDB: func(home string, backend dbm.BackendType) (dbm.DB, error) {
			return nil, errors.New("versiondb is not supported in this binary")
		},
		AppRocksDBOptions: func(sstFileWriter bool) *grocksdb.Options {
			return nil
		},
	}
}
