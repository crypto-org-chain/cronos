package app

import (
	"path/filepath"

	"github.com/spf13/cast"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"

	"github.com/crypto-org-chain/cronos/store/rootmulti"
)

const (
	FlagMemIAVL     = "memiavl.enable"
	FlagAsyncCommit = "memiavl.async-commit"
	FlagZeroCopy    = "memiavl.zero-copy"
)

func SetupMemIAVL(logger log.Logger, homePath string, appOpts servertypes.AppOptions, baseAppOptions []func(*baseapp.BaseApp)) []func(*baseapp.BaseApp) {
	if cast.ToBool(appOpts.Get(FlagMemIAVL)) {
		// cms must be overridden before the other options, because they may use the cms,
		// make sure the cms aren't be overridden by the other options later on.
		cms := rootmulti.NewStore(filepath.Join(homePath, "data", "memiavl.db"), logger)
		cms.SetAsyncCommit(cast.ToBool(appOpts.Get(FlagAsyncCommit)))
		cms.SetZeroCopy(cast.ToBool(appOpts.Get(FlagZeroCopy)))
		baseAppOptions = append([]func(*baseapp.BaseApp){setCMS(cms)}, baseAppOptions...)
	}

	return baseAppOptions
}
