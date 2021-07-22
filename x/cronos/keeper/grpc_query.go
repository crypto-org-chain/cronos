package keeper

import (
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var _ types.QueryServer = Keeper{}
