//go:build !rocksdb
// +build !rocksdb

package app

import (
	"errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/versiondb"
)

func (app *App) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	tkeys map[string]*storetypes.TransientStoreKey,
	memKeys map[string]*storetypes.MemoryStoreKey,
) (sdk.MultiStore, versiondb.VersionStore, error) {
	return nil, nil, errors.New("versiondb is not supported in this binary")
}

func (app *App) buildVersionDBSSTFiles(
	storeKeyNames []string,
	dbDir, homePath string,
	start, end int64,
) ([]string, error) {
	return nil, nil
}
