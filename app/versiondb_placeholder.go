//go:build !rocksdb

package app

import (
	"errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

func (app *App) setupVersionDB(
	homePath string,
	keys map[string]*storetypes.KVStoreKey,
	okeys map[string]*storetypes.ObjectStoreKey,
) (storetypes.MultiStore, error) {
	return nil, errors.New("versiondb is not supported in this binary")
}
